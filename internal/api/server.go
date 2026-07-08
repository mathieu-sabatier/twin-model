package api

import (
	"encoding/json"
	"net/http"

	"github.com/mathieu-sabatier/twin-model/internal/core"
)

// Server adapts core.Service to HTTP. It is stateless except for the Service it wraps.
type Server struct{ svc *core.Service }

// NewServer keeps its (host, store) signature so existing callers/tests are
// unchanged; it builds the core.Service internally.
func NewServer(host GitHost, store *Store) *Server { return &Server{svc: core.New(host, store)} }

// NewServerFromService adapts an existing core.Service to HTTP.
func NewServerFromService(svc *core.Service) *Server { return &Server{svc: svc} }

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
	_, _ = w.Write([]byte(s.svc.Schema()))
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
