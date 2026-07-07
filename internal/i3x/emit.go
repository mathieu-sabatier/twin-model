package i3x

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

// FileNames lists the emitted documents in deterministic order.
var FileNames = []string{
	"info.json",
	"namespaces.json",
	"relationshiptypes.json",
	"objecttypes.json",
	"objects.json",
}

// Bundle holds the emitted i3X documents keyed by filename.
type Bundle struct {
	files map[string][]byte
}

// Files returns the filename → bytes map (each value is indented JSON with a
// trailing newline).
func (b Bundle) Files() map[string][]byte { return b.files }

// File returns a single document's bytes (nil if absent).
func (b Bundle) File(name string) []byte { return b.files[name] }

// extRef is an imported (non-local) node the model references but does not
// define — a standard OPC UA (ns0) or DI node, keyed by its elementId.
type extRef struct {
	ns   string
	name string // BrowseName / SymbolicId
}

// emitter carries the model through the build so helpers can resolve type
// references without threading it everywhere. It also records the imported nodes
// referenced during the build, so they can be emitted as self-containing stubs.
type emitter struct {
	m        *dsl.Model
	extTypes map[string]extRef // referenced imported object *types* (elementId → ref)
	extObjs  map[string]extRef // referenced imported *objects* (elementId → ref), e.g. ObjectsFolder
}

// Emit transpiles a validated Model to the i3X document Bundle. It is a pure
// function of the AST: no I/O, time, or randomness.
func Emit(m *dsl.Model) (Bundle, error) {
	e := &emitter{
		m:        m,
		extTypes: map[string]extRef{},
		extObjs:  map[string]extRef{},
	}

	// Build object types and objects first: this records the imported nodes they
	// reference (extTypes/extObjs) so we can then append reference stubs that make
	// the bundle self-contained.
	objectTypes := e.objectTypes()
	objects := e.objects()
	objectTypes = append(objectTypes, e.stubObjectTypes()...)
	objects = append(objects, e.stubObjects()...)

	docs := map[string]any{
		"info.json":              e.info(),
		"namespaces.json":        e.namespaces(),
		"relationshiptypes.json": e.relationshipTypes(),
		"objecttypes.json":       objectTypes,
		"objects.json":           objects,
	}
	files := make(map[string][]byte, len(docs))
	for name, v := range docs {
		b, err := marshal(v)
		if err != nil {
			return Bundle{}, err
		}
		files[name] = b
	}
	return Bundle{files: files}, nil
}

// marshal renders v as 2-space-indented JSON with a trailing newline, matching
// the repo's golden style. HTML escaping is disabled so `<`/`>`/`&` stay literal.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil { // Encode appends the trailing newline
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- info -------------------------------------------------------------------

func (e *emitter) info() infoDoc {
	return infoDoc{
		Name:            e.m.Name,
		Version:         e.m.Version,
		PublicationDate: e.m.PublicationDate,
		I3XVersion:      "1.0",
	}
}

// --- namespaces -------------------------------------------------------------

func (e *emitter) namespaces() []namespaceDoc {
	out := []namespaceDoc{{URI: e.m.Namespace, DisplayName: e.m.Name}}
	for _, im := range e.m.Imports {
		dn := im.Alias
		if im.URI == opcUaNS {
			dn = "OPC UA"
		}
		out = append(out, namespaceDoc{URI: im.URI, DisplayName: dn})
	}
	return out
}

// --- relationship types -----------------------------------------------------

// relationshipTypes emits the OPC UA reference types the model relies on:
// HasComponent (composition) and Organizes (instances under a folder), each with
// its inverse.
func (e *emitter) relationshipTypes() []relationshipTypeDoc {
	return []relationshipTypeDoc{
		{ElementID: ns0("HasComponent"), DisplayName: "HasComponent", NamespaceURI: opcUaNS, RelationshipID: "HasComponent", ReverseOf: "ComponentOf"},
		{ElementID: ns0("Organizes"), DisplayName: "Organizes", NamespaceURI: opcUaNS, RelationshipID: "Organizes", ReverseOf: "OrganizedBy"},
	}
}

// --- object types -----------------------------------------------------------

func (e *emitter) objectTypes() []objectTypeDoc {
	out := make([]objectTypeDoc, 0, len(e.m.ObjectTypes))
	for _, ot := range e.m.ObjectTypes {
		out = append(out, objectTypeDoc{
			ElementID:    elementID(e.m.Namespace, ot.Name),
			DisplayName:  ot.Name,
			NamespaceURI: e.m.Namespace,
			SourceTypeID: e.typeRefElementID(ot.Base),
			Version:      e.m.Version,
			Schema:       e.typeSchema(ot),
		})
	}
	return out
}

// typeSchema builds the JSON Schema for one object type. Only the type's own
// members appear (inheritance is expressed via sourceTypeId); enums used by own
// members are inlined under $defs.
func (e *emitter) typeSchema(ot *dsl.ObjectType) *jobj {
	s := newObj()
	s.set("$id", elementID(e.m.Namespace, ot.Name))
	s.set("type", "object")
	s.set("title", ot.Name)
	if ot.Doc != "" {
		s.set("description", ot.Doc)
	}
	if ot.Abstract {
		s.set("x-opcua", newObj().set("abstract", true))
	}

	props := newObj()
	var required []string
	defs := newObj()
	seenEnum := map[string]bool{}
	var methods []any

	for _, mem := range ot.Members {
		switch mem.Kind {
		case dsl.KindMethod:
			methods = append(methods, e.method(mem))
		case dsl.KindObject:
			props.set(mem.Name, e.composition(mem))
		default: // property or variable → a leaf attribute
			props.set(mem.Name, e.leaf(mem, defs, seenEnum))
			if mem.Rule == dsl.RuleMandatory {
				required = append(required, mem.Name)
			}
		}
	}

	s.set("properties", props)
	if len(required) > 0 {
		s.set("required", required)
	}
	if defs.len() > 0 {
		s.set("$defs", defs)
	}
	if len(methods) > 0 {
		s.set("x-opcuaMethods", methods)
	}
	return s
}

// leaf builds a property/variable attribute value. Enum-typed members become a
// $ref into $defs (registered on first use); built-ins map to JSON Schema types;
// anything else falls back to "string" with the raw dataType preserved.
func (e *emitter) leaf(mem *dsl.Member, defs *jobj, seenEnum map[string]bool) *jobj {
	pv := newObj()
	r := e.m.ResolveType(mem.Type)

	switch {
	case r.Kind == dsl.RefLocal && e.enum(r.Name) != nil:
		pv.set("$ref", "#/$defs/"+r.Name)
		if !seenEnum[r.Name] {
			seenEnum[r.Name] = true
			defs.set(r.Name, e.enumDef(e.enum(r.Name)))
		}
		pv.set("readOnly", e.readOnly(mem))
	case r.Kind == dsl.RefBuiltin:
		typ, format, _ := builtinJSONType(r.Name)
		if mem.Rule == dsl.RuleOptional {
			pv.set("type", []any{typ, "null"})
		} else {
			pv.set("type", typ)
		}
		if format != "" {
			pv.set("format", format)
		}
		pv.set("readOnly", e.readOnly(mem))
		if mem.Unit != "" {
			if u, ok := dsl.LookupUnit(mem.Unit); ok {
				pv.set("engineeringUnits", newObj().
					set("unitId", u.UnitID).
					set("displayName", u.DisplayName).
					set("namespaceUri", uneceNS))
			}
		}
		if mem.Kind == dsl.KindProperty {
			pv.set("x-opcua", newObj().set("nodeClass", "Property"))
		}
	default:
		pv.set("type", "string")
		pv.set("readOnly", e.readOnly(mem))
		pv.set("x-opcua", newObj().set("dataType", mem.Type.Raw))
	}
	return pv
}

// readOnly reports the JSON Schema readOnly flag: r (default) → true, rw → false.
func (e *emitter) readOnly(mem *dsl.Member) bool {
	return mem.Access != dsl.AccessReadWrite
}

// enumDef renders an enum as a $defs entry.
func (e *emitter) enumDef(en *dsl.Enum) *jobj {
	ids := make([]int, len(en.Values))
	names := make([]string, len(en.Values))
	for i, v := range en.Values {
		ids[i] = v.Identifier
		names[i] = v.Name
	}
	def := newObj().
		set("type", "integer").
		set("enum", ids).
		set("x-enumNames", names)
	if en.Doc != "" {
		def.set("description", en.Doc)
	}
	return def
}

// composition builds the attribute for an object-kind member (a composed
// sub-object / folder). A placeholder child (Zone<No>) becomes
// additionalProperties + an x-opcua-placeholder annotation.
func (e *emitter) composition(mem *dsl.Member) *jobj {
	comp := newObj()
	comp.set("x-opcua", newObj().
		set("nodeClass", "Object").
		set("typeDefinition", e.typeRefElementID(mem.Type)).
		set("composition", true))
	comp.set("type", "object")
	if ph := placeholderChild(mem); ph != nil {
		comp.set("additionalProperties", newObj().set("$ref", e.typeRefElementID(ph.Type)))
		comp.set("x-opcua-placeholder", newObj().
			set("symbolicName", ph.Name).
			set("browseNamePattern", ph.BrowseName).
			set("rule", modellingRuleName(ph.Rule)))
	}
	return comp
}

// method records a method member losslessly under x-opcuaMethods (it is not an
// i3X node). Empty description / argument lists are omitted.
func (e *emitter) method(mem *dsl.Member) *jobj {
	mth := newObj().set("name", mem.Name)
	if mem.Doc != "" {
		mth.set("description", mem.Doc)
	}
	if len(mem.In) > 0 {
		mth.set("in", e.args(mem.In))
	}
	if len(mem.Out) > 0 {
		mth.set("out", e.args(mem.Out))
	}
	return mth
}

func (e *emitter) args(args []dsl.Argument) []any {
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = newObj().set("name", a.Name).set("type", a.Type.Name)
	}
	return out
}

// --- objects (instance topology) --------------------------------------------

func (e *emitter) objects() []objectDoc {
	var out []objectDoc
	for _, inst := range e.m.Instances {
		instID := elementID(e.m.Namespace, inst.Name)
		out = append(out, objectDoc{
			ElementID:     instID,
			DisplayName:   inst.Name,
			TypeElementID: e.typeRefElementID(inst.Type),
			ParentID:      e.instanceParent(inst.Under),
			IsComposition: false,
		})
		out = append(out, e.composedObjects(inst, instID)...)
	}
	return out
}

// composedObjects emits an instance's composed sub-objects: first the object-kind
// members materialized from the (flattened) type, then any instantiated
// placeholder children. Both sit directly under the instance, matching the
// SymbolicId (e.g. Furnace02_Zones, Furnace02_Zone1).
func (e *emitter) composedObjects(inst *dsl.Instance, parentID string) []objectDoc {
	var out []objectDoc
	if r := e.m.ResolveType(inst.Type); r.Kind == dsl.RefLocal {
		if resolved, err := e.m.ResolveMembers(r.Name); err == nil {
			for _, rm := range resolved {
				if rm.Kind != dsl.KindObject {
					continue
				}
				out = append(out, objectDoc{
					ElementID:     elementID(e.m.Namespace, symbolicID(inst.Name, rm.Name)),
					DisplayName:   rm.Name,
					TypeElementID: e.typeRefElementID(rm.Type),
					ParentID:      parentID,
					IsComposition: true,
				})
			}
		}
	}
	for _, ch := range inst.Children {
		out = append(out, objectDoc{
			ElementID:     elementID(e.m.Namespace, symbolicID(inst.Name, ch.Name)),
			DisplayName:   ch.Name,
			TypeElementID: e.placeholderTypeElementID(inst, ch.Of),
			ParentID:      parentID,
			IsComposition: true,
		})
	}
	return out
}

// instanceParent resolves an instance's `under` target to an elementId: an
// imported/core folder (ObjectsFolder), or a sibling declared instance (nesting).
// An imported parent is recorded as a referenced object so it gets a stub.
func (e *emitter) instanceParent(under dsl.TypeRef) string {
	switch r := e.m.ResolveType(under); r.Kind {
	// RefImportUnknown is treated exactly like RefImport here: an under: target
	// such as OpcUa:ObjectsFolder is a valid ns0 object instance, but it resolves
	// to RefImportUnknown because ns0 indexes it by BrowseName ("Objects") and it
	// is not a type in the catalog. Emit must stay identical to the pre-catalog
	// behavior, so it is recorded as a referenced object (gets an object stub),
	// never reclassified as a type. See also dsl.checkUnder.
	case dsl.RefImport, dsl.RefImportUnknown:
		ns := e.importURI(r.Alias)
		id := elementID(ns, r.Name)
		e.extObjs[id] = extRef{ns: ns, name: r.Name}
		return id
	case dsl.RefUnknownName:
		return elementID(e.m.Namespace, under.Name) // nested under a declared instance
	default:
		return e.typeRefElementID(under)
	}
}

// placeholderTypeElementID resolves the composed type an instantiated placeholder
// child stands in for, by locating the placeholder member (by base name) on the
// instance's resolved type.
func (e *emitter) placeholderTypeElementID(inst *dsl.Instance, of dsl.TypeRef) string {
	base := of.Raw
	if b, _, ok := splitPlaceholder(of.Raw); ok {
		base = b
	}
	r := e.m.ResolveType(inst.Type)
	if r.Kind != dsl.RefLocal {
		return ""
	}
	resolved, err := e.m.ResolveMembers(r.Name)
	if err != nil {
		return ""
	}
	var find func([]*dsl.Member) string
	find = func(ms []*dsl.Member) string {
		for _, mem := range ms {
			if mem.IsPlaceholder() && mem.Name == base {
				return e.typeRefElementID(mem.Type)
			}
			if t := find(mem.Children); t != "" {
				return t
			}
		}
		return ""
	}
	for _, rm := range resolved {
		if t := find([]*dsl.Member{rm.Member}); t != "" {
			return t
		}
	}
	return ""
}

// --- reference stubs (self-containment) -------------------------------------

// stubObjectTypes emits a minimal objecttype for every imported object type the
// model references but does not define (e.g. ns0 BaseObjectType, FolderType),
// closing over base types transitively so every elementId in the bundle resolves
// without an external namespace catalog. Stubs carry schema.x-opcua.stub=true —
// they are reference placeholders, not authoritative definitions.
func (e *emitter) stubObjectTypes() []objectTypeDoc {
	want := map[string]extRef{}
	var add func(ref extRef)
	add = func(ref extRef) {
		id := elementID(ref.ns, ref.name)
		if _, seen := want[id]; seen {
			return
		}
		want[id] = ref
		if base, ok := stubBaseOf(ref); ok {
			add(base) // pull in the base chain (FolderType → BaseObjectType)
		}
	}
	for _, ref := range e.extTypes {
		add(ref)
	}
	for _, ref := range e.extObjs {
		add(stubTypeOf(ref)) // an object's type must resolve too (ObjectsFolder → FolderType)
	}

	out := make([]objectTypeDoc, 0, len(want))
	for _, id := range sortedKeys(want) {
		ref := want[id]
		doc := objectTypeDoc{
			ElementID:    id,
			DisplayName:  ref.name,
			NamespaceURI: ref.ns,
			Schema: newObj().
				set("$id", id).
				set("type", "object").
				set("title", ref.name).
				set("x-opcua", newObj().set("stub", true)),
		}
		if base, ok := stubBaseOf(ref); ok {
			doc.SourceTypeID = elementID(base.ns, base.name)
		}
		out = append(out, doc)
	}
	return out
}

// stubObjects emits a minimal object for every imported object the model
// references but does not define (e.g. ns0 ObjectsFolder). A stub object is a
// root anchor: no parent (that would dangle), isComposition=false.
func (e *emitter) stubObjects() []objectDoc {
	out := make([]objectDoc, 0, len(e.extObjs))
	for _, id := range sortedKeys(e.extObjs) {
		ref := e.extObjs[id]
		t := stubTypeOf(ref)
		out = append(out, objectDoc{
			ElementID:     id,
			DisplayName:   ref.name,
			TypeElementID: elementID(t.ns, t.name),
			IsComposition: false,
		})
	}
	return out
}

// stubBaseOf returns the base type of a referenced type, terminating at ns0
// BaseObjectType (the root, which has no base). Any non-root type ultimately
// bases on BaseObjectType.
func stubBaseOf(ref extRef) (extRef, bool) {
	if ref.ns == opcUaNS && ref.name == "BaseObjectType" {
		return extRef{}, false
	}
	return extRef{ns: opcUaNS, name: "BaseObjectType"}, true
}

// stubTypeOf returns the type an imported object instantiates. ObjectsFolder is a
// FolderType; anything else defaults to BaseObjectType.
func stubTypeOf(ref extRef) extRef {
	if ref.ns == opcUaNS && ref.name == "ObjectsFolder" {
		return extRef{ns: opcUaNS, name: "FolderType"}
	}
	return extRef{ns: opcUaNS, name: "BaseObjectType"}
}

// sortedKeys returns the map keys in ascending order, for deterministic output.
func sortedKeys(m map[string]extRef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- shared helpers ---------------------------------------------------------

// typeRefElementID resolves any TypeRef to an elementId: local types live in the
// model namespace; imported names (BaseObjectType, FolderType) live in their
// import's namespace; built-ins are addressed by name in ns0. Imported types are
// recorded so they can be emitted as self-containing stubs.
func (e *emitter) typeRefElementID(ref dsl.TypeRef) string {
	switch r := e.m.ResolveType(ref); r.Kind {
	case dsl.RefLocal:
		return elementID(e.m.Namespace, r.Name)
	case dsl.RefImport:
		ns := e.importURI(r.Alias)
		id := elementID(ns, r.Name)
		e.extTypes[id] = extRef{ns: ns, name: r.Name}
		return id
	default:
		id := ns0(ref.Name)
		e.extTypes[id] = extRef{ns: opcUaNS, name: ref.Name}
		return id
	}
}

// importURI returns the namespace URI for an import alias (OpcUa always resolves
// to the core namespace).
func (e *emitter) importURI(alias string) string {
	for _, im := range e.m.Imports {
		if im.Alias == alias {
			return im.URI
		}
	}
	if alias == "OpcUa" {
		return opcUaNS
	}
	return ""
}

// enum returns the named local enum, or nil.
func (e *emitter) enum(name string) *dsl.Enum {
	for _, en := range e.m.Enums {
		if en.Name == name {
			return en
		}
	}
	return nil
}

// placeholderChild returns the first placeholder member among a member's
// children, or nil.
func placeholderChild(mem *dsl.Member) *dsl.Member {
	for _, ch := range mem.Children {
		if ch.IsPlaceholder() {
			return ch
		}
	}
	return nil
}

// modellingRuleName renders a Rule as its OPC UA ModellingRule browse name.
func modellingRuleName(r dsl.Rule) string {
	switch r {
	case dsl.RuleOptional:
		return "Optional"
	case dsl.RuleOptionalPlaceholder:
		return "OptionalPlaceholder"
	case dsl.RuleMandatoryPlaceholder:
		return "MandatoryPlaceholder"
	default:
		return "Mandatory"
	}
}

// splitPlaceholder recognises a "Name<Suffix>" reference, returning the base name.
var placeholderRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)<([A-Za-z_][A-Za-z0-9_]*)>$`)

func splitPlaceholder(s string) (base, suffix string, ok bool) {
	mm := placeholderRe.FindStringSubmatch(s)
	if mm == nil {
		return "", "", false
	}
	return mm[1], mm[2], true
}
