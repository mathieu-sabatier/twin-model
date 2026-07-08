// Package nodeset parses OPC UA NodeSet2.xml companion specifications into a
// cross-spec type Catalog that twinmodel resolves imported references against.
// It owns all NodeSet2/XML concerns; the semantic view it produces is XML-free.
package nodeset

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
)

// NodeSet is one parsed NodeSet2.xml document.
type NodeSet struct {
	NamespaceURIs []string // file-local ns table; index 0 here == file ns index 1
	Models        []Model
	Aliases       []Alias
	Nodes         []Node
}

// Model is a <Models><Model> header entry.
type Model struct {
	ModelURI        string
	Version         string
	PublicationDate string // as written, e.g. "2022-11-01T00:00:00Z"
	RequiredModels  []RequiredModel
}

// RequiredModel is a transitive dependency declared by a Model.
type RequiredModel struct {
	ModelURI        string
	Version         string
	PublicationDate string
}

// Alias maps a reference alias (e.g. "HasSubtype") to a NodeId ("i=45").
type Alias struct {
	Name  string
	Value string
}

// Node is any UA* node, tagged with its NodeClass in Class.
type Node struct {
	Class        string // ObjectType|VariableType|DataType|ReferenceType|Object|Variable|Method
	NodeID       string // as written, e.g. "ns=1;i=1002" or "i=58"
	BrowseName   string // as written, e.g. "1:DeviceType"
	SymbolicName string // as written, no namespace-index prefix (e.g. "ObjectsFolder")
	IsAbstract   bool
	DataType     string // for variables/properties; as written
	Refs         []Reference
	// Enum encodings, populated on UADataType (EnumMembers) and on the
	// EnumStrings/EnumValues UAVariable nodes (EnumStrings/EnumValues). The
	// catalog pass stitches the variable payloads onto their DataType.
	EnumMembers []EnumMember // from <Definition> Value-bearing, DataType-less fields
	EnumStrings []string     // from EnumStrings variable (implicit indices 0..n-1)
	EnumValues  []EnumMember // from EnumValues variable (explicit EnumValueType)
}

// EnumMember is one normalized enumeration value.
type EnumMember struct {
	Name  string
	Value int64
}

// Reference is one <Reference> edge.
type Reference struct {
	Type    string // alias or nodeid, as written
	Forward bool   // IsForward; defaults to true when the attr is absent
	Target  string // target NodeId, as written
}

// raw* mirror the NodeSet2 XSD for encoding/xml decoding.
type rawNodeSet struct {
	XMLName       xml.Name   `xml:"UANodeSet"`
	NamespaceURIs []string   `xml:"NamespaceUris>Uri"`
	Models        []rawModel `xml:"Models>Model"`
	Aliases       []rawAlias `xml:"Aliases>Alias"`
	Object        []rawNode  `xml:"UAObjectType"`
	Variable      []rawNode  `xml:"UAVariableType"`
	Data          []rawNode  `xml:"UADataType"`
	Reference     []rawNode  `xml:"UAReferenceType"`
	Objects       []rawNode  `xml:"UAObject"`
	Variables     []rawNode  `xml:"UAVariable"`
	Methods       []rawNode  `xml:"UAMethod"`
}

type rawModel struct {
	ModelURI        string        `xml:"ModelUri,attr"`
	Version         string        `xml:"Version,attr"`
	PublicationDate string        `xml:"PublicationDate,attr"`
	RequiredModels  []rawRequired `xml:"RequiredModel"`
}

type rawRequired struct {
	ModelURI        string `xml:"ModelUri,attr"`
	Version         string `xml:"Version,attr"`
	PublicationDate string `xml:"PublicationDate,attr"`
}

type rawAlias struct {
	Alias string `xml:"Alias,attr"`
	Value string `xml:",chardata"`
}

type rawNode struct {
	NodeID       string         `xml:"NodeId,attr"`
	BrowseName   string         `xml:"BrowseName,attr"`
	SymbolicName string         `xml:"SymbolicName,attr"`
	IsAbstract   bool           `xml:"IsAbstract,attr"`
	DataType     string         `xml:"DataType,attr"`
	Definition   *rawDefinition `xml:"Definition"`
	Value        *rawValue      `xml:"Value"`
	Refs         []rawRef       `xml:"References>Reference"`
}

type rawRef struct {
	Type      string `xml:"ReferenceType,attr"`
	IsForward *bool  `xml:"IsForward,attr"` // nil (absent) means forward
	Target    string `xml:",chardata"`
}

// rawDefinition is the <Definition> child of a UADataType. Enum fields carry a
// Value and no DataType; struct fields carry a DataType and no Value.
type rawDefinition struct {
	Name   string        `xml:"Name,attr"`
	Fields []rawDefField `xml:"Field"`
}

type rawDefField struct {
	Name     string `xml:"Name,attr"`
	Value    string `xml:"Value,attr"`    // present on enum fields
	DataType string `xml:"DataType,attr"` // present on struct fields
}

// rawValue is a UAVariable's <Value>. Both list payloads resolve to the 2008/02
// Types.xsd namespace (EnumStrings via default xmlns, EnumValues via the uax:
// prefix). encoding/xml matches on LOCAL name and ignores the prefix, so bare
// local-name tags are correct and consistent with the rest of parse.go.
type rawValue struct {
	LocalizedText []rawLocalizedText `xml:"ListOfLocalizedText>LocalizedText"`
	ExtObjects    []rawExtObject     `xml:"ListOfExtensionObject>ExtensionObject"`
}

type rawLocalizedText struct {
	Text string `xml:"Text"`
}

type rawExtObject struct {
	EnumValue rawEnumValueType `xml:"Body>EnumValueType"`
}

type rawEnumValueType struct {
	Value       int64  `xml:"Value"`
	DisplayName string `xml:"DisplayName>Text"`
}

// Parse decodes one NodeSet2.xml document.
func Parse(r io.Reader) (*NodeSet, error) {
	var raw rawNodeSet
	if err := xml.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("nodeset: decode: %w", err)
	}
	ns := &NodeSet{NamespaceURIs: raw.NamespaceURIs}
	for _, m := range raw.Models {
		mm := Model{ModelURI: m.ModelURI, Version: m.Version, PublicationDate: m.PublicationDate}
		for _, rm := range m.RequiredModels {
			mm.RequiredModels = append(mm.RequiredModels, RequiredModel(rm))
		}
		ns.Models = append(ns.Models, mm)
	}
	for _, a := range raw.Aliases {
		ns.Aliases = append(ns.Aliases, Alias{Name: a.Alias, Value: a.Value})
	}
	add := func(class string, nodes []rawNode) {
		for _, n := range nodes {
			node := Node{Class: class, NodeID: n.NodeID, BrowseName: n.BrowseName, SymbolicName: n.SymbolicName, IsAbstract: n.IsAbstract, DataType: n.DataType}
			for _, r := range n.Refs {
				fwd := true
				if r.IsForward != nil {
					fwd = *r.IsForward
				}
				node.Refs = append(node.Refs, Reference{Type: r.Type, Forward: fwd, Target: r.Target})
			}
			// <Definition> enum fields: Value present, DataType absent. Struct
			// fields (DataType present) are excluded.
			if n.Definition != nil {
				for _, f := range n.Definition.Fields {
					if f.DataType != "" || f.Value == "" {
						continue
					}
					v, err := strconv.ParseInt(f.Value, 10, 64)
					if err != nil {
						continue
					}
					node.EnumMembers = append(node.EnumMembers, EnumMember{Name: f.Name, Value: v})
				}
			}
			// EnumStrings/EnumValues child-variable <Value> payloads.
			if n.Value != nil {
				for _, lt := range n.Value.LocalizedText {
					node.EnumStrings = append(node.EnumStrings, lt.Text)
				}
				for _, eo := range n.Value.ExtObjects {
					node.EnumValues = append(node.EnumValues, EnumMember{Name: eo.EnumValue.DisplayName, Value: eo.EnumValue.Value})
				}
			}
			ns.Nodes = append(ns.Nodes, node)
		}
	}
	add("ObjectType", raw.Object)
	add("VariableType", raw.Variable)
	add("DataType", raw.Data)
	add("ReferenceType", raw.Reference)
	add("Object", raw.Objects)
	add("Variable", raw.Variables)
	add("Method", raw.Methods)
	return ns, nil
}
