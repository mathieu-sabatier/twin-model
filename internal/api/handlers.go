package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

// This file holds the HTTP handlers. Each one is a thin adapter: parse the
// request, call the corresponding core.Service method, and write the response.
// All modeling logic lives in core.Service; writeErr centralizes the sentinel→
// HTTP-status mapping so every handler maps errors the same way.

// writeErr maps a core sentinel error to the HTTP status the old handlers used.
// Each case strips its OWN sentinel's "<sentinel>: " prefix (not just the first
// ": " in the string), so a detail that itself starts with a colon-bearing label
// (e.g. "parse: ...", "read tree: ...", "emit: ...") survives intact — see msg().
func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, core.ErrInvalid):
		httpErr(w, http.StatusBadRequest, msg(err, core.ErrInvalid))
	case errors.Is(err, core.ErrNotFound):
		httpErr(w, http.StatusNotFound, msg(err, core.ErrNotFound))
	case errors.Is(err, core.ErrParse):
		httpErr(w, http.StatusUnprocessableEntity, msg(err, core.ErrParse))
	case errors.Is(err, core.ErrReadTree):
		httpErr(w, http.StatusBadGateway, msg(err, core.ErrReadTree))
	case errors.Is(err, core.ErrInternal):
		httpErr(w, http.StatusInternalServerError, msg(err, core.ErrInternal))
	default:
		httpErr(w, http.StatusBadGateway, err.Error())
	}
}

// msg strips the matched sentinel's own "<sentinel>: " prefix (exactly once, from
// the start) so the response text matches the old handlers byte-for-byte, even
// when the detail after the sentinel itself begins with a "label: " that echoes
// words in the sentinel (e.g. ErrReadTree "read tree" + detail "read tree: x").
func msg(err error, sentinel error) string {
	return strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
}

// handleModel serves the AST-as-JSON of a model file read from a committed ref.
func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.ReadModel(r.Context(), r.URL.Query().Get("ref"), r.URL.Query().Get("file"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
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
	resp, err := s.svc.CreateDraft(r.Context(), req.BaseRef)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// handleDraft returns a draft's metadata so a client can re-hydrate it by id.
func (s *Server) handleDraft(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.DraftMeta(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDraftModel serves the AST-as-JSON of a draft's selected file.
func (s *Server) handleDraftModel(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.DraftModel(r.PathValue("id"), r.URL.Query().Get("file"))
	if err != nil {
		writeErr(w, err)
		return
	}
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
	resp, err := s.svc.UpdateDraft(r.PathValue("id"), req.Files)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleValidate returns structured diagnostics for a draft's selected file.
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.DraftValidate(r.PathValue("id"), r.URL.Query().Get("file"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleModelDesign renders the generated ModelDesign XML for a draft file.
func (s *Server) handleModelDesign(w http.ResponseWriter, r *http.Request) {
	xml, err := s.svc.DraftModelDesign(r.PathValue("id"), r.URL.Query().Get("file"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(xml) // Write implies 200
}

// handleDiagram renders Mermaid source for a draft file (?view=types|instances).
func (s *Server) handleDiagram(w http.ResponseWriter, r *http.Request) {
	src, err := s.svc.DraftDiagram(r.PathValue("id"), r.URL.Query().Get("file"), r.URL.Query().Get("view"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(src)) // Write implies 200
}

// handleDiff returns the semantic changelist of a draft file vs its baseRef.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.DraftDiff(r.PathValue("id"), r.URL.Query().Get("file"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleResolved returns the flattened inherited members of a type in a draft.
func (s *Server) handleResolved(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.DraftResolve(r.PathValue("id"), r.URL.Query().Get("file"), r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handlePRs lists the open pull requests on the model repo.
func (s *Server) handlePRs(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.ListPRs(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleRepo returns repo context, the commit identity, and whether proposing is
// possible, so the UI can show which repo/identity it operates as and gate the
// Propose button up front instead of failing at submit.
func (s *Server) handleRepo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.RepoInfo())
}

// handleBranches lists the repo's branches (default first) and its resolved
// default branch, so the footer branch picker can offer the real branches. A
// listing failure is a bad gateway; the SPA degrades to default+current base.
func (s *Server) handleBranches(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.Branches(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handlePropose validates every model file and, if none is lint-red or
// unparseable, commits the fileset and opens a pull request. A lint-red draft is
// rejected with 409 and the blocking diagnostics.
func (s *Server) handlePropose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Branch  string `json:"branch"`
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	// A JSON decode failure must still reject as if branch/title were empty, the
	// same as the original handler's `err != nil || req.Branch == "" || req.Title
	// == ""` check. But encoding/json partially populates the struct before
	// failing (e.g. {"branch":"b","title":"t","message":42} decodes Branch/Title
	// fine and only fails on Message), so an ignored error can leave a
	// non-empty Branch/Title behind and let a malformed body sail through to
	// core.Service.Propose as if it were valid. Zero both fields on any decode
	// error so Propose's existing draft-lookup(404)-then-branch/title(400)
	// ordering still yields the original 400 for a known draft, and 404 for an
	// unknown one — see Task 3 review Finding 2.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Branch, req.Title = "", ""
	}
	resp, err := s.svc.Propose(r.Context(), r.PathValue("id"), req.Branch, req.Title, req.Message)
	if err != nil {
		var ve *core.ValidationError
		if errors.As(err, &ve) {
			writeJSON(w, http.StatusConflict, dto.ConflictResponse{Diagnostics: ve.Blocking})
			return
		}
		if !errors.Is(err, core.ErrNotFound) && !errors.Is(err, core.ErrInvalid) {
			// Not a Service-level sentinel: an OpenPR (host) failure. Render the
			// friendly propose-error payload rather than the generic sentinel body.
			m, detail := core.DescribeProposeError(err)
			writeJSON(w, http.StatusBadGateway, dto.ProposeErrorResponse{Error: m, Detail: detail})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleUnits returns all known engineering units sorted by symbol.
func (s *Server) handleUnits(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.Units())
}

// handleDraftFileRaw serves a draft file's stored canonical YAML as text, for the
// UI's YAML pane. The bytes were canonicalized on PUT, so this is authoritative.
func (s *Server) handleDraftFileRaw(w http.ResponseWriter, r *http.Request) {
	data, err := s.svc.DraftFileRaw(r.PathValue("id"), r.URL.Query().Get("file"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}
