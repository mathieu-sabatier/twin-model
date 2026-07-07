package dsl

// Catalog is the seam through which dsl resolves references into imported
// companion-spec namespaces WITHOUT knowing how NodeSet2 files are parsed. A nil
// *Model.Catalog means "no companion specs loaded" — imported references then
// keep their legacy trusted (unchecked) behavior. internal/nodeset implements it.
type Catalog interface {
	// Namespace reports version/publication-date metadata for a loaded companion
	// namespace, if that namespace was bundled and parsed.
	Namespace(uri string) (CatalogNamespace, bool)
	// LookupType finds a type by absolute namespace URI + browse name.
	LookupType(uri, name string) (CatalogType, bool)
}

// CatalogNamespace is a loaded companion namespace's header metadata.
type CatalogNamespace struct {
	URI             string
	Version         string
	PublicationDate string // "YYYY-MM-DD" (normalized from the NodeSet2 dateTime)
}

// CatalogType is a companion-spec type, flattened for consumption by dsl.
type CatalogType struct {
	NamespaceURI string
	Name         string // BrowseName, un-prefixed
	NodeClass    string // "ObjectType" | "VariableType" | "DataType" | "ReferenceType"
	Abstract     bool
	BaseURI      string // "" at the root of the hierarchy
	BaseName     string
	Members      []CatalogMember // flattened own + inherited (most-derived-wins)
}

// EnumMember is one allowed value of an enumeration DataType.
type EnumMember struct {
	Name  string
	Value int64
}

// CatalogMember is one declared member of a companion type.
type CatalogMember struct {
	Name        string
	Kind        Kind
	Placeholder bool
	// TypeURI/TypeName identify the member's type definition (from its
	// HasTypeDefinition). Empty when the member has no resolvable type. Lets a
	// consumer link a companion-typed member to that type's detail.
	TypeURI  string
	TypeName string
	// Enum is the allowed values when the member's DataType is an enumeration;
	// nil otherwise.
	Enum []EnumMember
}
