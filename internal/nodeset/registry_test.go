package nodeset

import "testing"

func TestSpecForURI(t *testing.T) {
	s, ok := SpecForURI("http://opcfoundation.org/UA/DI/")
	if !ok || s.Prefix != "Opc.Ua.DI" || s.Alias != "DI" || s.File == "" {
		t.Fatalf("DI spec = %+v ok=%v", s, ok)
	}
	if _, ok := SpecForURI("http://example.com/UA/Nope/"); ok {
		t.Error("unexpected match for unknown URI")
	}
}

func TestLoadAllCoversRegistry(t *testing.T) {
	c, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	for _, s := range Registry() {
		if _, ok := c.Namespace(s.URI); !ok {
			t.Errorf("LoadAll missing namespace %s (%s)", s.Alias, s.URI)
		}
	}
	// DI is a base spec every profile pulls in; DeviceType must resolve.
	if _, ok := c.LookupType("http://opcfoundation.org/UA/DI/", "DeviceType"); !ok {
		t.Error("LoadAll: DI:DeviceType not found")
	}
}

func TestSpecForRef(t *testing.T) {
	byAlias, ok := SpecForRef("DI")
	if !ok || byAlias.URI != "http://opcfoundation.org/UA/DI/" {
		t.Fatalf("SpecForRef(DI) = %+v, ok=%v", byAlias, ok)
	}
	byURI, ok := SpecForRef("http://opcfoundation.org/UA/DI/")
	if !ok || byURI.Alias != "DI" {
		t.Fatalf("SpecForRef(uri) = %+v, ok=%v", byURI, ok)
	}
	if _, ok := SpecForRef("nope"); ok {
		t.Error("SpecForRef(nope) should be false")
	}
}

func TestDependencyAliasesMachineryPullsDI(t *testing.T) {
	deps, err := DependencyAliases("http://opcfoundation.org/UA/Machinery/")
	if err != nil {
		t.Fatalf("DependencyAliases: %v", err)
	}
	found := false
	for _, a := range deps {
		if a == "Machinery" {
			t.Error("DependencyAliases must exclude the spec itself")
		}
		if a == "DI" {
			found = true
		}
	}
	if !found {
		t.Errorf("Machinery deps %v must include DI", deps)
	}
}
