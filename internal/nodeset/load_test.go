package nodeset

import "testing"

func TestLoadForImportsTransitive(t *testing.T) {
	// Importing Machinery must transitively pull DI (Machinery RequiredModel).
	c, err := LoadForImports([]string{"http://opcfoundation.org/UA/Machinery/"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Namespace("http://opcfoundation.org/UA/Machinery/"); !ok {
		t.Error("Machinery namespace missing")
	}
	if _, ok := c.Namespace("http://opcfoundation.org/UA/DI/"); !ok {
		t.Error("DI namespace not transitively loaded")
	}
}

func TestLoadForImportsSkipsUnbundled(t *testing.T) {
	c, err := LoadForImports([]string{"http://example.com/UA/Nope/", OpcUaCoreURI})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := c.Namespace("http://example.com/UA/Nope/"); ok {
		t.Error("unbundled URI should not load")
	}
}
