package nodeset

import (
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

// Catalog is a cross-spec index of companion-spec types, keyed by absolute
// namespace URI + identifier. It implements dsl.Catalog.
type Catalog struct {
	namespaces map[string]dsl.CatalogNamespace // by URI
	byID       map[string]*entry               // key: URI + "|" + id (e.g. "...DI/|i=1002")
	byName     map[string]*entry               // key: URI + "|" + browseName
	bySymbolic map[string]*entry               // key: URI + "|" + SymbolicName (DSL uses SymbolicNames; browse-keyed byName misses e.g. ObjectsFolder)
}

type entry struct {
	ref      NodeRef // this node's own (URI, id)
	class    string
	browse   string // un-prefixed browse name
	abstract bool
	baseURI  string
	baseName string
	members  []dsl.CatalogMember // filled in Task 5
	enum     []dsl.EnumMember    // populated for DataType entries by computeEnums
	node     *Node
	set      *NodeSet
}

// NewCatalog indexes one or more parsed NodeSets and resolves identity + base
// chains across them.
func NewCatalog(sets ...*NodeSet) (*Catalog, error) {
	c := &Catalog{
		namespaces: map[string]dsl.CatalogNamespace{},
		byID:       map[string]*entry{},
		byName:     map[string]*entry{},
		bySymbolic: map[string]*entry{},
	}
	// Pass 1: namespaces + identity.
	for _, ns := range sets {
		for _, m := range ns.Models {
			c.namespaces[m.ModelURI] = dsl.CatalogNamespace{
				URI: m.ModelURI, Version: m.Version, PublicationDate: normDate(m.PublicationDate),
			}
		}
		for i := range ns.Nodes {
			n := &ns.Nodes[i]
			ref, err := ns.resolveNodeID(n.NodeID)
			if err != nil {
				return nil, err
			}
			e := &entry{ref: ref, class: n.Class, browse: unprefixBrowse(n.BrowseName), abstract: n.IsAbstract, node: n, set: ns}
			c.byID[ref.URI+"|"+ref.ID] = e
			c.byName[ref.URI+"|"+e.browse] = e
			if sym := n.SymbolicName; sym != "" && sym != e.browse {
				c.bySymbolic[ref.URI+"|"+sym] = e
			}
		}
	}
	// Pass 2: base chains (HasSubtype, inverse -> supertype).
	for _, e := range c.byID {
		for _, r := range e.node.Refs {
			if e.set.resolveRefType(r) == refHasSubtype && !r.Forward {
				base, err := e.set.resolveNodeID(r.Target)
				if err != nil {
					return nil, err
				}
				e.baseURI, e.baseName = base.URI, c.nameFor(base)
			}
		}
	}
	// Pass 2.5: resolve enum values per DataType (precedence: Definition >
	// EnumValues > EnumStrings), so members can carry their DataType's values.
	c.computeEnums()
	// Pass 3: own members. An instance declaration is a node reached by a forward
	// reference whose type is a subtype of Aggregates (HasComponent/HasProperty and
	// their subtypes) — NOT just the exact HasComponent/HasProperty types. Companion
	// specs like ISA-95 attach members through custom hierarchical reference types
	// (HasISA95Attribute, MadeUpOfEquipment, …); matching only i=46/i=47 dropped them.
	memberBearing, propertyKind := c.refTypeClosures()
	for _, e := range c.byID {
		if e.class != "ObjectType" && e.class != "VariableType" {
			continue
		}
		for _, r := range e.node.Refs {
			if !r.Forward {
				continue
			}
			rtRef, err := e.set.resolveNodeID(e.set.resolveRefType(r))
			if err != nil {
				return nil, err
			}
			rtKey := rtRef.URI + "|" + rtRef.ID
			if !memberBearing[rtKey] {
				continue
			}
			target, err := e.set.resolveNodeID(r.Target)
			if err != nil {
				return nil, err
			}
			te, ok := c.byID[target.URI+"|"+target.ID]
			if !ok {
				continue // target defined in a spec we didn't load; skip
			}
			typeURI, typeName := c.typeDefOf(te)
			e.members = append(e.members, dsl.CatalogMember{
				Name:        te.browse,
				Kind:        kindFor(propertyKind[rtKey], te.class),
				Placeholder: te.modellingRule() == ruleOptionalPlace || te.modellingRule() == ruleMandatoryPlace,
				TypeURI:     typeURI,
				TypeName:    typeName,
				Enum:        c.enumFor(te),
			})
		}
	}
	return c, nil
}

// typeDefOf returns the (URI, browseName) of a node's HasTypeDefinition target.
// Returns "","" when unresolvable.
func (c *Catalog) typeDefOf(e *entry) (uri, name string) {
	for _, r := range e.node.Refs {
		if !r.Forward || e.set.resolveRefType(r) != refHasTypeDefinition {
			continue
		}
		ref, err := e.set.resolveNodeID(r.Target)
		if err != nil {
			return "", ""
		}
		if te, ok := c.byID[ref.URI+"|"+ref.ID]; ok {
			return te.ref.URI, te.browse
		}
		return "", "" // target defined in a spec we didn't load
	}
	return "", ""
}

// refTypeClosures computes, over all indexed reference types, which ones are
// member-bearing (subtypes of Aggregates, i=44) and which imply a property member
// (subtypes of HasProperty, i=46). Both are keyed by absolute "URI|id". The ns0
// core reference types are not bundled, so their branch is seeded directly.
func (c *Catalog) refTypeClosures() (memberBearing, propertyKind map[string]bool) {
	core := func(id string) string { return OpcUaCoreURI + "|" + id }
	// Aggregates subtree in ns0: Aggregates -> {HasComponent, HasProperty,
	// HasOrderedComponent}. HasProperty is also the property-kind seed.
	memberBearing = map[string]bool{core("i=44"): true, core("i=46"): true, core("i=47"): true, core("i=49"): true}
	propertyKind = map[string]bool{core("i=46"): true}

	// supertype edge (child key -> parent key) for every bundled reference type.
	super := map[string]string{}
	var keys []string
	for k, e := range c.byID {
		if e.class != "ReferenceType" {
			continue
		}
		keys = append(keys, k)
		for _, r := range e.node.Refs {
			if e.set.resolveRefType(r) == refHasSubtype && !r.Forward {
				if base, err := e.set.resolveNodeID(r.Target); err == nil {
					super[k] = base.URI + "|" + base.ID
				}
			}
		}
	}
	// Fixpoint: propagate closure membership up each supertype chain.
	for changed := true; changed; {
		changed = false
		for _, k := range keys {
			s, ok := super[k]
			if !ok {
				continue
			}
			if memberBearing[s] && !memberBearing[k] {
				memberBearing[k] = true
				changed = true
			}
			if propertyKind[s] && !propertyKind[k] {
				propertyKind[k] = true
				changed = true
			}
		}
	}
	return memberBearing, propertyKind
}

// nameFor returns a browse name for a resolved ref, consulting the index.
func (c *Catalog) nameFor(ref NodeRef) string {
	if e, ok := c.byID[ref.URI+"|"+ref.ID]; ok {
		return e.browse
	}
	return ""
}

// Namespace implements dsl.Catalog.
func (c *Catalog) Namespace(uri string) (dsl.CatalogNamespace, bool) {
	n, ok := c.namespaces[uri]
	return n, ok
}

// LookupType implements dsl.Catalog.
func (c *Catalog) LookupType(uri, name string) (dsl.CatalogType, bool) {
	e, ok := c.byName[uri+"|"+name]
	if !ok {
		e, ok = c.bySymbolic[uri+"|"+name]
	}
	if !ok {
		return dsl.CatalogType{}, false
	}
	return dsl.CatalogType{
		NamespaceURI: e.ref.URI, Name: e.browse, NodeClass: e.class, Abstract: e.abstract,
		BaseURI: e.baseURI, BaseName: e.baseName, Members: c.flatten(e),
	}, true
}

// unprefixBrowse drops the "<nsIndex>:" prefix from a BrowseName.
func unprefixBrowse(b string) string {
	if _, name, ok := strings.Cut(b, ":"); ok {
		return name
	}
	return b
}

// normDate turns a NodeSet2 dateTime ("2022-11-01T00:00:00Z") into "YYYY-MM-DD".
func normDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

var _ dsl.Catalog = (*Catalog)(nil)

// flatten returns a type's own + inherited members, most-derived-wins by name.
func (c *Catalog) flatten(e *entry) []dsl.CatalogMember {
	var out []dsl.CatalogMember
	seen := map[string]bool{}
	visited := map[string]bool{}
	cur := e
	for cur != nil {
		if visited[cur.ref.URI+"|"+cur.ref.ID] {
			break
		}
		visited[cur.ref.URI+"|"+cur.ref.ID] = true
		for _, m := range cur.members {
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true
			out = append(out, m)
		}
		if cur.baseURI == "" {
			break
		}
		cur = c.byID[cur.baseURI+"|"+idForName(c, cur.baseURI, cur.baseName)]
	}
	return out
}

// idForName finds the identifier of a (uri, browseName) among indexed entries.
func idForName(c *Catalog, uri, name string) string {
	if e, ok := c.byName[uri+"|"+name]; ok {
		return e.ref.ID
	}
	return ""
}

// TypeNames returns the browse names of ObjectTypes/VariableTypes defined in a
// namespace (sorted by the caller).
func (c *Catalog) TypeNames(uri string) []string {
	if uri == OpcUaCoreURI {
		return nil // ns0 is loaded for resolution but never listed
	}
	var out []string
	for _, e := range c.byName {
		if e.ref.URI == uri && (e.class == "ObjectType" || e.class == "VariableType") {
			out = append(out, e.browse)
		}
	}
	return out
}

// computeEnums fills entry.enum for every DataType, applying the precedence
// <Definition> > EnumValues > EnumStrings. EnumStrings/EnumValues live on
// separate UAVariable nodes linked from the DataType by a forward HasProperty
// ref, so we walk the DataType's forward refs to find them by browse name.
func (c *Catalog) computeEnums() {
	for _, e := range c.byID {
		if e.class != "DataType" {
			continue
		}
		// 1. <Definition> on the DataType itself.
		if len(e.node.EnumMembers) > 0 {
			e.enum = toEnumMembers(e.node.EnumMembers)
			continue
		}
		// 2/3. EnumValues (preferred) or EnumStrings child variable.
		var values []EnumMember
		var strs []string
		for _, r := range e.node.Refs {
			if !r.Forward {
				continue
			}
			tgt, err := e.set.resolveNodeID(r.Target)
			if err != nil {
				continue
			}
			ce, ok := c.byID[tgt.URI+"|"+tgt.ID]
			if !ok {
				continue
			}
			switch ce.browse {
			case "EnumValues":
				values = ce.node.EnumValues
			case "EnumStrings":
				strs = ce.node.EnumStrings
			}
		}
		switch {
		case len(values) > 0:
			e.enum = toEnumMembers(values)
		case len(strs) > 0:
			e.enum = stringsToEnumMembers(strs)
		}
	}
}

// enumFor resolves a member target's DataType attribute to its DataType entry
// and returns that DataType's enum members, or nil when the target has no
// DataType, the DataType is not loaded, or it is not an enumeration.
func (c *Catalog) enumFor(te *entry) []dsl.EnumMember {
	if te.node.DataType == "" {
		return nil
	}
	dt, err := te.set.resolveNodeID(te.node.DataType)
	if err != nil {
		return nil
	}
	if dte, ok := c.byID[dt.URI+"|"+dt.ID]; ok {
		return dte.enum
	}
	return nil
}

// toEnumMembers converts parsed nodeset enum members to the dsl contract type.
func toEnumMembers(in []EnumMember) []dsl.EnumMember {
	out := make([]dsl.EnumMember, len(in))
	for i, m := range in {
		out[i] = dsl.EnumMember{Name: m.Name, Value: m.Value}
	}
	return out
}

// stringsToEnumMembers normalizes an EnumStrings list to members with implicit
// 0..n-1 values.
func stringsToEnumMembers(in []string) []dsl.EnumMember {
	out := make([]dsl.EnumMember, len(in))
	for i, s := range in {
		out[i] = dsl.EnumMember{Name: s, Value: int64(i)}
	}
	return out
}

// kindFor maps a member's reference (property-kind or not) + target node class to
// a dsl.Kind. propertyRef is true when the reference type is HasProperty or a
// subtype of it; otherwise the kind follows the target node class.
func kindFor(propertyRef bool, targetClass string) dsl.Kind {
	switch {
	case propertyRef:
		return dsl.KindProperty
	case targetClass == "Object":
		return dsl.KindObject
	case targetClass == "Method":
		return dsl.KindMethod
	default:
		return dsl.KindVariable
	}
}

// modellingRule returns the target's HasModellingRule nodeid ("" if none).
func (e *entry) modellingRule() string {
	for _, r := range e.node.Refs {
		if e.set.resolveRefType(r) == refHasModellingRule && r.Forward {
			ref, err := e.set.resolveNodeID(r.Target)
			if err == nil {
				return ref.ID
			}
		}
	}
	return ""
}
