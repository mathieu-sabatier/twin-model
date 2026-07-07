// Package modeldesign renders a validated dsl.Model to UA-ModelCompiler
// ModelDesign XML. It owns all XML/QName concerns; the dsl package stays
// XML-free. Output is deterministic (source order preserved, no timestamps
// beyond the model publication date) and byte-stable so it can be golden-tested
// and diffed in review.
package modeldesign

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

const (
	nsModelDesign = "http://opcfoundation.org/UA/ModelDesign.xsd"
	nsUA          = "http://opcfoundation.org/UA/"
	nsUAX         = "http://opcfoundation.org/UA/2008/02/Types.xsd"
	nsXSI         = "http://www.w3.org/2001/XMLSchema-instance"

	euEncodingXMLNodeID = "i=888" // EUInformation_Encoding_DefaultXml
	uneceNamespaceURI   = "http://www.opcfoundation.org/UA/units/un/cefact"
)

// Emit transpiles a Model to ModelDesign XML bytes.
func Emit(m *dsl.Model) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	buf.WriteString(rootOpenTag(m))

	// Body nodes, in a fixed deterministic order: namespaces, enums, object
	// types, then instances.
	nodes := []any{buildNamespaces(m)}
	for _, e := range m.Enums {
		nodes = append(nodes, buildDataType(e))
	}
	for _, ot := range m.ObjectTypes {
		nodes = append(nodes, buildObjectType(m, ot))
	}
	for _, inst := range m.Instances {
		nodes = append(nodes, buildInstance(m, inst))
	}
	nodes = append(nodes, buildPerspectiveNodes(m)...)

	for _, n := range nodes {
		b, err := xml.MarshalIndent(n, "  ", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	buf.WriteString("</opc:ModelDesign>\n")

	return []byte(selfCloseEmpties(buf.String())), nil
}

func isOpcUaImport(im dsl.Import) bool {
	return im.Alias == "OpcUa" || im.URI == dsl.OpcUaNamespaceURI
}

// additionalImports are the non-ns0 imports; ns0 is always the fixed `ua` prefix.
func additionalImports(m *dsl.Model) []dsl.Import {
	var out []dsl.Import
	for _, im := range m.Imports {
		if !isOpcUaImport(im) {
			out = append(out, im)
		}
	}
	return out
}

// typesXMLNamespace is the XML namespace of the model's generated DataType schema.
func typesXMLNamespace(m *dsl.Model) string { return m.Namespace + "Types.xsd" }

func rootOpenTag(m *dsl.Model) string {
	var b strings.Builder
	b.WriteString("<opc:ModelDesign\n")
	fmt.Fprintf(&b, "    xmlns:opc=%q\n", nsModelDesign)
	fmt.Fprintf(&b, "    xmlns:xsi=%q\n", nsXSI)
	fmt.Fprintf(&b, "    xmlns:ua=%q\n", nsUA)
	fmt.Fprintf(&b, "    xmlns:uax=%q\n", nsUAX)
	for _, im := range additionalImports(m) {
		fmt.Fprintf(&b, "    xmlns:%s=%q\n", im.Alias, im.URI)
	}
	fmt.Fprintf(&b, "    xmlns=%q\n", m.Namespace)
	fmt.Fprintf(&b, "    TargetNamespace=%q\n", m.Namespace)
	fmt.Fprintf(&b, "    TargetXmlNamespace=%q\n", typesXMLNamespace(m))
	fmt.Fprintf(&b, "    TargetVersion=%q\n", m.Version)
	fmt.Fprintf(&b, "    TargetPublicationDate=%q>\n", m.PublicationDate+"T00:00:00Z")
	return b.String()
}

func buildNamespaces(m *dsl.Model) xmlNamespaces {
	own := xmlNamespace{
		Name:         m.Name,
		Prefix:       m.Prefix,
		XmlNamespace: typesXMLNamespace(m),
		XmlPrefix:    m.Name,
		URI:          m.Namespace,
	}
	opcua := xmlNamespace{
		Name:           "OpcUa",
		Prefix:         "Opc.Ua",
		InternalPrefix: "Opc.Ua.Server",
		XmlNamespace:   nsUAX,
		XmlPrefix:      "OpcUa",
		URI:            nsUA,
	}
	// Own namespace first, then imports (OpcUa always last).
	list := []xmlNamespace{own}
	for _, im := range additionalImports(m) {
		e := xmlNamespace{Name: im.Alias, Prefix: im.Alias, XmlNamespace: im.URI, XmlPrefix: im.Alias, URI: im.URI}
		if m.Catalog != nil {
			if ns, ok := m.Catalog.Namespace(im.URI); ok {
				e.Version = ns.Version
				if ns.PublicationDate != "" {
					e.PublicationDate = ns.PublicationDate + "T00:00:00Z"
				}
			}
		}
		list = append(list, e)
	}
	list = append(list, opcua)
	return xmlNamespaces{List: list}
}

func buildDataType(e *dsl.Enum) xmlDataType {
	dt := xmlDataType{
		SymbolicName:    e.Name,
		BaseType:        "ua:Enumeration",
		ForceEnumValues: needsForceEnumValues(e),
		Description:     e.Doc,
	}
	for _, v := range e.Values {
		dt.Fields = append(dt.Fields, xmlField{Name: v.Name, Identifier: v.Identifier})
	}
	return dt
}

// needsForceEnumValues is true when ids are not the plain contiguous 0..n-1
// sequence, so the compiler emits EnumValues instead of EnumStrings.
func needsForceEnumValues(e *dsl.Enum) bool {
	for i, v := range e.Values {
		if v.Explicit || v.Identifier != i {
			return true
		}
	}
	return false
}

func buildObjectType(m *dsl.Model, ot *dsl.ObjectType) xmlObjectType {
	return xmlObjectType{
		SymbolicName: ot.Name,
		BaseType:     qname(m, ot.Base),
		IsAbstract:   ot.Abstract,
		Description:  ot.Doc,
		Children:     buildChildren(m, ot.Members),
	}
}

func buildChildren(m *dsl.Model, members []*dsl.Member) *xmlChildren {
	if len(members) == 0 {
		return nil
	}
	c := &xmlChildren{}
	for _, mem := range members {
		c.Items = append(c.Items, buildChild(m, mem))
	}
	return c
}

func buildChild(m *dsl.Model, mem *dsl.Member) xmlChild {
	ch := xmlChild{
		SymbolicName:  mem.Name,
		ModellingRule: modellingRule(mem.Rule),
		BrowseName:    mem.BrowseName,
		Description:   mem.Doc,
	}
	switch mem.Kind {
	case dsl.KindProperty:
		ch.XMLName = xml.Name{Local: "opc:Property"}
		ch.DataType = qname(m, mem.Type)
	case dsl.KindObject:
		ch.XMLName = xml.Name{Local: "opc:Object"}
		ch.TypeDefinition = qname(m, mem.Type)
		ch.Children = buildChildren(m, mem.Children)
	case dsl.KindMethod:
		ch.XMLName = xml.Name{Local: "opc:Method"}
		ch.InputArguments = buildArguments(m, mem.In)
		ch.OutputArgs = buildArguments(m, mem.Out)
	default: // variable
		ch.XMLName = xml.Name{Local: "opc:Variable"}
		ch.DataType = qname(m, mem.Type)
		ch.ValueRank = "Scalar"
		ch.AccessLevel = accessLevel(mem.Access)
		if mem.Unit != "" {
			ch.TypeDefinition = "ua:AnalogUnitType"
			u, _ := dsl.LookupUnit(mem.Unit) // validated earlier
			ch.Children = &xmlChildren{Items: []xmlChild{engineeringUnits(u)}}
		}
	}
	return ch
}

func buildArguments(m *dsl.Model, args []dsl.Argument) *xmlArguments {
	if len(args) == 0 {
		return nil
	}
	out := &xmlArguments{}
	for _, a := range args {
		out.Arguments = append(out.Arguments, xmlArgument{Name: a.Name, DataType: qname(m, a.Type)})
	}
	return out
}

func engineeringUnits(u dsl.Unit) xmlChild {
	return xmlChild{
		XMLName:       xml.Name{Local: "opc:Property"},
		SymbolicName:  "ua:EngineeringUnits",
		DataType:      "ua:EUInformation",
		ModellingRule: "Mandatory",
		DefaultValue: &xmlDefaultValue{
			ExtensionObject: &xmlExtensionObject{
				TypeId: xmlTypeID{Identifier: euEncodingXMLNodeID},
				Body: xmlEOBody{EUInformation: xmlEUInformation{
					NamespaceURI: uneceNamespaceURI,
					UnitID:       u.UnitID,
					DisplayName:  xmlLocalizedText{Locale: "en", Text: u.DisplayName},
					Description:  xmlLocalizedText{Locale: "en", Text: u.Description},
				}},
			},
		},
	}
}

func buildInstance(m *dsl.Model, inst *dsl.Instance) xmlInstance {
	xi := xmlInstance{
		SymbolicName:   inst.Name,
		TypeDefinition: qname(m, inst.Type),
		References: xmlReferences{References: []xmlReference{{
			IsInverse:     true,
			ReferenceType: "ua:Organizes",
			TargetID:      instanceTarget(m, inst.Under),
		}}},
	}
	if items := buildInstanceChildren(m, inst); len(items) > 0 {
		xi.Children = &xmlChildren{Items: items}
	}
	return xi
}

// instanceTarget resolves an instance's `under` to a QName TargetId: an import
// target via qname, or a bare local instance name (nesting) emitted unprefixed.
func instanceTarget(m *dsl.Model, under dsl.TypeRef) string {
	if r := m.ResolveType(under); r.Kind == dsl.RefUnknownName {
		return under.Name // nesting under a declared instance (validated earlier)
	}
	return qname(m, under)
}

// buildInstanceChildren renders value overrides and placeholder instantiations,
// in that order, as the instance Object's <opc:Children> (before References).
func buildInstanceChildren(m *dsl.Model, inst *dsl.Instance) []xmlChild {
	var items []xmlChild
	if lvl := equipmentLevelChild(m, inst); lvl != nil {
		items = append(items, *lvl)
	}

	// Resolve the instance's type members once, to type the overrides.
	var byName map[string]*dsl.Member
	if r := m.ResolveType(inst.Type); r.Kind == dsl.RefLocal {
		if resolved, err := m.ResolveMembers(r.Name); err == nil {
			byName = map[string]*dsl.Member{}
			for _, rm := range resolved {
				byName[rm.Name] = rm.Member
			}
		}
	}

	for _, val := range inst.Values {
		mem := byName[val.Member]
		if mem == nil {
			continue // validated earlier; skip defensively
		}
		items = append(items, buildValueOverride(m, mem, val.Raw))
	}
	for _, ch := range inst.Children {
		td := placeholderType(m, inst, ch.Of)
		if td == "" {
			continue // unresolved placeholder (validated earlier); never emit a TypeDefinition-less Object
		}
		items = append(items, xmlChild{
			XMLName:        xml.Name{Local: "opc:Object"},
			SymbolicName:   ch.Name,
			TypeDefinition: td,
		})
	}
	return items
}

// equipmentLevelChild emits the ISA-95 EquipmentLevel value-override for a node
// that declares a `level` and whose type actually carries that member. The enum
// value comes from the canonical ISA-95 table (dsl); the DataType QName uses the
// model's own import alias for the ISA-95 namespace.
func equipmentLevelChild(m *dsl.Model, inst *dsl.Instance) *xmlChild {
	if inst.Level == "" || !m.TypeHasMember(inst.Type, dsl.ISA95EquipmentLevelMember) {
		return nil
	}
	val, ok := dsl.ISA95LevelValue(inst.Level)
	if !ok {
		return nil // validated earlier; skip defensively
	}
	return &xmlChild{
		XMLName:      xml.Name{Local: "opc:Variable"},
		SymbolicName: dsl.ISA95EquipmentLevelMember,
		DataType:     importedEnumQName(m, dsl.ISA95NamespaceURI, dsl.ISA95EquipmentLevelEnum),
		ValueRank:    "Scalar",
		DefaultValue: &xmlDefaultValue{Scalar: &xmlScalar{
			XMLName: xml.Name{Local: "uax:Int32"},
			Value:   strconv.Itoa(val),
		}},
	}
}

// importedEnumQName builds `<alias>:<enumName>` using the model's import alias
// for the given namespace URI (falling back to the bare name if not imported).
func importedEnumQName(m *dsl.Model, uri, enumName string) string {
	for _, im := range m.Imports {
		if im.URI == uri {
			return im.Alias + ":" + enumName
		}
	}
	return enumName
}

// buildValueOverride emits a Property/Variable child that re-states an inherited
// member with a scalar DefaultValue. SymbolicName is the member name (unprefixed
// for local-chain members, Style A); no ModellingRule/AccessLevel. A variable
// keeps ValueRank="Scalar" to match the type-level declaration; a property has
// none.
func buildValueOverride(m *dsl.Model, mem *dsl.Member, raw string) xmlChild {
	elem := "opc:Variable"
	if mem.Kind == dsl.KindProperty {
		elem = "opc:Property"
	}
	ch := xmlChild{
		XMLName:      xml.Name{Local: elem},
		SymbolicName: mem.Name,
		DataType:     qname(m, mem.Type),
		DefaultValue: &xmlDefaultValue{Scalar: &xmlScalar{
			XMLName: xml.Name{Local: scalarElement(m, mem.Type)},
			Value:   scalarText(m, mem.Type, raw),
		}},
	}
	if mem.Kind == dsl.KindVariable {
		ch.ValueRank = "Scalar"
	}
	return ch
}

// placeholderType resolves the TypeDefinition for an instantiated placeholder
// child: find the placeholder member (by base name) on the resolved type and use
// its declared type.
func placeholderType(m *dsl.Model, inst *dsl.Instance, of dsl.TypeRef) string {
	base := of.Raw
	if b, _, ok := splitPlaceholderRaw(of.Raw); ok {
		base = b
	}
	r := m.ResolveType(inst.Type)
	if r.Kind != dsl.RefLocal {
		return ""
	}
	resolved, err := m.ResolveMembers(r.Name)
	if err != nil {
		return ""
	}
	var find func(members []*dsl.Member) string
	find = func(members []*dsl.Member) string {
		for _, mem := range members {
			if mem.IsPlaceholder() && mem.Name == base {
				return qname(m, mem.Type)
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

// scalarElement maps a member DataType to its uax Variant element name.
func scalarElement(m *dsl.Model, t dsl.TypeRef) string {
	r := m.ResolveType(t)
	if r.Kind == dsl.RefBuiltin {
		return "uax:" + r.Name
	}
	if r.Kind == dsl.RefLocal {
		// Local enums encode as their underlying Int32.
		for _, e := range m.Enums {
			if e.Name == r.Name {
				return "uax:Int32"
			}
		}
	}
	return "uax:String" // fallback; other cases are out of phase-2 scope
}

// scalarText converts the raw YAML value to its Variant text. For a local enum,
// a value name resolves to the enum member's identifier; a numeric raw passes
// through.
func scalarText(m *dsl.Model, t dsl.TypeRef, raw string) string {
	if r := m.ResolveType(t); r.Kind == dsl.RefLocal {
		for _, e := range m.Enums {
			if e.Name == r.Name {
				for _, val := range e.Values {
					if val.Name == raw {
						return fmt.Sprintf("%d", val.Identifier)
					}
				}
			}
		}
	}
	return raw
}

// splitPlaceholderRaw recognises a "Name<Suffix>" reference in emitter code.
var placeholderRawRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)<([A-Za-z_][A-Za-z0-9_]*)>$`)

func splitPlaceholderRaw(s string) (base, suffix string, ok bool) {
	mm := placeholderRawRe.FindStringSubmatch(s)
	if mm == nil {
		return "", "", false
	}
	return mm[1], mm[2], true
}

// --- QName + enum mapping ----------------------------------------------------

func qname(m *dsl.Model, ref dsl.TypeRef) string {
	r := m.ResolveType(ref)
	switch r.Kind {
	case dsl.RefBuiltin:
		return "ua:" + r.Name
	case dsl.RefLocal:
		return r.Name // Style A: own namespace is the default xmlns
	case dsl.RefImport, dsl.RefImportUnknown:
		// RefImportUnknown (e.g. OpcUa:ObjectsFolder — a valid ns0 object that the
		// catalog indexes by BrowseName, not as a type) must emit with its import
		// prefix exactly like RefImport, keeping emit byte-identical to the
		// pre-catalog behavior rather than falling through to the raw form.
		return xmlPrefixFor(m, r.Alias) + ":" + r.Name
	default:
		return ref.Raw // unreachable after Validate; emit raw rather than panic
	}
}

func xmlPrefixFor(m *dsl.Model, alias string) string {
	if alias == "OpcUa" {
		return "ua"
	}
	for _, im := range m.Imports {
		if im.Alias == alias {
			if im.URI == dsl.OpcUaNamespaceURI {
				return "ua"
			}
			return alias
		}
	}
	return alias
}

func modellingRule(r dsl.Rule) string {
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

func accessLevel(a dsl.Access) string {
	if a == dsl.AccessReadWrite {
		return "ReadWrite"
	}
	return "Read"
}

// --- formatting --------------------------------------------------------------

// emptyElem matches a whole line that is a single empty element Go rendered as
// `<tag attrs></tag>`, so we can rewrite it to `<tag attrs/>`.
var emptyElem = regexp.MustCompile(`^(\s*)<([\w:.-]+)((?: [^<>]*)?)></([\w:.-]+)>$`)

func selfCloseEmpties(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if mm := emptyElem.FindStringSubmatch(ln); mm != nil && mm[2] == mm[4] {
			lines[i] = mm[1] + "<" + mm[2] + mm[3] + "/>"
		}
	}
	return strings.Join(lines, "\n")
}
