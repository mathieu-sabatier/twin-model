package mermaid

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

var update = flag.Bool("update", false, "update golden files")

func loadExample(t *testing.T) *dsl.Model {
	t.Helper()
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := dsl.Parse("equipment.yaml", data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	golden := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("%s drifted:\n--- got ---\n%s", name, got)
	}
}

func TestTypesDiagram(t *testing.T) {
	m := loadExample(t)
	got := TypesDiagram(m)
	if !strings.HasPrefix(got, "classDiagram\n") {
		t.Fatalf("want classDiagram header, got:\n%s", got)
	}
	checkGolden(t, "types.golden.mmd", got)
}

func TestInstancesDiagram(t *testing.T) {
	m := loadExample(t)
	got := InstancesDiagram(m)
	if !strings.HasPrefix(got, "flowchart TD\n") {
		t.Fatalf("want flowchart header, got:\n%s", got)
	}
	checkGolden(t, "instances.golden.mmd", got)
}

// TestDiagramDeterministic: same input, identical output. The results are bound
// to separate variables so the comparison reflects two independent calls rather
// than a self-comparison the linter would flag.
func TestDiagramDeterministic(t *testing.T) {
	m := loadExample(t)
	types1, types2 := TypesDiagram(m), TypesDiagram(m)
	inst1, inst2 := InstancesDiagram(m), InstancesDiagram(m)
	if types1 != types2 || inst1 != inst2 {
		t.Error("diagram output is not deterministic")
	}
}
