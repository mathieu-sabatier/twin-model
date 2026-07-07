package i3x

import "strings"

// NamespaceURI is an OPC UA namespace URI (the `nsu=` part of an elementId).
type NamespaceURI = string

const (
	// opcUaNS is the OPC UA core namespace (ns0): the home of the well-known
	// base types (BaseObjectType, FolderType, ObjectsFolder) and reference types
	// (HasComponent, Organizes) we reference but do not define.
	opcUaNS NamespaceURI = "http://opcfoundation.org/UA/"

	// uneceNS is the UNECE unit-code namespace used by EUInformation.
	uneceNS NamespaceURI = "http://www.opcfoundation.org/UA/units/un/cefact"
)

// symbolicID reconstructs an OPC UA SymbolicId from the chain of SymbolicNames
// (root type/instance name followed by each nested member Name), joined by "_".
// This is a pure function of the model — it reproduces the left column of the
// ModelDesign CSV with no CSV dependency (see docs/i3x-notes.md).
//
// For a placeholder member the contributing part is its base Name (e.g. "Zone"),
// never the browse pattern ("<ZoneNo>").
func symbolicID(parts ...string) string {
	return strings.Join(parts, "_")
}

// elementID formats an expanded OPC UA string-NodeId: `nsu=<ns>;s=<symbolicId>`.
// Self-describing, a legal NodeId form, and round-trippable.
func elementID(ns NamespaceURI, sid string) string {
	return "nsu=" + ns + ";s=" + sid
}

// ns0 formats the elementId of a well-known OPC UA core node addressed by its
// BrowseName (e.g. BaseObjectType, FolderType, ObjectsFolder, HasComponent).
func ns0(browseName string) string {
	return elementID(opcUaNS, browseName)
}
