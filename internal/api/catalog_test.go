package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

func TestCatalogList(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out dto.CatalogListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	var di *dto.CatalogSpec
	for i := range out.Specs {
		if out.Specs[i].Alias == "DI" {
			di = &out.Specs[i]
		}
	}
	if di == nil {
		t.Fatalf("DI not in catalog list: %+v", out.Specs)
	}
	if di.Version == "" || di.URI != "http://opcfoundation.org/UA/DI/" {
		t.Errorf("DI spec incomplete: %+v", di)
	}
}

func TestCatalogListDependencies(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out dto.CatalogListResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	sawMachinery := false
	for _, s := range out.Specs {
		if s.Alias != "Machinery" {
			continue
		}
		sawMachinery = true
		found := false
		for _, d := range s.Dependencies {
			if d == "DI" {
				found = true
			}
		}
		if !found {
			t.Errorf("Machinery.dependencies %v must include DI", s.Dependencies)
		}
	}
	if !sawMachinery {
		t.Error("Machinery spec not found in catalog list")
	}
}

func TestCatalogTypes(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog/DI/types")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out dto.CatalogTypesResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	// Sorted + contains DeviceType.
	prev, found := "", false
	for _, ty := range out.Types {
		if ty.Name < prev {
			t.Errorf("types not sorted: %q after %q", ty.Name, prev)
		}
		prev = ty.Name
		if ty.Name == "DeviceType" {
			found = true
		}
	}
	if !found {
		t.Error("DI types must include DeviceType")
	}
}

func TestCatalogTypeDetail(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog/DI/types/DeviceType")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var d dto.CatalogTypeDetail
	_ = json.NewDecoder(resp.Body).Decode(&d)
	if d.Name != "DeviceType" || d.NodeClass != "ObjectType" || !d.Abstract {
		t.Errorf("unexpected detail: %+v", d)
	}
	if len(d.BaseChain) == 0 {
		t.Error("DeviceType must have a base chain")
	}
	hasManufacturer := false
	for _, m := range d.Members {
		if m.Name == "Manufacturer" {
			hasManufacturer = true
		}
	}
	if !hasManufacturer {
		t.Errorf("DeviceType members must include Manufacturer: %+v", d.Members)
	}
}

// A member whose type is a bundled companion type carries a linkable Type ref so
// the SPA can navigate to it. ISA95:EquipmentType is recursively composed of
// EquipmentType via its <Equipment> member.
func TestCatalogTypeMemberCarriesLinkableType(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog/ISA95/types/EquipmentType")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var d dto.CatalogTypeDetail
	_ = json.NewDecoder(resp.Body).Decode(&d)

	var eq *dto.CatalogMember
	for i := range d.Members {
		if d.Members[i].Name == "<Equipment>" {
			eq = &d.Members[i]
		}
	}
	if eq == nil {
		t.Fatalf("EquipmentType must have an <Equipment> member: %+v", d.Members)
	}
	if eq.Type == nil {
		t.Fatalf("<Equipment> must carry a linkable Type")
	}
	if eq.Type.Alias != "ISA95" || eq.Type.Name != "EquipmentType" {
		t.Errorf("<Equipment> type = %s:%s, want ISA95:EquipmentType", eq.Type.Alias, eq.Type.Name)
	}
}

func TestCatalogTypeDetailUnknown(t *testing.T) {
	ts, _, _ := newTestServer(t)
	for _, path := range []string{"/api/catalog/NOPE/types/X", "/api/catalog/DI/types/NoSuchType"} {
		resp, _ := http.Get(ts.URL + path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestCatalogSearch(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog/search?q=device")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out dto.CatalogSearchResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Hits) == 0 {
		t.Fatal("search device returned no hits")
	}
	for _, h := range out.Hits {
		if h.Alias == "" || h.Name == "" {
			t.Errorf("incomplete hit: %+v", h)
		}
	}
}

func TestCatalogSearchMissingQuery(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog/search")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCatalogTypeMemberCarriesEnum(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/catalog/ISA95/types/EquipmentClassType")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var d dto.CatalogTypeDetail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatal(err)
	}
	var lvl *dto.CatalogMember
	for i := range d.Members {
		if d.Members[i].Name == "EquipmentLevel" {
			lvl = &d.Members[i]
		}
	}
	if lvl == nil || lvl.Enum == nil {
		t.Fatalf("EquipmentLevel must carry an Enum; got %+v", d.Members)
	}
	if len(lvl.Enum.Members) != 15 {
		t.Errorf("enum has %d members, want 15", len(lvl.Enum.Members))
	}
	if lvl.Enum.Members[0].Name != "Enterprise" || lvl.Enum.Members[0].Value != 0 {
		t.Errorf("first = %d %s, want 0 Enterprise", lvl.Enum.Members[0].Value, lvl.Enum.Members[0].Name)
	}
}
