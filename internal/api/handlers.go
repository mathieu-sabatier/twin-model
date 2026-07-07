package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/api/dto"
	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/mermaid"
	"github.com/mathieu-sabatier/twin-model/internal/modeldesign"
	"github.com/mathieu-sabatier/twin-model/internal/semdiff"
)

// This file holds the HTTP handlers. Draft/file resolution is centralized in the
// draft/draftFile/draftModel helpers below so each handler stays a thin shell of
// its own logic, and the shared 404/422 responses live in exactly one place.

// draft looks up the draft named by the request's {id}, writing a 404 and
// returning ok=false when it is unknown.
func (s *Server) draft(w http.ResponseWriter, r *http.Request) (*Draft, bool) {
	d, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		httpErr(w, http.StatusNotFound, "draft not found")
	}
	return d, ok
}

// draftFile resolves the draft and its selected ?file=, writing the appropriate
// 404 and returning ok=false on failure. The bytes come back unparsed, for the
// endpoints that treat a parse error as data (model, validate).
func (s *Server) draftFile(w http.ResponseWriter, r *http.Request) (d *Draft, path string, data []byte, ok bool) {
	if d, ok = s.draft(w, r); !ok {
		return nil, "", nil, false
	}
	if path, data, ok = selectFile(d.Files, r.URL.Query().Get("file")); !ok {
		httpErr(w, http.StatusNotFound, "file not found in draft")
		return nil, "", nil, false
	}
	return d, path, data, true
}

// draftModel is draftFile plus a parse, writing a 422 on a structural parse
// error. Used by the endpoints for which an unparseable file is a hard failure
// (previews, diff, resolved) rather than data.
func (s *Server) draftModel(w http.ResponseWriter, r *http.Request) (d *Draft, path string, m *dsl.Model, ok bool) {
	d, path, data, ok := s.draftFile(w, r)
	if !ok {
		return nil, "", nil, false
	}
	m, err := dsl.Parse(path, data)
	if err != nil {
		httpErr(w, http.StatusUnprocessableEntity, "parse: "+err.Error())
		return nil, "", nil, false
	}
	return d, path, m, true
}

// buildModelResponse parses data and builds the model envelope. A parse error is
// returned as data (ParseError set, Model nil), not a transport error.
func (s *Server) buildModelResponse(path string, data []byte) dto.ModelResponse {
	m, err := dsl.Parse(path, data)
	if err != nil {
		return dto.ModelResponse{File: path, ParseError: err.Error()}
	}
	if c, err := s.catalogInstance(); err == nil {
		m.Catalog = c
	}
	return dto.ModelResponse{
		File:        path,
		Model:       dto.FromModel(m),
		Diagnostics: dto.FromDiagnostics(dsl.Validate(m)),
	}
}

// handleModel serves the AST-as-JSON of a model file read from a committed ref.
func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		httpErr(w, http.StatusBadRequest, "ref is required")
		return
	}
	tree, err := s.host.ReadTree(r.Context(), ref)
	if err != nil {
		httpErr(w, http.StatusBadGateway, "read tree: "+err.Error())
		return
	}
	// Restrict to model files so a non-model path 404s rather than surfacing a
	// parse error — symmetric with draft creation.
	path, data, ok := selectFile(modelFilesOnly(tree), r.URL.Query().Get("file"))
	if !ok {
		httpErr(w, http.StatusNotFound, "file not found in ref")
		return
	}
	writeJSON(w, http.StatusOK, s.buildModelResponse(path, data))
}

// handleCreateDraft creates a draft from a committed ref's model files.
func (s *Server) handleCreateDraft(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseRef string `json:"baseRef"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BaseRef == "" {
		httpErr(w, http.StatusBadRequest, "baseRef is required")
		return
	}
	tree, err := s.host.ReadTree(r.Context(), req.BaseRef)
	if err != nil {
		// A missing branch is a user error (they chose/typed a base that does not
		// exist), not a bad gateway: go-git reports "couldn't find remote ref …".
		if strings.Contains(err.Error(), "couldn't find remote ref") {
			httpErr(w, http.StatusNotFound, fmt.Sprintf("branch %q not found", req.BaseRef))
			return
		}
		httpErr(w, http.StatusBadGateway, "read tree: "+err.Error())
		return
	}
	d := s.store.Create(req.BaseRef, modelFilesOnly(tree))
	writeJSON(w, http.StatusCreated, dto.CreateDraftResponse{
		ID:      d.ID,
		BaseRef: d.BaseRef,
		Files:   sortedKeys(d.Files),
	})
}

// handleDraft returns a draft's metadata so a client can re-hydrate it by id.
func (s *Server) handleDraft(w http.ResponseWriter, r *http.Request) {
	d, ok := s.draft(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, dto.DraftResponse{
		ID:        d.ID,
		BaseRef:   d.BaseRef,
		Files:     sortedKeys(d.Files),
		UpdatedAt: d.UpdatedAt.UTC().Format(time.RFC3339),
	})
}

// handleDraftModel serves the AST-as-JSON of a draft's selected file.
func (s *Server) handleDraftModel(w http.ResponseWriter, r *http.Request) {
	d, path, data, ok := s.draftFile(w, r)
	if !ok {
		return
	}
	resp := s.buildModelResponse(path, data)
	resp.Files = sortedKeys(d.Files) // lets a refreshed client rebuild the file switcher
	writeJSON(w, http.StatusOK, resp)
}

// handlePutFiles updates draft files, canonicalizing each parseable file via
// dsl.Format and keeping raw bytes for any file that does not parse.
func (s *Server) handlePutFiles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Files map[string]string `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Files) == 0 {
		httpErr(w, http.StatusBadRequest, "files is required")
		return
	}
	d, ok := s.store.Update(r.PathValue("id"), func(d *Draft) {
		for name, content := range req.Files {
			key := resolveWriteKey(d.Files, name)
			d.Files[key] = canonicalize(key, []byte(content))
		}
	})
	if !ok {
		httpErr(w, http.StatusNotFound, "draft not found")
		return
	}
	writeJSON(w, http.StatusOK, dto.FilesResponse{Files: sortedKeys(d.Files)})
}

// handleValidate returns structured diagnostics for a draft's selected file.
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	_, path, data, ok := s.draftFile(w, r)
	if !ok {
		return
	}
	m, err := dsl.Parse(path, data)
	if err != nil {
		writeJSON(w, http.StatusOK, dto.ValidateResponse{File: path, ParseError: err.Error()})
		return
	}
	if c, err := s.catalogInstance(); err == nil {
		m.Catalog = c
	}
	writeJSON(w, http.StatusOK, dto.ValidateResponse{File: path, Diagnostics: dto.FromDiagnostics(dsl.Validate(m))})
}

// handleModelDesign renders the generated ModelDesign XML for a draft file.
func (s *Server) handleModelDesign(w http.ResponseWriter, r *http.Request) {
	_, _, m, ok := s.draftModel(w, r)
	if !ok {
		return
	}
	xml, err := modeldesign.Emit(m)
	if err != nil {
		httpErr(w, http.StatusUnprocessableEntity, "emit: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(xml) // Write implies 200
}

// handleDiagram renders Mermaid source for a draft file (?view=types|instances).
func (s *Server) handleDiagram(w http.ResponseWriter, r *http.Request) {
	_, _, m, ok := s.draftModel(w, r)
	if !ok {
		return
	}
	src := mermaid.TypesDiagram(m)
	if r.URL.Query().Get("view") == "instances" {
		src = mermaid.InstancesDiagram(m)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(src)) // Write implies 200
}

// handleDiff returns the semantic changelist of a draft file vs its baseRef.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	d, path, draftModel, ok := s.draftModel(w, r)
	if !ok {
		return
	}
	// Base side: the same file's baseRef snapshot. A file added in the draft has
	// no base counterpart, so diff against an empty model.
	baseModel := &dsl.Model{}
	if _, baseData, ok := selectFile(d.BaseFiles, path); ok {
		if bm, err := dsl.Parse(path, baseData); err == nil {
			baseModel = bm
		}
	}
	changes := semdiff.Diff(baseModel, draftModel)
	writeJSON(w, http.StatusOK, dto.DiffResponse{Changes: changes, Text: semdiff.Render(changes)})
}

// handleResolved returns the flattened inherited members of a type in a draft.
func (s *Server) handleResolved(w http.ResponseWriter, r *http.Request) {
	_, _, m, ok := s.draftModel(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	rms, err := m.ResolveMembers(name)
	if err != nil {
		httpErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ResolvedResponse{Type: name, Members: dto.FromResolvedMembers(rms)})
}

// handlePRs lists the open pull requests on the model repo.
func (s *Server) handlePRs(w http.ResponseWriter, r *http.Request) {
	prs, err := s.host.ListPRs(r.Context())
	if err != nil {
		httpErr(w, http.StatusBadGateway, "list prs: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.PRListResponse{PRs: prs})
}

// handleRepo returns repo context, the commit identity, and whether proposing is
// possible, so the UI can show which repo/identity it operates as and gate the
// Propose button up front instead of failing at submit.
func (s *Server) handleRepo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.host.Info())
}

// handleBranches lists the repo's branches (default first) and its resolved
// default branch, so the footer branch picker can offer the real branches. A
// listing failure is a bad gateway; the SPA degrades to default+current base.
func (s *Server) handleBranches(w http.ResponseWriter, r *http.Request) {
	list, err := s.host.Branches(r.Context())
	if err != nil {
		httpErr(w, http.StatusBadGateway, "list branches: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handlePropose validates every model file and, if none is lint-red or
// unparseable, commits the fileset and opens a pull request. A lint-red draft is
// rejected with 409 and the blocking diagnostics.
func (s *Server) handlePropose(w http.ResponseWriter, r *http.Request) {
	d, ok := s.draft(w, r)
	if !ok {
		return
	}
	var req struct {
		Branch  string `json:"branch"`
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Branch == "" || req.Title == "" {
		httpErr(w, http.StatusBadRequest, "branch and title are required")
		return
	}
	if blocking := s.lintFileset(d.Files); len(blocking) > 0 {
		writeJSON(w, http.StatusConflict, dto.ConflictResponse{Diagnostics: blocking})
		return
	}
	url, err := s.host.OpenPR(r.Context(), ProposeParams{
		BaseRef: d.BaseRef,
		Branch:  req.Branch,
		Title:   req.Title,
		Message: req.Message,
		Files:   d.Files,
	})
	if err != nil {
		msg, detail := describeProposeError(err)
		writeJSON(w, http.StatusBadGateway, dto.ProposeErrorResponse{Error: msg, Detail: detail})
		return
	}
	writeJSON(w, http.StatusOK, dto.ProposeResponse{URL: url})
}

// describeProposeError maps an OpenPR failure to a friendly, actionable message and
// a raw detail string. The branch/commit are created before the PR is opened, so we
// tell the user their local work landed even though the PR step failed. QA finding M3.
func describeProposeError(err error) (msg, detail string) {
	var pr *PRError
	if errors.As(err, &pr) {
		switch pr.Status {
		case http.StatusNotFound:
			msg = "Couldn't open the pull request: the repository or branch wasn't found, or the access token lacks permission. The branch and commit were created, but the PR could not be opened."
		default:
			msg = fmt.Sprintf("Couldn't open the pull request (GitHub returned %d). The branch and commit were created, but the PR could not be opened.", pr.Status)
		}
		return msg, pr.Body
	}
	return "Couldn't open the pull request. The branch and commit may have been created locally, but pushing to the remote failed — check that a GitHub remote and access token are configured.", err.Error()
}

// handleUnits returns all known engineering units sorted by symbol.
func (s *Server) handleUnits(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dto.UnitsResponse{Units: dto.FromUnits(dsl.Units())})
}

// handleDraftFileRaw serves a draft file's stored canonical YAML as text, for the
// UI's YAML pane. The bytes were canonicalized on PUT, so this is authoritative.
func (s *Server) handleDraftFileRaw(w http.ResponseWriter, r *http.Request) {
	_, _, data, ok := s.draftFile(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

// canonicalize returns the canonical YAML for content when it parses; otherwise
// it returns the raw bytes, so the next model/validate read surfaces the
// structural error rather than silently dropping the edit.
func canonicalize(filename string, content []byte) []byte {
	m, err := dsl.Parse(filename, content)
	if err != nil {
		return content
	}
	formatted, err := dsl.Format(m)
	if err != nil {
		return content
	}
	return formatted
}

// lintFileset returns the blocking diagnostics across every model file in
// deterministic order: a synthetic parse-error diagnostic for an unparseable
// file, plus each error-severity diagnostic from a parseable one. An empty
// result means the fileset is safe to propose.
func (s *Server) lintFileset(files map[string][]byte) []dto.Diagnostic {
	var blocking []dto.Diagnostic
	for _, path := range sortedKeys(files) {
		m, err := dsl.Parse(path, files[path])
		if err != nil {
			blocking = append(blocking, dto.Diagnostic{
				Code: "parse-error", Severity: "error", File: path, Path: path, Message: err.Error(),
			})
			continue
		}
		if c, err := s.catalogInstance(); err == nil {
			m.Catalog = c
		}
		for _, dg := range dsl.Validate(m) {
			if dg.Severity == dsl.SeverityError {
				blocking = append(blocking, dto.FromDiagnostic(dg))
			}
		}
	}
	return blocking
}
