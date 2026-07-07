package nodeset

import (
	"os"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

func loadMini(t *testing.T) *Catalog {
	t.Helper()
	f, err := os.Open("testdata/mini.di.NodeSet2.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ns, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	ns0, err := loadNS0()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCatalog(ns0, ns)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCatalogMembers(t *testing.T) {
	c := loadMini(t)
	dt, _ := c.LookupType("http://opcfoundation.org/UA/DI/", "DeviceType")
	if len(dt.Members) != 1 {
		t.Fatalf("members = %+v", dt.Members)
	}
	m := dt.Members[0]
	if m.Name != "SerialNumber" || m.Kind != dsl.KindProperty {
		t.Errorf("member = %+v", m)
	}
}

func TestCatalogIdentityAndBase(t *testing.T) {
	c := loadMini(t)
	n, ok := c.Namespace("http://opcfoundation.org/UA/DI/")
	if !ok || n.Version != "1.04.0" || n.PublicationDate != "2022-11-01" {
		t.Fatalf("namespace = %+v ok=%v", n, ok)
	}
	dt, ok := c.LookupType("http://opcfoundation.org/UA/DI/", "DeviceType")
	if !ok {
		t.Fatal("DeviceType not found")
	}
	if dt.NodeClass != "ObjectType" || !dt.Abstract {
		t.Errorf("class=%q abstract=%v", dt.NodeClass, dt.Abstract)
	}
	if dt.BaseURI != OpcUaCoreURI || dt.BaseName != "BaseObjectType" {
		t.Errorf("base = %s:%s", dt.BaseURI, dt.BaseName)
	}
}

// ISA95URI is the historical 2013 namespace for the OPC UA ISA-95 companion spec.
const isa95TestURI = "http://www.OPCFoundation.org/UA/2013/01/ISA95"

func memberByName(members []dsl.CatalogMember, name string) (dsl.CatalogMember, bool) {
	for _, m := range members {
		if m.Name == name {
			return m, true
		}
	}
	return dsl.CatalogMember{}, false
}

// ISA-95 attaches members through custom reference types that are SUBTYPES of
// HasComponent / Aggregates (e.g. HasISA95Attribute, HasISA95ClassProperty,
// MadeUpOfEquipment). Member enumeration must follow those, not just the exact
// HasComponent/HasProperty reference types — otherwise EquipmentType silently
// reports only AssetAssignment and drops its real fields (QA finding).
func TestCatalogMembersFollowHierarchicalSubtypes(t *testing.T) {
	c, err := LoadForImports([]string{isa95TestURI})
	if err != nil {
		t.Fatal(err)
	}
	et, ok := c.LookupType(isa95TestURI, "EquipmentType")
	if !ok {
		t.Fatal("ISA95:EquipmentType not found")
	}

	// EquipmentLevel: a MANDATORY variable attached via HasISA95Attribute (a
	// subtype of HasComponent). This is the clearest dropped member.
	if m, ok := memberByName(et.Members, "EquipmentLevel"); !ok {
		t.Errorf("EquipmentLevel missing; members = %+v", et.Members)
	} else if m.Kind != dsl.KindVariable {
		t.Errorf("EquipmentLevel kind = %q, want variable", m.Kind)
	} else if m.Placeholder {
		t.Errorf("EquipmentLevel should not be a placeholder")
	}

	// AssetAssignment: plain HasComponent — must still be present (no regression).
	if _, ok := memberByName(et.Members, "AssetAssignment"); !ok {
		t.Errorf("AssetAssignment missing; members = %+v", et.Members)
	}

	// <PhysicalAsset> is reached via ImplementedBy, a NON-hierarchical reference
	// (an association, not composition). It must NOT be reported as a member.
	if _, ok := memberByName(et.Members, "<PhysicalAsset>"); ok {
		t.Errorf("<PhysicalAsset> is non-hierarchical and must not be a member; members = %+v", et.Members)
	}
}

func TestNS0LoadedButNotListed(t *testing.T) {
	c, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	// ns0 is indexed for resolution.
	if _, ok := c.Namespace(OpcUaCoreURI); !ok {
		t.Errorf("ns0 namespace should be loaded into the catalog")
	}
	if _, ok := c.LookupType(OpcUaCoreURI, "BaseObjectType"); !ok {
		t.Errorf("ns0 BaseObjectType should resolve once ns0 is loaded")
	}
	if _, ok := c.LookupType(OpcUaCoreURI, "Objects"); !ok {
		t.Errorf("ns0 Objects (an Object) should resolve by browse name")
	}
	// ns0 is NOT registered.
	for _, s := range Registry() {
		if s.URI == OpcUaCoreURI {
			t.Errorf("ns0 must not be in Registry()")
		}
	}
}

func TestTypeNamesExcludesNS0(t *testing.T) {
	c, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if names := c.TypeNames(OpcUaCoreURI); names != nil {
		t.Errorf("TypeNames(ns0) = %v, want nil (ns0 must never be listed)", names)
	}
}

// TestLookupResolvesSymbolicName asserts LookupType resolves both the BrowseName
// ("Objects") and the SymbolicName ("ObjectsFolder") for the ns0 Objects node.
// The DSL writes "under: OpcUa:ObjectsFolder" (the SymbolicName), while the
// NodeSet2 XML has BrowseName "Objects"; without the bySymbolic index that ref
// would resolve as unknown-import-type.
func TestLookupResolvesSymbolicName(t *testing.T) {
	c, err := LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.LookupType(OpcUaCoreURI, "ObjectsFolder"); !ok {
		t.Errorf("LookupType(ns0, %q) = false; want true (DSL uses SymbolicName)", "ObjectsFolder")
	}
	if _, ok := c.LookupType(OpcUaCoreURI, "Objects"); !ok {
		t.Errorf("LookupType(ns0, %q) = false; want true (browse name must still work)", "Objects")
	}
}

func TestCatalogMemberCarriesEnum(t *testing.T) {
	c, err := LoadForImports([]string{isa95TestURI})
	if err != nil {
		t.Fatal(err)
	}
	et, ok := c.LookupType(isa95TestURI, "EquipmentClassType")
	if !ok {
		t.Fatalf("EquipmentClassType not found")
	}
	m, ok := memberByName(et.Members, "EquipmentLevel")
	if !ok {
		t.Fatalf("EquipmentLevel member not found in %+v", et.Members)
	}
	if len(m.Enum) != 15 {
		t.Fatalf("EquipmentLevel enum has %d members, want 15", len(m.Enum))
	}
	if m.Enum[0].Name != "Enterprise" || m.Enum[0].Value != 0 {
		t.Errorf("Enum[0] = %d %s, want 0 Enterprise", m.Enum[0].Value, m.Enum[0].Name)
	}
	if m.Enum[14].Name != "Other" || m.Enum[14].Value != 14 {
		t.Errorf("Enum[14] = %d %s, want 14 Other", m.Enum[14].Value, m.Enum[14].Name)
	}
}

// Members carry their type definition (from HasTypeDefinition) so the UI can link
// a companion-typed member to that type's detail. <Equipment> is composed of
// EquipmentType (recursive), so its member type is ISA95:EquipmentType.
func TestCatalogMemberCarriesTypeDefinition(t *testing.T) {
	c, err := LoadForImports([]string{isa95TestURI})
	if err != nil {
		t.Fatal(err)
	}
	et, ok := c.LookupType(isa95TestURI, "EquipmentType")
	if !ok {
		t.Fatal("ISA95:EquipmentType not found")
	}
	eq, ok := memberByName(et.Members, "<Equipment>")
	if !ok {
		t.Fatalf("<Equipment> member missing; members = %+v", et.Members)
	}
	if eq.TypeURI != isa95TestURI || eq.TypeName != "EquipmentType" {
		t.Errorf("<Equipment> type = %s:%s, want %s:EquipmentType", eq.TypeURI, eq.TypeName, isa95TestURI)
	}
}
