package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/mathieu-sabatier/twin-model/internal/nodeset"
	"github.com/mathieu-sabatier/twin-model/schema"
)

// Server holds the draft store and the git host and wires the HTTP routes. It is
// stateless between requests except the store and the lazily-built catalog.
type Server struct {
	store *Store
	host  GitHost

	catalogOnce sync.Once
	catalog     *nodeset.Catalog
	catalogErr  error
}

// NewServer constructs a Server.
func NewServer(host GitHost, store *Store) *Server {
	return &Server{store: store, host: host}
}

// catalogInstance builds the full companion-spec catalog once and caches it.
// All catalog endpoints share this instance; the NodeSet2 XML is parsed at most
// once per process.
func (s *Server) catalogInstance() (*nodeset.Catalog, error) {
	s.catalogOnce.Do(func() {
		s.catalog, s.catalogErr = nodeset.LoadAll()
	})
	return s.catalog, s.catalogErr
}

// Routes returns the API mux. Stdlib net/http (Go 1.22+ method+path patterns) —
// no framework.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/schema", s.handleSchema)
	mux.HandleFunc("GET /api/model", s.handleModel)
	mux.HandleFunc("POST /api/drafts", s.handleCreateDraft)
	mux.HandleFunc("GET /api/drafts/{id}", s.handleDraft)
	mux.HandleFunc("GET /api/drafts/{id}/model", s.handleDraftModel)
	mux.HandleFunc("PUT /api/drafts/{id}/files", s.handlePutFiles)
	mux.HandleFunc("POST /api/drafts/{id}/validate", s.handleValidate)
	mux.HandleFunc("GET /api/drafts/{id}/preview/modeldesign", s.handleModelDesign)
	mux.HandleFunc("GET /api/drafts/{id}/preview/diagram", s.handleDiagram)
	mux.HandleFunc("GET /api/drafts/{id}/diff", s.handleDiff)
	mux.HandleFunc("GET /api/drafts/{id}/types/{name}/resolved", s.handleResolved)
	mux.HandleFunc("POST /api/drafts/{id}/propose", s.handlePropose)
	mux.HandleFunc("GET /api/units", s.handleUnits)
	mux.HandleFunc("GET /api/catalog", s.handleCatalogList)
	mux.HandleFunc("GET /api/catalog/search", s.handleCatalogSearch)
	mux.HandleFunc("GET /api/catalog/{alias}/types", s.handleCatalogTypes)
	mux.HandleFunc("GET /api/catalog/{alias}/types/{name}", s.handleCatalogType)
	mux.HandleFunc("GET /api/drafts/{id}/file", s.handleDraftFileRaw)
	mux.HandleFunc("GET /api/prs", s.handlePRs)
	mux.HandleFunc("GET /api/repo", s.handleRepo)
	mux.HandleFunc("GET /api/branches", s.handleBranches)
	return mux
}

func (s *Server) handleSchema(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(schema.JSON))
}

// writeJSON writes v as indented JSON with the given status.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// httpErr writes a JSON {"error": msg} with the given status.
func httpErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
