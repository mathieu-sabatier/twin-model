// Package i3x transpiles a validated dsl.Model to CESMII i3X 1.0 JSON documents.
// It is a build-time complement to a live i3X server: a pure AST → JSON function
// that emits the model an i3X platform would serve (namespaces, object types as
// JSON Schema, relationship types, and instance topology). Live values,
// subscriptions, and history are deliberately out of scope — this repo has no
// running server. Output is deterministic and byte-stable for golden testing.
//
// Identity: every elementId is `nsu=<namespaceUri>;s=<SymbolicId>`, where the
// SymbolicId is reconstructed from the AST (the chain of SymbolicNames) with no
// CSV dependency — it reproduces the ModelDesign CSV keys exactly. See
// docs/i3x-notes.md.
package i3x

// Well-known OPC UA (ns0) elementIds, addressed by BrowseName.
var (
	elemBaseObjectType = ns0("BaseObjectType")
	elemFolderType     = ns0("FolderType")
	elemObjectsFolder  = ns0("ObjectsFolder")
)

// infoDoc is info.json: the model header.
type infoDoc struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	PublicationDate string `json:"publicationDate"`
	I3XVersion      string `json:"i3xVersion"`
}

// namespaceDoc is one entry of namespaces.json.
type namespaceDoc struct {
	URI         string `json:"uri"`
	DisplayName string `json:"displayName"`
}

// relationshipTypeDoc is one entry of relationshiptypes.json.
type relationshipTypeDoc struct {
	ElementID      string `json:"elementId"`
	DisplayName    string `json:"displayName"`
	NamespaceURI   string `json:"namespaceUri"`
	RelationshipID string `json:"relationshipId"`
	ReverseOf      string `json:"reverseOf"`
}

// objectTypeDoc is one entry of objecttypes.json. Schema is a JSON Schema
// document built as an insertion-ordered jobj so its (dynamic) property keys stay
// in declared order.
type objectTypeDoc struct {
	ElementID    string `json:"elementId"`
	DisplayName  string `json:"displayName"`
	NamespaceURI string `json:"namespaceUri"`
	SourceTypeID string `json:"sourceTypeId,omitempty"` // omitted for root types (e.g. BaseObjectType stub)
	Version      string `json:"version,omitempty"`      // omitted for reference stubs
	Schema       *jobj  `json:"schema"`
}

// objectDoc is one entry of objects.json (an instance or a composed sub-object).
type objectDoc struct {
	ElementID     string `json:"elementId"`
	DisplayName   string `json:"displayName"`
	TypeElementID string `json:"typeElementId"`
	ParentID      string `json:"parentId,omitempty"` // omitted for root anchors (e.g. ObjectsFolder stub)
	IsComposition bool   `json:"isComposition"`
}

// builtinJSONType maps a built-in OPC UA DataType name to its JSON Schema type
// token (and optional "format"). ok is false for names that are not built-in
// scalars (local enums, NodeId, structured types), which the caller handles
// separately (enum → $ref; unknown → conservative "string" fallback).
func builtinJSONType(name string) (typ, format string, ok bool) {
	switch name {
	case "String":
		return "string", "", true
	case "Double", "Float":
		return "number", "", true
	case "Boolean":
		return "boolean", "", true
	case "SByte", "Byte", "Int16", "UInt16", "Int32", "UInt32", "Int64", "UInt64":
		return "integer", "", true
	case "DateTime":
		return "string", "date-time", true
	default:
		return "", "", false
	}
}
