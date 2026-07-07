package dsl

import "testing"

func TestResolveMembers(t *testing.T) {
	m := mustParse(t, hdr+`object_types:
  BaseT:
    base: OpcUa:BaseObjectType
    members:
      Manufacturer: { kind: property, type: String }
      State: { type: Double }
  DerivedT:
    base: BaseT
    members:
      State: { type: UInt32 }
      Extra: { type: Boolean }
`)
	got, err := m.ResolveMembers("DerivedT")
	if err != nil {
		t.Fatalf("ResolveMembers: %v", err)
	}
	// Most-derived-wins on name collisions; inherited members included.
	byName := map[string]ResolvedMember{}
	for _, rm := range got {
		byName[rm.Name] = rm
	}
	if len(byName) != 3 {
		t.Fatalf("got %d distinct members, want 3: %v", len(byName), names(got))
	}
	if byName["State"].Type.Raw != "UInt32" {
		t.Errorf("State should be overridden to UInt32, got %q", byName["State"].Type.Raw)
	}
	if byName["State"].DeclaredIn != "DerivedT" {
		t.Errorf("State.DeclaredIn = %q, want DerivedT", byName["State"].DeclaredIn)
	}
	if byName["Manufacturer"].DeclaredIn != "BaseT" {
		t.Errorf("Manufacturer.DeclaredIn = %q, want BaseT", byName["Manufacturer"].DeclaredIn)
	}
}

func TestResolveMembersUnknownType(t *testing.T) {
	m := mustParse(t, hdr)
	if _, err := m.ResolveMembers("Nope"); err == nil {
		t.Error("ResolveMembers(unknown) should error")
	}
}

func TestResolveMembersCrossesImportBoundary(t *testing.T) {
	m := &Model{
		Imports: []Import{{Alias: "DI", URI: "http://opcfoundation.org/UA/DI/"}},
		ObjectTypes: []*ObjectType{{
			Name:    "PumpType",
			Base:    TypeRef{Alias: "DI", Name: "DeviceType", Raw: "DI:DeviceType"},
			Members: []*Member{{Name: "FlowRate", Kind: KindVariable}},
		}},
		Catalog: fakeCatalog{
			ns: map[string]CatalogNamespace{"http://opcfoundation.org/UA/DI/": {URI: "http://opcfoundation.org/UA/DI/"}},
			types: map[string]CatalogType{
				"http://opcfoundation.org/UA/DI/|DeviceType": {
					NamespaceURI: "http://opcfoundation.org/UA/DI/", Name: "DeviceType", NodeClass: "ObjectType",
					Members: []CatalogMember{{Name: "SerialNumber", Kind: KindProperty}},
				},
			},
		},
	}
	rms, err := m.ResolveMembers("PumpType")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, rm := range rms {
		got[rm.Name] = rm.DeclaredIn
	}
	if got["FlowRate"] != "PumpType" {
		t.Errorf("FlowRate declaredIn = %q", got["FlowRate"])
	}
	if got["SerialNumber"] != "DI:DeviceType" {
		t.Errorf("inherited SerialNumber declaredIn = %q (want DI:DeviceType)", got["SerialNumber"])
	}
}

func names(rms []ResolvedMember) []string {
	var out []string
	for _, rm := range rms {
		out = append(out, rm.Name)
	}
	return out
}
