package dsl

import "testing"

// ns0FakeCat resolves only the ns0 names we seed; Namespace(ns0) is present so
// nsBundled(ns0) is true.
type ns0FakeCat struct{ names map[string]bool }

func (f ns0FakeCat) Namespace(uri string) (CatalogNamespace, bool) {
	if uri == OpcUaNamespaceURI {
		return CatalogNamespace{URI: uri, Version: "1.05.03"}, true
	}
	return CatalogNamespace{}, false
}
func (f ns0FakeCat) LookupType(uri, name string) (CatalogType, bool) {
	if uri == OpcUaNamespaceURI && f.names[name] {
		return CatalogType{NamespaceURI: uri, Name: name, NodeClass: "ObjectType"}, true
	}
	return CatalogType{}, false
}

func TestResolveNS0NarrowedValidation(t *testing.T) {
	m := &Model{
		Imports: []Import{{Alias: "OpcUa", URI: OpcUaNamespaceURI}},
		Catalog: ns0FakeCat{names: map[string]bool{"BaseObjectType": true}},
	}
	found := m.ResolveType(TypeRef{Alias: "OpcUa", Name: "BaseObjectType"})
	if found.Kind != RefImport {
		t.Errorf("found ns0 ref Kind = %v, want RefImport (emit-safe)", found.Kind)
	}
	absent := m.ResolveType(TypeRef{Alias: "OpcUa", Name: "Foldr"})
	if absent.Kind != RefImportUnknown {
		t.Errorf("absent ns0 ref Kind = %v, want RefImportUnknown", absent.Kind)
	}
	// No catalog: stays trusted RefImport.
	m2 := &Model{Imports: []Import{{Alias: "OpcUa", URI: OpcUaNamespaceURI}}}
	if r := m2.ResolveType(TypeRef{Alias: "OpcUa", Name: "Anything"}); r.Kind != RefImport {
		t.Errorf("no-catalog ns0 ref Kind = %v, want RefImport", r.Kind)
	}
}

func mustParse(t *testing.T, src string) *Model {
	t.Helper()
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

func TestResolveTypeWithCatalog(t *testing.T) {
	m := &Model{
		Imports: []Import{{Alias: "DI", URI: "http://opcfoundation.org/UA/DI/"}},
		Catalog: fakeCatalog{
			ns: map[string]CatalogNamespace{"http://opcfoundation.org/UA/DI/": {URI: "http://opcfoundation.org/UA/DI/"}},
			types: map[string]CatalogType{
				"http://opcfoundation.org/UA/DI/|DeviceType": {NamespaceURI: "http://opcfoundation.org/UA/DI/", Name: "DeviceType", NodeClass: "ObjectType"},
			},
		},
	}
	if got := m.ResolveType(TypeRef{Alias: "DI", Name: "DeviceType", Raw: "DI:DeviceType"}); got.Kind != RefImportResolved {
		t.Errorf("DeviceType kind = %v, want RefImportResolved", got.Kind)
	}
	if got := m.ResolveType(TypeRef{Alias: "DI", Name: "Nonexistent", Raw: "DI:Nonexistent"}); got.Kind != RefImportUnknown {
		t.Errorf("Nonexistent kind = %v, want RefImportUnknown", got.Kind)
	}
	// ns0 stays trusted here: this catalog lacks the ns0 namespace, so
	// narrowing doesn't fire (only an ns0-loaded catalog narrows OpcUa:* refs).
	if got := m.ResolveType(TypeRef{Alias: "OpcUa", Name: "BaseObjectType", Raw: "OpcUa:BaseObjectType"}); got.Kind != RefImport {
		t.Errorf("OpcUa kind = %v, want RefImport", got.Kind)
	}
}

func TestResolveTypeNoCatalogIsLegacy(t *testing.T) {
	m := &Model{Imports: []Import{{Alias: "DI", URI: "http://opcfoundation.org/UA/DI/"}}}
	if got := m.ResolveType(TypeRef{Alias: "DI", Name: "DeviceType", Raw: "DI:DeviceType"}); got.Kind != RefImport {
		t.Errorf("no-catalog kind = %v, want RefImport (legacy trusted)", got.Kind)
	}
}

func TestResolveType(t *testing.T) {
	m := mustParse(t, `
model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }
imports: { OpcUa: http://opcfoundation.org/UA/ }
enums: { EquipmentState: { values: [A, B] } }
object_types: { FooType: { base: OpcUa:BaseObjectType } }
`)
	cases := []struct {
		raw  string
		want RefKind
	}{
		{"Double", RefBuiltin},
		{"String", RefBuiltin},
		{"UInt32", RefBuiltin},
		{"Boolean", RefBuiltin},
		{"EquipmentState", RefLocal},
		{"FooType", RefLocal},
		{"OpcUa:BaseObjectType", RefImport},
		{"OpcUa:FolderType", RefImport},
		{"Nope:Thing", RefUnknownAlias},
		{"Nonexistent", RefUnknownName},
	}
	for _, c := range cases {
		r := m.ResolveType(parseTypeRef(c.raw, Pos{}))
		if r.Kind != c.want {
			t.Errorf("ResolveType(%q).Kind = %v, want %v", c.raw, r.Kind, c.want)
		}
	}
}
