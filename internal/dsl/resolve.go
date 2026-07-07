package dsl

// RefKind classifies a resolved TypeRef. This is semantic (which namespace the
// name lives in), not syntactic — turning a Resolved into a ModelDesign QName
// ("ua:Double", "EquipmentType", …) is the emitter's job.
type RefKind int

const (
	RefBuiltin        RefKind = iota // built-in ns0 scalar DataType (String, Double, …)
	RefLocal                         // a type defined in this model
	RefImport                        // alias known, NO catalog loaded — trusted (legacy)
	RefImportResolved                // alias known, catalog loaded, (URI,Name) found
	RefImportUnknown                 // alias known, catalog loaded, (URI,Name) NOT found
	RefUnknownAlias                  // prefixed with an alias that was never imported
	RefUnknownName                   // unprefixed name that is neither built-in nor local
)

// Resolved is the outcome of classifying a TypeRef against a Model.
type Resolved struct {
	Kind  RefKind
	Alias string // for RefImport* / RefUnknownAlias
	Name  string
	URI   string // namespace URI for RefImport* (the import's resolved URI)
}

// builtinTypes are the ns0 built-in DataTypes a member may reference unprefixed.
// Reference / ObjectType / VariableType names (BaseObjectType, FolderType,
// AnalogUnitType, Organizes, …) are NOT here — those are written with the OpcUa:
// prefix. See docs/modeldesign-notes.md §11.
var builtinTypes = map[string]bool{
	"Boolean": true, "SByte": true, "Byte": true,
	"Int16": true, "UInt16": true, "Int32": true, "UInt32": true,
	"Int64": true, "UInt64": true, "Float": true, "Double": true,
	"String": true, "DateTime": true, "Guid": true, "ByteString": true,
	"XmlElement": true, "NodeId": true, "ExpandedNodeId": true,
	"StatusCode": true, "QualifiedName": true, "LocalizedText": true,
	"DataValue": true, "Variant": true, "DiagnosticInfo": true,
	"Number": true, "Integer": true, "UInteger": true, "Enumeration": true,
	"BaseDataType": true, "Duration": true, "UtcTime": true,
}

// ResolveType classifies a type reference against imports + local types + the
// optional companion-spec catalog. It never fails.
func (m *Model) ResolveType(ref TypeRef) Resolved {
	if ref.Alias != "" {
		uri, ok := m.importURI(ref.Alias)
		if !ok {
			return Resolved{Kind: RefUnknownAlias, Alias: ref.Alias, Name: ref.Name}
		}
		// ns0 (OpcUa) is loaded for resolution. Keep FOUND names as trusted
		// RefImport so the emit paths (which switch on RefImport for OpcUa) are
		// byte-identical. Only a genuinely-absent name (catalog attached, so ns0
		// is loaded) becomes RefImportUnknown, surfacing as a validation error.
		if uri == OpcUaNamespaceURI {
			if m.Catalog != nil {
				if _, nsOk := m.Catalog.Namespace(uri); nsOk {
					if _, found := m.Catalog.LookupType(uri, ref.Name); !found {
						return Resolved{Kind: RefImportUnknown, Alias: ref.Alias, Name: ref.Name, URI: uri}
					}
				}
			}
			return Resolved{Kind: RefImport, Alias: ref.Alias, Name: ref.Name, URI: uri}
		}
		if m.Catalog == nil {
			return Resolved{Kind: RefImport, Alias: ref.Alias, Name: ref.Name, URI: uri}
		}
		if _, found := m.Catalog.LookupType(uri, ref.Name); found {
			return Resolved{Kind: RefImportResolved, Alias: ref.Alias, Name: ref.Name, URI: uri}
		}
		return Resolved{Kind: RefImportUnknown, Alias: ref.Alias, Name: ref.Name, URI: uri}
	}
	if builtinTypes[ref.Name] {
		return Resolved{Kind: RefBuiltin, Name: ref.Name}
	}
	if m.hasLocalType(ref.Name) {
		return Resolved{Kind: RefLocal, Name: ref.Name}
	}
	return Resolved{Kind: RefUnknownName, Name: ref.Name}
}

// OpcUaNamespaceURI is ns0, always available even if not explicitly imported.
const OpcUaNamespaceURI = "http://opcfoundation.org/UA/"

// importURI returns the namespace URI an alias resolves to (OpcUa is always ns0).
func (m *Model) importURI(alias string) (string, bool) {
	if alias == "OpcUa" {
		return OpcUaNamespaceURI, true
	}
	for _, im := range m.Imports {
		if im.Alias == alias {
			return im.URI, true
		}
	}
	return "", false
}

// CatalogType returns the catalog type a resolved import reference points at.
func (m *Model) CatalogType(r Resolved) (CatalogType, bool) {
	if m.Catalog == nil || (r.Kind != RefImportResolved) {
		return CatalogType{}, false
	}
	return m.Catalog.LookupType(r.URI, r.Name)
}

// localObjectType returns a locally-defined ObjectType by name.
func (m *Model) localObjectType(name string) (*ObjectType, bool) {
	for _, ot := range m.ObjectTypes {
		if ot.Name == name {
			return ot, true
		}
	}
	return nil, false
}

func (m *Model) hasLocalType(name string) bool {
	for _, e := range m.Enums {
		if e.Name == name {
			return true
		}
	}
	for _, ot := range m.ObjectTypes {
		if ot.Name == name {
			return true
		}
	}
	return false
}
