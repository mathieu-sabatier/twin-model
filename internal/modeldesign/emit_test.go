package modeldesign

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

// TestEmitGolden is the capstone: the canonical example YAML must transpile
// byte-for-byte to the XSD-validated golden ModelDesign.
func TestEmitGolden(t *testing.T) {
	src, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := dsl.Parse("examples/equipment.yaml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	want, err := os.ReadFile("../../examples/Equipment.ModelDesign.xml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("emit output != golden file\n%s", firstDiff(string(got), string(want)))
	}
}

func TestBuildNamespacesEnrichedFromCatalog(t *testing.T) {
	m := &dsl.Model{
		Name: "M", Namespace: "https://ex/UA/M/", Prefix: "Ex.M", Version: "1.0.0", PublicationDate: "2026-07-04",
		Imports: []dsl.Import{{Alias: "DI", URI: "http://opcfoundation.org/UA/DI/"}},
		Catalog: fakeCat{"http://opcfoundation.org/UA/DI/": dsl.CatalogNamespace{URI: "http://opcfoundation.org/UA/DI/", Version: "1.04.0", PublicationDate: "2022-11-01"}},
	}
	out, err := Emit(m)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `Version="1.04.0"`) || !strings.Contains(s, `PublicationDate="2022-11-01T00:00:00Z"`) {
		t.Errorf("DI namespace not enriched:\n%s", s)
	}
}

// fakeCat is a minimal dsl.Catalog for emit tests (only Namespace is used).
type fakeCat map[string]dsl.CatalogNamespace

func (f fakeCat) Namespace(uri string) (dsl.CatalogNamespace, bool) { n, ok := f[uri]; return n, ok }
func (f fakeCat) LookupType(uri, name string) (dsl.CatalogType, bool) {
	return dsl.CatalogType{}, false
}

// firstDiff returns a short report of the first differing line.
func firstDiff(got, want string) string {
	gl := strings.Split(got, "\n")
	wl := strings.Split(want, "\n")
	n := len(gl)
	if len(wl) < n {
		n = len(wl)
	}
	for i := 0; i < n; i++ {
		if gl[i] != wl[i] {
			return fmt.Sprintf("first diff at line %d:\n  got:  %q\n  want: %q", i+1, gl[i], wl[i])
		}
	}
	if len(gl) != len(wl) {
		return fmt.Sprintf("line counts differ: got %d, want %d", len(gl), len(wl))
	}
	return "(no line diff; trailing bytes differ)"
}
