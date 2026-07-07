package nodeset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDependenciesTransitiveOrder(t *testing.T) {
	deps, err := Dependencies([]string{"http://opcfoundation.org/UA/Machinery/"})
	if err != nil {
		t.Fatal(err)
	}
	// DI must appear before Machinery (dependencies first).
	var order []string
	for _, d := range deps {
		order = append(order, d.Alias)
	}
	di, mach := indexOf(order, "DI"), indexOf(order, "Machinery")
	if di < 0 || mach < 0 || di > mach {
		t.Fatalf("order = %v (DI must precede Machinery)", order)
	}
}

// Regression: Machinery's RequiredModel on http://opcfoundation.org/UA/IA/ must
// resolve to the bundled IA spec, so a Machinery import materializes IA into the
// ModelCompiler -d2 chain. Before IA was bundled, this dependency was silently
// dropped and a Machinery compile failed on the missing IA namespace.
// Found by /qa on 2026-07-05 (nodeset catalog imports story).
func TestMachineryImportPullsIA(t *testing.T) {
	deps, err := Dependencies([]string{"http://opcfoundation.org/UA/Machinery/"})
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, d := range deps {
		order = append(order, d.Alias)
	}
	ia, mach := indexOf(order, "IA"), indexOf(order, "Machinery")
	if ia < 0 {
		t.Fatalf("IA not pulled in transitively; order = %v", order)
	}
	if ia > mach {
		t.Fatalf("IA must precede Machinery (dependencies first); order = %v", order)
	}
}

func TestMaterialize(t *testing.T) {
	dir := t.TempDir()
	deps, _ := Dependencies([]string{"http://opcfoundation.org/UA/DI/"})
	paths, err := Materialize(dir, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(deps) {
		t.Fatalf("paths = %d deps = %d", len(paths), len(deps))
	}
	if _, err := os.Stat(filepath.Join(dir, "Opc.Ua.Di.NodeSet2.xml")); err != nil {
		t.Errorf("materialized DI missing: %v", err)
	}
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}
