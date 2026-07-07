package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/api/dto"
	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/nodeset"
)

// handleCatalogList serves the bundled companion specs with version + transitive
// dependency aliases (GET /api/catalog).
func (s *Server) handleCatalogList(w http.ResponseWriter, _ *http.Request) {
	c, err := s.catalogInstance()
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "load catalog: "+err.Error())
		return
	}
	specs := make([]dto.CatalogSpec, 0, len(nodeset.Registry()))
	for _, sp := range nodeset.Registry() {
		version, pubDate := "", ""
		if ns, ok := c.Namespace(sp.URI); ok {
			version, pubDate = ns.Version, ns.PublicationDate
		}
		deps, err := nodeset.DependencyAliases(sp.URI)
		if err != nil {
			httpErr(w, http.StatusInternalServerError, "deps: "+err.Error())
			return
		}
		specs = append(specs, dto.CatalogSpec{
			Alias: sp.Alias, URI: sp.URI, Version: version,
			PublicationDate: pubDate, Dependencies: deps,
		})
	}
	writeJSON(w, http.StatusOK, dto.CatalogListResponse{Specs: specs})
}

// handleCatalogTypes lists the ObjectTypes/VariableTypes in one spec, sorted.
func (s *Server) handleCatalogTypes(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalogInstance()
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "load catalog: "+err.Error())
		return
	}
	spec, ok := nodeset.SpecForRef(r.PathValue("alias"))
	if !ok {
		httpErr(w, http.StatusNotFound, "unknown spec")
		return
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
	writeJSON(w, http.StatusOK, dto.CatalogTypesResponse{Types: types})
}

// handleCatalogType returns a type's base chain + resolved members.
func (s *Server) handleCatalogType(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalogInstance()
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "load catalog: "+err.Error())
		return
	}
	spec, ok := nodeset.SpecForRef(r.PathValue("alias"))
	if !ok {
		httpErr(w, http.StatusNotFound, "unknown spec")
		return
	}
	t, ok := c.LookupType(spec.URI, r.PathValue("name"))
	if !ok {
		httpErr(w, http.StatusNotFound, "type not found in spec")
		return
	}
	writeJSON(w, http.StatusOK, dto.FromCatalogType(spec.Alias, spec.URI, t, baseChain(c, t), aliasForURI))
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

// handleCatalogSearch matches types by case-insensitive substring across all
// specs (GET /api/catalog/search?q=).
func (s *Server) handleCatalogSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		httpErr(w, http.StatusBadRequest, "q is required")
		return
	}
	c, err := s.catalogInstance()
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "load catalog: "+err.Error())
		return
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
	writeJSON(w, http.StatusOK, dto.CatalogSearchResponse{Hits: hits})
}
