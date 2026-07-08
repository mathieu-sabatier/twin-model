package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
	"github.com/mathieu-sabatier/twin-model/internal/mermaid"
	"github.com/mathieu-sabatier/twin-model/internal/modeldesign"
	"github.com/mathieu-sabatier/twin-model/internal/nodeset"
	"github.com/mathieu-sabatier/twin-model/internal/semdiff"
	"github.com/mathieu-sabatier/twin-model/schema"
)

// Service is the transport-agnostic modeling API. It owns the only server state
// (the draft Store), the git host, and a lazily-built companion-spec catalog.
type Service struct {
	store *Store
	host  GitHost

	catalogOnce sync.Once
	catalog     *nodeset.Catalog
	catalogErr  error
}

// New constructs a Service. host may be nil for transports that only use the
// stateless/catalog operations (they never touch the git host).
func New(host GitHost, store *Store) *Service { return &Service{store: store, host: host} }

func (s *Service) Store() *Store { return s.store }

// Catalog builds the full companion-spec catalog once and caches it.
func (s *Service) Catalog() (*nodeset.Catalog, error) {
	s.catalogOnce.Do(func() { s.catalog, s.catalogErr = nodeset.LoadAll() })
	return s.catalog, s.catalogErr
}

// parseModelResponse parses data and builds the model envelope; a parse error is
// data (ParseError set), never a transport error. (Lifted from Server.buildModelResponse.)
func (s *Service) parseModelResponse(path string, data []byte) dto.ModelResponse {
	m, err := dsl.Parse(path, data)
	if err != nil {
		return dto.ModelResponse{File: path, ParseError: err.Error()}
	}
	if c, err := s.Catalog(); err == nil {
		m.Catalog = c
	}
	return dto.ModelResponse{File: path, Model: dto.FromModel(m), Diagnostics: dto.FromDiagnostics(dsl.Validate(m))}
}

// draftParsedModel resolves a draft's selected file and parses it, returning
// ErrNotFound (unknown draft/file) or ErrParse (structural error). Used by the
// preview/diff/resolve operations for which an unparseable file is a hard error.
func (s *Service) draftParsedModel(id, file string) (*Draft, string, *dsl.Model, error) {
	d, ok := s.store.Get(id)
	if !ok {
		return nil, "", nil, fmt.Errorf("%w: draft not found", ErrNotFound)
	}
	path, data, ok := selectFile(d.Files, file)
	if !ok {
		return nil, "", nil, fmt.Errorf("%w: file not found in draft", ErrNotFound)
	}
	m, err := dsl.Parse(path, data)
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: parse: %s", ErrParse, err.Error())
	}
	return d, path, m, nil
}

// draftFile resolves a draft's selected file without parsing it, returning
// ErrNotFound (unknown draft/file). Used by operations for which a parse error
// is data, not a transport error (model, validate, raw).
func (s *Service) draftFile(id, file string) (*Draft, string, []byte, error) {
	d, ok := s.store.Get(id)
	if !ok {
		return nil, "", nil, fmt.Errorf("%w: draft not found", ErrNotFound)
	}
	path, data, ok := selectFile(d.Files, file)
	if !ok {
		return nil, "", nil, fmt.Errorf("%w: file not found in draft", ErrNotFound)
	}
	return d, path, data, nil
}

// ReadModel returns the AST-as-JSON + diagnostics of a model file at a committed ref.
func (s *Service) ReadModel(ctx context.Context, ref, file string) (dto.ModelResponse, error) {
	if ref == "" {
		return dto.ModelResponse{}, fmt.Errorf("%w: ref is required", ErrInvalid)
	}
	tree, err := s.host.ReadTree(ctx, ref)
	if err != nil {
		return dto.ModelResponse{}, fmt.Errorf("%w: read tree: %s", ErrReadTree, err.Error())
	}
	// Restrict to model files so a non-model path 404s rather than surfacing a
	// parse error — symmetric with draft creation.
	path, data, ok := selectFile(modelFilesOnly(tree), file)
	if !ok {
		return dto.ModelResponse{}, fmt.Errorf("%w: file not found in ref", ErrNotFound)
	}
	return s.parseModelResponse(path, data), nil
}

// ReadModelSource returns the raw YAML bytes of a model file at a committed ref
// — the exact source text, for a caller that needs to round-trip it rather than
// reconstruct it from the AST.
func (s *Service) ReadModelSource(ctx context.Context, ref, file string) ([]byte, error) {
	if ref == "" {
		return nil, fmt.Errorf("%w: ref is required", ErrInvalid)
	}
	tree, err := s.host.ReadTree(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrReadTree, err.Error())
	}
	_, data, ok := selectFile(modelFilesOnly(tree), file)
	if !ok {
		return nil, fmt.Errorf("%w: file in ref", ErrNotFound)
	}
	return data, nil
}

// ListModelFiles lists the model-file paths at a committed ref.
func (s *Service) ListModelFiles(ctx context.Context, ref string) ([]string, error) {
	if ref == "" {
		return nil, fmt.Errorf("%w: ref is required", ErrInvalid)
	}
	tree, err := s.host.ReadTree(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("%w: read tree: %s", ErrReadTree, err.Error())
	}
	return SortedKeys(modelFilesOnly(tree)), nil
}

// ParseModel parses data and builds the model envelope (AST + diagnostics). A
// parse error is returned as data (ParseError set), never as an error.
func (s *Service) ParseModel(file string, data []byte) dto.ModelResponse {
	return s.parseModelResponse(file, data)
}

// PreviewModelDesign renders the generated ModelDesign XML for inline model data
// (not a draft).
func (s *Service) PreviewModelDesign(file string, data []byte) ([]byte, error) {
	m, err := dsl.Parse(file, data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParse, err.Error())
	}
	xml, err := modeldesign.Emit(m)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParse, err.Error())
	}
	return xml, nil
}

// PreviewDiagram renders Mermaid source for inline model data (view=types|instances).
func (s *Service) PreviewDiagram(file string, data []byte, view string) (string, error) {
	m, err := dsl.Parse(file, data)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrParse, err.Error())
	}
	if view == "instances" {
		return mermaid.InstancesDiagram(m), nil
	}
	return mermaid.TypesDiagram(m), nil
}

// ResolveType returns the flattened inherited members of a type parsed from
// inline model data (not a draft).
func (s *Service) ResolveType(file string, data []byte, name string) (dto.ResolvedResponse, error) {
	m, err := dsl.Parse(file, data)
	if err != nil {
		return dto.ResolvedResponse{}, fmt.Errorf("%w: %s", ErrParse, err.Error())
	}
	rms, err := m.ResolveMembers(name)
	if err != nil {
		return dto.ResolvedResponse{}, fmt.Errorf("%w: %s", ErrNotFound, err.Error())
	}
	return dto.ResolvedResponse{Type: name, Members: dto.FromResolvedMembers(rms)}, nil
}

// Schema returns the twinmodel JSON schema document.
func (s *Service) Schema() string { return schema.JSON }

// Units returns all known engineering units sorted by symbol.
func (s *Service) Units() dto.UnitsResponse {
	return dto.UnitsResponse{Units: dto.FromUnits(dsl.Units())}
}

// CatalogList returns the bundled companion specs with version + transitive
// dependency aliases.
func (s *Service) CatalogList() (dto.CatalogListResponse, error) {
	c, err := s.Catalog()
	if err != nil {
		return dto.CatalogListResponse{}, fmt.Errorf("%w: load catalog: %s", ErrInternal, err.Error())
	}
	specs := make([]dto.CatalogSpec, 0, len(nodeset.Registry()))
	for _, sp := range nodeset.Registry() {
		version, pubDate := "", ""
		if ns, ok := c.Namespace(sp.URI); ok {
			version, pubDate = ns.Version, ns.PublicationDate
		}
		deps, err := nodeset.DependencyAliases(sp.URI)
		if err != nil {
			return dto.CatalogListResponse{}, fmt.Errorf("%w: deps: %s", ErrInternal, err.Error())
		}
		specs = append(specs, dto.CatalogSpec{
			Alias: sp.Alias, URI: sp.URI, Version: version,
			PublicationDate: pubDate, Dependencies: deps,
		})
	}
	return dto.CatalogListResponse{Specs: specs}, nil
}

// CatalogTypes lists the ObjectTypes/VariableTypes in one spec, sorted.
func (s *Service) CatalogTypes(alias string) (dto.CatalogTypesResponse, error) {
	c, err := s.Catalog()
	if err != nil {
		return dto.CatalogTypesResponse{}, fmt.Errorf("%w: load catalog: %s", ErrInternal, err.Error())
	}
	spec, ok := nodeset.SpecForRef(alias)
	if !ok {
		return dto.CatalogTypesResponse{}, fmt.Errorf("%w: unknown spec", ErrNotFound)
	}
	names := c.TypeNames(spec.URI)
	sort.Strings(names)
	types := make([]dto.CatalogTypeSummary, 0, len(names))
	for _, n := range names {
		t, ok := c.LookupType(spec.URI, n)
		if !ok {
			continue
		}
		types = append(types, dto.CatalogTypeSummary{Name: t.Name, NodeClass: t.NodeClass, Abstract: t.Abstract})
	}
	return dto.CatalogTypesResponse{Types: types}, nil
}

// CatalogType returns a type's base chain + resolved members.
func (s *Service) CatalogType(alias, name string) (dto.CatalogTypeDetail, error) {
	c, err := s.Catalog()
	if err != nil {
		return dto.CatalogTypeDetail{}, fmt.Errorf("%w: load catalog: %s", ErrInternal, err.Error())
	}
	spec, ok := nodeset.SpecForRef(alias)
	if !ok {
		return dto.CatalogTypeDetail{}, fmt.Errorf("%w: unknown spec", ErrNotFound)
	}
	t, ok := c.LookupType(spec.URI, name)
	if !ok {
		return dto.CatalogTypeDetail{}, fmt.Errorf("%w: type not found in spec", ErrNotFound)
	}
	return dto.FromCatalogType(spec.Alias, spec.URI, t, baseChain(c, t), aliasForURI), nil
}

// aliasForURI resolves a namespace URI to its registry alias, or "" when the URI
// is not a bundled companion spec (ns0, or anything unknown). Used to decide
// whether a member's type is linkable.
func aliasForURI(uri string) string {
	if sp, ok := nodeset.SpecForURI(uri); ok {
		return sp.Alias
	}
	return ""
}

// baseChain walks a companion type's supertypes, most-derived-first. Each hop
// resolves the base's URI→alias via the registry (empty alias for ns0, which is
// not bundled); the walk stops at the first base not present in the catalog.
func baseChain(c *nodeset.Catalog, t dsl.CatalogType) []dto.CatalogTypeRef {
	out := []dto.CatalogTypeRef{}
	uri, name := t.BaseURI, t.BaseName
	seen := map[string]bool{}
	for name != "" {
		key := uri + "|" + name
		if seen[key] {
			break // cycle (malformed data): stop before re-recording the duplicate.
		}
		seen[key] = true
		alias := ""
		if sp, ok := nodeset.SpecForRef(uri); ok {
			alias = sp.Alias
		}
		out = append(out, dto.CatalogTypeRef{Alias: alias, Name: name, URI: uri})
		next, ok := c.LookupType(uri, name)
		if !ok {
			break // base defined in an unloaded namespace (e.g. ns0): recorded, stop.
		}
		uri, name = next.BaseURI, next.BaseName
	}
	return out
}

// CatalogSearch matches types by case-insensitive substring across all specs.
func (s *Service) CatalogSearch(q string) (dto.CatalogSearchResponse, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return dto.CatalogSearchResponse{}, fmt.Errorf("%w: q is required", ErrInvalid)
	}
	c, err := s.Catalog()
	if err != nil {
		return dto.CatalogSearchResponse{}, fmt.Errorf("%w: load catalog: %s", ErrInternal, err.Error())
	}
	kw := strings.ToLower(q)
	hits := []dto.CatalogSearchHit{}
	for _, sp := range nodeset.Registry() {
		names := c.TypeNames(sp.URI)
		sort.Strings(names)
		for _, n := range names {
			if !strings.Contains(strings.ToLower(n), kw) {
				continue
			}
			t, ok := c.LookupType(sp.URI, n)
			if !ok {
				continue
			}
			hits = append(hits, dto.CatalogSearchHit{Alias: sp.Alias, Name: t.Name, NodeClass: t.NodeClass, Abstract: t.Abstract})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Alias != hits[j].Alias {
			return hits[i].Alias < hits[j].Alias
		}
		return hits[i].Name < hits[j].Name
	})
	return dto.CatalogSearchResponse{Hits: hits}, nil
}

// CreateDraft creates a draft from a committed ref's model files.
func (s *Service) CreateDraft(ctx context.Context, baseRef string) (dto.CreateDraftResponse, error) {
	if baseRef == "" {
		return dto.CreateDraftResponse{}, fmt.Errorf("%w: baseRef is required", ErrInvalid)
	}
	tree, err := s.host.ReadTree(ctx, baseRef)
	if err != nil {
		// A missing branch is a user error (they chose/typed a base that does not
		// exist), not a bad gateway: go-git reports "couldn't find remote ref …".
		if strings.Contains(err.Error(), "couldn't find remote ref") {
			return dto.CreateDraftResponse{}, fmt.Errorf("%w: %s", ErrNotFound, fmt.Sprintf("branch %q not found", baseRef))
		}
		return dto.CreateDraftResponse{}, fmt.Errorf("%w: read tree: %s", ErrReadTree, err.Error())
	}
	d := s.store.Create(baseRef, modelFilesOnly(tree))
	return dto.CreateDraftResponse{ID: d.ID, BaseRef: d.BaseRef, Files: SortedKeys(d.Files)}, nil
}

// DraftMeta returns a draft's metadata so a client can re-hydrate it by id.
func (s *Service) DraftMeta(id string) (dto.DraftResponse, error) {
	d, ok := s.store.Get(id)
	if !ok {
		return dto.DraftResponse{}, fmt.Errorf("%w: draft not found", ErrNotFound)
	}
	return dto.DraftResponse{
		ID:        d.ID,
		BaseRef:   d.BaseRef,
		Files:     SortedKeys(d.Files),
		UpdatedAt: d.UpdatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// UpdateDraft updates draft files, canonicalizing each parseable file via
// dsl.Format and keeping raw bytes for any file that does not parse.
func (s *Service) UpdateDraft(id string, files map[string]string) (dto.FilesResponse, error) {
	if len(files) == 0 {
		return dto.FilesResponse{}, fmt.Errorf("%w: files is required", ErrInvalid)
	}
	d, ok := s.store.Update(id, func(d *Draft) {
		for name, content := range files {
			key := resolveWriteKey(d.Files, name)
			d.Files[key] = canonicalize(key, []byte(content))
		}
	})
	if !ok {
		return dto.FilesResponse{}, fmt.Errorf("%w: draft not found", ErrNotFound)
	}
	return dto.FilesResponse{Files: SortedKeys(d.Files)}, nil
}

// DraftModel serves the AST-as-JSON of a draft's selected file.
func (s *Service) DraftModel(id, file string) (dto.ModelResponse, error) {
	d, path, data, err := s.draftFile(id, file)
	if err != nil {
		return dto.ModelResponse{}, err
	}
	resp := s.parseModelResponse(path, data)
	resp.Files = SortedKeys(d.Files) // lets a refreshed client rebuild the file switcher
	return resp, nil
}

// DraftFileRaw serves a draft file's stored canonical YAML bytes, for the UI's
// YAML pane. The bytes were canonicalized on PUT, so this is authoritative.
func (s *Service) DraftFileRaw(id, file string) ([]byte, error) {
	_, _, data, err := s.draftFile(id, file)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// DraftValidate returns structured diagnostics for a draft's selected file.
func (s *Service) DraftValidate(id, file string) (dto.ValidateResponse, error) {
	_, path, data, err := s.draftFile(id, file)
	if err != nil {
		return dto.ValidateResponse{}, err
	}
	m, perr := dsl.Parse(path, data)
	if perr != nil {
		return dto.ValidateResponse{File: path, ParseError: perr.Error()}, nil
	}
	if c, err := s.Catalog(); err == nil {
		m.Catalog = c
	}
	return dto.ValidateResponse{File: path, Diagnostics: dto.FromDiagnostics(dsl.Validate(m))}, nil
}

// DraftModelDesign renders the generated ModelDesign XML for a draft file.
func (s *Service) DraftModelDesign(id, file string) ([]byte, error) {
	_, _, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return nil, err
	}
	xml, err := modeldesign.Emit(m)
	if err != nil {
		return nil, fmt.Errorf("%w: emit: %s", ErrParse, err.Error())
	}
	return xml, nil
}

// DraftDiagram renders Mermaid source for a draft file (view=types|instances).
func (s *Service) DraftDiagram(id, file, view string) (string, error) {
	_, _, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return "", err
	}
	if view == "instances" {
		return mermaid.InstancesDiagram(m), nil
	}
	return mermaid.TypesDiagram(m), nil
}

// DraftDiff returns the semantic changelist of a draft file vs its baseRef.
func (s *Service) DraftDiff(id, file string) (dto.DiffResponse, error) {
	d, path, draftModel, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.DiffResponse{}, err
	}
	// Base side: the same file's baseRef snapshot. A file added in the draft has
	// no base counterpart, so diff against an empty model.
	baseModel := &dsl.Model{}
	if _, baseData, ok := selectFile(d.BaseFiles, path); ok {
		if bm, perr := dsl.Parse(path, baseData); perr == nil {
			baseModel = bm
		}
	}
	changes := semdiff.Diff(baseModel, draftModel)
	return dto.DiffResponse{Changes: changes, Text: semdiff.Render(changes)}, nil
}

// DraftResolve returns the flattened inherited members of a type in a draft.
func (s *Service) DraftResolve(id, file, name string) (dto.ResolvedResponse, error) {
	_, _, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.ResolvedResponse{}, err
	}
	rms, err := m.ResolveMembers(name)
	if err != nil {
		return dto.ResolvedResponse{}, fmt.Errorf("%w: %s", ErrNotFound, err.Error())
	}
	return dto.ResolvedResponse{Type: name, Members: dto.FromResolvedMembers(rms)}, nil
}

// Propose validates every model file and, if none is lint-red or unparseable,
// commits the fileset and opens a pull request. A lint-red draft yields a
// *ValidationError; an OpenPR failure is returned wrapped (adapters call
// DescribeProposeError to render it).
func (s *Service) Propose(ctx context.Context, id, branch, title, msg string) (dto.ProposeResponse, error) {
	d, ok := s.store.Get(id)
	if !ok {
		return dto.ProposeResponse{}, fmt.Errorf("%w: draft not found", ErrNotFound)
	}
	if branch == "" || title == "" {
		return dto.ProposeResponse{}, fmt.Errorf("%w: branch and title are required", ErrInvalid)
	}
	if blocking := s.lintFileset(d.Files); len(blocking) > 0 {
		return dto.ProposeResponse{}, &ValidationError{Blocking: blocking}
	}
	url, err := s.host.OpenPR(ctx, ProposeParams{BaseRef: d.BaseRef, Branch: branch, Title: title, Message: msg, Files: d.Files})
	if err != nil {
		return dto.ProposeResponse{}, err // *PRError or transport error; adapter renders
	}
	return dto.ProposeResponse{URL: url}, nil
}

// lintFileset returns the blocking diagnostics across every model file in
// deterministic order: a synthetic parse-error diagnostic for an unparseable
// file, plus each error-severity diagnostic from a parseable one. An empty
// result means the fileset is safe to propose.
func (s *Service) lintFileset(files map[string][]byte) []dto.Diagnostic {
	var blocking []dto.Diagnostic
	for _, path := range sortedKeys(files) {
		m, err := dsl.Parse(path, files[path])
		if err != nil {
			blocking = append(blocking, dto.Diagnostic{
				Code: "parse-error", Severity: "error", File: path, Path: path, Message: err.Error(),
			})
			continue
		}
		if c, err := s.Catalog(); err == nil {
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

// DescribeProposeError maps an OpenPR failure to a friendly, actionable message
// and a raw detail string. The branch/commit are created before the PR is
// opened, so we tell the user their local work landed even though the PR step
// failed. QA finding M3.
func DescribeProposeError(err error) (msg, detail string) {
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

// ListPRs lists the open pull requests on the model repo.
func (s *Service) ListPRs(ctx context.Context) (dto.PRListResponse, error) {
	prs, err := s.host.ListPRs(ctx)
	if err != nil {
		return dto.PRListResponse{}, fmt.Errorf("list prs: %s", err.Error())
	}
	return dto.PRListResponse{PRs: prs}, nil
}

// RepoInfo returns repo context, the commit identity, and whether proposing is
// possible, so the UI can show which repo/identity it operates as and gate the
// Propose button up front instead of failing at submit.
func (s *Service) RepoInfo() dto.RepoInfo { return s.host.Info() }

// Branches lists the repo's branches (default first) and its resolved default
// branch, so the footer branch picker can offer the real branches. A listing
// failure is wrapped for the adapter to render as its own transport error.
func (s *Service) Branches(ctx context.Context) (dto.BranchList, error) {
	list, err := s.host.Branches(ctx)
	if err != nil {
		return dto.BranchList{}, fmt.Errorf("list branches: %s", err.Error())
	}
	return list, nil
}
