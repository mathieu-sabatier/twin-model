package dsl

import "fmt"

// ResolvedMember is a member of a type after inheritance flattening, tagged with
// the local type that declares it.
type ResolvedMember struct {
	*Member
	DeclaredIn string
}

// ResolveMembers flattens the member set of a local object type: it walks the
// local base chain (most-derived first) and includes inherited members, with
// most-derived-wins on name collisions.
//
// When a base resolves to RefImportResolved (a companion-spec type found in the
// catalog), ResolveMembers crosses into the catalog and appends the companion
// type's pre-flattened members as synthesized *Member values (DeclaredIn set to
// "alias:Name"). Catalog members are already fully flattened so the walk stops
// there.
//
// Imported bases without a catalog (OpcUa:BaseObjectType, etc.) remain opaque —
// only members declared in the local type chain are returned.
func (m *Model) ResolveMembers(typeName string) ([]ResolvedMember, error) {
	ot, ok := m.localObjectType(typeName)
	if !ok {
		return nil, fmt.Errorf("resolve members: %q is not a local object type", typeName)
	}
	var out []ResolvedMember
	seen := map[string]bool{}    // member names already taken (derived wins)
	visited := map[string]bool{} // types walked (cycle guard)
	cur := ot
	for cur != nil {
		if visited[cur.Name] {
			break // defensive: inheritance cycles are a separate lint error
		}
		visited[cur.Name] = true
		for _, mem := range cur.Members {
			if seen[mem.Name] {
				continue
			}
			seen[mem.Name] = true
			out = append(out, ResolvedMember{Member: mem, DeclaredIn: cur.Name})
		}
		if cur.Base.IsZero() {
			break
		}
		r := m.ResolveType(cur.Base)
		if r.Kind == RefImportResolved {
			ct, ok := m.CatalogType(r)
			if ok {
				for _, cm := range ct.Members {
					if seen[cm.Name] {
						continue
					}
					seen[cm.Name] = true
					out = append(out, ResolvedMember{Member: synthMember(cm), DeclaredIn: r.Alias + ":" + r.Name})
				}
			}
			break // catalog members are already flattened; nothing further to walk
		}
		if r.Kind != RefLocal {
			break // imported-without-catalog / unknown base: opaque
		}
		next, ok := m.localObjectType(r.Name)
		if !ok {
			break
		}
		cur = next
	}
	return out, nil
}

// TypeHasMember reports whether the type referenced by t declares a member with
// the given name, following the local inheritance chain or a catalog-resolved
// import. It is the shared primitive behind hierarchy validation and export.
func (m *Model) TypeHasMember(t TypeRef, name string) bool {
	switch r := m.ResolveType(t); r.Kind {
	case RefLocal:
		resolved, err := m.ResolveMembers(r.Name)
		if err != nil {
			return false
		}
		for _, rm := range resolved {
			if rm.Name == name {
				return true
			}
		}
	case RefImportResolved:
		if ct, ok := m.CatalogType(r); ok {
			for _, cm := range ct.Members {
				if cm.Name == name {
					return true
				}
			}
		}
	}
	return false
}

// synthMember builds a minimal *Member view of a companion-spec member, enough
// for instance value/child validation (name, kind, placeholder-ness).
func synthMember(cm CatalogMember) *Member {
	mem := &Member{Name: cm.Name, Kind: cm.Kind}
	if cm.Placeholder {
		mem.BrowseName = "<" + cm.Name + ">" // makes IsPlaceholder() true; base name matches
	}
	return mem
}
