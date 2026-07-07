package nodeset

import "testing"

func TestResolveNodeID(t *testing.T) {
	ns := &NodeSet{NamespaceURIs: []string{"http://opcfoundation.org/UA/DI/"}}
	// ns index 0 -> OPC UA core (implicit, not in the table)
	core, err := ns.resolveNodeID("i=58")
	if err != nil || core.URI != OpcUaCoreURI || core.ID != "i=58" {
		t.Fatalf("core = %+v err=%v", core, err)
	}
	// ns index 1 -> first entry of the table
	di, err := ns.resolveNodeID("ns=1;i=1002")
	if err != nil || di.URI != "http://opcfoundation.org/UA/DI/" || di.ID != "i=1002" {
		t.Fatalf("di = %+v err=%v", di, err)
	}
	// malformed ns indices must error, never panic (trailing junk / negative / out of range).
	for _, bad := range []string{"ns=1abc;i=99", "ns=-1;i=99", "ns=5;i=1"} {
		if _, err := ns.resolveNodeID(bad); err == nil {
			t.Errorf("resolveNodeID(%q) = nil error, want error", bad)
		}
	}
}

func TestResolveRefType(t *testing.T) {
	ns := &NodeSet{Aliases: []Alias{{Name: "HasSubtype", Value: "i=45"}}}
	if got := ns.resolveRefType(Reference{Type: "HasSubtype"}); got != "i=45" {
		t.Errorf("alias -> %q", got)
	}
	if got := ns.resolveRefType(Reference{Type: "i=47"}); got != "i=47" {
		t.Errorf("nodeid passthrough -> %q", got)
	}
}
