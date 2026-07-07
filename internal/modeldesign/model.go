package modeldesign

import "encoding/xml"

// These structs mirror the ModelDesign XSD closely enough to marshal via
// encoding/xml. Element names carry their `opc:`/`uax:` prefix literally; the
// prefixes are declared on the hand-written root element in emit.go. Field
// declaration order is significant: the XSD uses xs:sequence, so children must
// appear in the order defined here (see docs/modeldesign-notes.md §3).

// xmlNamespaces is the <opc:Namespaces> table.
type xmlNamespaces struct {
	XMLName xml.Name       `xml:"opc:Namespaces"`
	List    []xmlNamespace `xml:"opc:Namespace"`
}

type xmlNamespace struct {
	Name            string `xml:"Name,attr"`
	Prefix          string `xml:"Prefix,attr"`
	InternalPrefix  string `xml:"InternalPrefix,attr,omitempty"`
	XmlNamespace    string `xml:"XmlNamespace,attr"`
	XmlPrefix       string `xml:"XmlPrefix,attr"`
	Version         string `xml:"Version,attr,omitempty"`
	PublicationDate string `xml:"PublicationDate,attr,omitempty"`
	URI             string `xml:",chardata"`
}

// xmlDataType is an enumeration DataType.
type xmlDataType struct {
	XMLName         xml.Name   `xml:"opc:DataType"`
	SymbolicName    string     `xml:"SymbolicName,attr"`
	BaseType        string     `xml:"BaseType,attr"`
	ForceEnumValues bool       `xml:"ForceEnumValues,attr,omitempty"`
	Description     string     `xml:"opc:Description,omitempty"`
	Fields          []xmlField `xml:"opc:Fields>opc:Field"`
}

type xmlField struct {
	Name       string `xml:"Name,attr"`
	Identifier int    `xml:"Identifier,attr"`
}

// xmlObjectType is an ObjectType definition.
type xmlObjectType struct {
	XMLName      xml.Name     `xml:"opc:ObjectType"`
	SymbolicName string       `xml:"SymbolicName,attr"`
	BaseType     string       `xml:"BaseType,attr"`
	IsAbstract   bool         `xml:"IsAbstract,attr,omitempty"`
	Description  string       `xml:"opc:Description,omitempty"`
	Children     *xmlChildren `xml:"opc:Children,omitempty"`
}

// xmlChildren holds an ordered, heterogeneous list of child nodes. Each child's
// element name comes from its own XMLName, so source order is preserved.
type xmlChildren struct {
	Items []xmlChild
}

// xmlChild is the union of Property/Variable/Object/Method children. Fields not
// relevant to a given kind stay zero and are omitted. Attribute + element order
// below satisfies the XSD sequence for every kind simultaneously.
type xmlChild struct {
	XMLName        xml.Name
	SymbolicName   string           `xml:"SymbolicName,attr"`
	DataType       string           `xml:"DataType,attr,omitempty"`
	TypeDefinition string           `xml:"TypeDefinition,attr,omitempty"`
	ValueRank      string           `xml:"ValueRank,attr,omitempty"`
	AccessLevel    string           `xml:"AccessLevel,attr,omitempty"`
	ModellingRule  string           `xml:"ModellingRule,attr,omitempty"`
	BrowseName     string           `xml:"opc:BrowseName,omitempty"`
	Description    string           `xml:"opc:Description,omitempty"`
	Children       *xmlChildren     `xml:"opc:Children,omitempty"`
	DefaultValue   *xmlDefaultValue `xml:"opc:DefaultValue,omitempty"`
	InputArguments *xmlArguments    `xml:"opc:InputArguments,omitempty"`
	OutputArgs     *xmlArguments    `xml:"opc:OutputArguments,omitempty"`
}

type xmlArguments struct {
	Arguments []xmlArgument `xml:"opc:Argument"`
}

type xmlArgument struct {
	Name     string `xml:"Name,attr"`
	DataType string `xml:"DataType,attr,omitempty"`
}

// xmlDefaultValue is either an EUInformation ExtensionObject (unit variables) or
// a bare uax scalar Variant (instance value overrides). Exactly one arm is set;
// nil pointers marshal to nothing. The EUInformation field order copies
// StandardTypes.xml:8715 exactly.
type xmlDefaultValue struct {
	ExtensionObject *xmlExtensionObject `xml:"uax:ExtensionObject"`
	Scalar          *xmlScalar
}

// xmlScalar is a single uax Variant scalar element; the element name (uax:String,
// uax:Double, uax:Int32, ...) comes from XMLName.
type xmlScalar struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type xmlExtensionObject struct {
	TypeId xmlTypeID `xml:"uax:TypeId"`
	Body   xmlEOBody `xml:"uax:Body"`
}

type xmlTypeID struct {
	Identifier string `xml:"uax:Identifier"`
}

type xmlEOBody struct {
	EUInformation xmlEUInformation `xml:"uax:EUInformation"`
}

type xmlEUInformation struct {
	NamespaceURI string           `xml:"uax:NamespaceUri"`
	UnitID       int32            `xml:"uax:UnitId"`
	DisplayName  xmlLocalizedText `xml:"uax:DisplayName"`
	Description  xmlLocalizedText `xml:"uax:Description"`
}

type xmlLocalizedText struct {
	Locale string `xml:"uax:Locale"`
	Text   string `xml:"uax:Text"`
}

// xmlInstance is a top-level Object instance with its inverse-Organizes reference.
type xmlInstance struct {
	XMLName        xml.Name      `xml:"opc:Object"`
	SymbolicName   string        `xml:"SymbolicName,attr"`
	TypeDefinition string        `xml:"TypeDefinition,attr"`
	Children       *xmlChildren  `xml:"opc:Children,omitempty"`
	References     xmlReferences `xml:"opc:References"`
}

type xmlReferences struct {
	References []xmlReference `xml:"opc:Reference"`
}

type xmlReference struct {
	IsInverse     bool   `xml:"IsInverse,attr,omitempty"`
	ReferenceType string `xml:"opc:ReferenceType"`
	TargetID      string `xml:"opc:TargetId"`
}
