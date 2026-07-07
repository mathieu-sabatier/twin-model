package dsl

import "testing"

// fakeCatalog is a hand-built Catalog used across dsl tests.
type fakeCatalog struct {
	ns    map[string]CatalogNamespace
	types map[string]CatalogType // key: uri + "|" + name
}

func (f fakeCatalog) Namespace(uri string) (CatalogNamespace, bool) { n, ok := f.ns[uri]; return n, ok }
func (f fakeCatalog) LookupType(uri, name string) (CatalogType, bool) {
	t, ok := f.types[uri+"|"+name]
	return t, ok
}

func TestCatalogInterfaceSatisfied(t *testing.T) {
	var c Catalog = fakeCatalog{
		ns: map[string]CatalogNamespace{"u": {URI: "u", Version: "1.0.0"}},
		types: map[string]CatalogType{
			"u|DeviceType": {NamespaceURI: "u", Name: "DeviceType", NodeClass: "ObjectType", Abstract: true,
				Members: []CatalogMember{{Name: "SerialNumber", Kind: KindProperty}}},
		},
	}
	if _, ok := c.Namespace("u"); !ok {
		t.Fatal("Namespace")
	}
	dt, ok := c.LookupType("u", "DeviceType")
	if !ok || dt.NodeClass != "ObjectType" || !dt.Abstract || len(dt.Members) != 1 {
		t.Fatalf("LookupType = %+v ok=%v", dt, ok)
	}
}
