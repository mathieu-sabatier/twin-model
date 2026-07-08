package api

import "net/http"

// This file holds the HTTP adapters for the catalog endpoints. All catalog
// logic (loading, base-chain walking, alias resolution) lives in core.Service.

// handleCatalogList serves the bundled companion specs with version + transitive
// dependency aliases (GET /api/catalog).
func (s *Server) handleCatalogList(w http.ResponseWriter, _ *http.Request) {
	resp, err := s.svc.CatalogList()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleCatalogTypes lists the ObjectTypes/VariableTypes in one spec, sorted.
func (s *Server) handleCatalogTypes(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.CatalogTypes(r.PathValue("alias"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleCatalogType returns a type's base chain + resolved members.
func (s *Server) handleCatalogType(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.CatalogType(r.PathValue("alias"), r.PathValue("name"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleCatalogSearch matches types by case-insensitive substring across all
// specs (GET /api/catalog/search?q=).
func (s *Server) handleCatalogSearch(w http.ResponseWriter, r *http.Request) {
	resp, err := s.svc.CatalogSearch(r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
