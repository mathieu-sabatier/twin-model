package i3x

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"
)

// csvKeys reads the SymbolicId column (first field) of the committed ModelDesign
// CSV — the NodeId source of truth produced by the UA-ModelCompiler.
func csvKeys(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open("../../examples/Equipment.ModelDesign.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	keys := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		keys[strings.SplitN(line, ",", 2)[0]] = true
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}

var modelNSElementRe = regexp.MustCompile(`nsu=https://acme\.example/UA/Equipment/;s=([A-Za-z0-9_]+)`)

// TestSymbolicIDsMatchModelDesignCSV is the identity proof: every model-namespace
// SymbolicId the exporter emits (reconstructed purely from the AST) is a key in
// the ModelCompiler CSV. This is what lets an i3X consumer join back to the
// NodeSet by SymbolicId without the CSV ever being an input to this exporter.
func TestSymbolicIDsMatchModelDesignCSV(t *testing.T) {
	keys := csvKeys(t)
	m := loadExampleModel(t)
	b, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	emitted := map[string]bool{}
	for _, name := range FileNames {
		for _, mm := range modelNSElementRe.FindAllStringSubmatch(string(b.File(name)), -1) {
			emitted[mm[1]] = true
		}
	}
	if len(emitted) == 0 {
		t.Fatal("no model-namespace SymbolicIds emitted")
	}
	for sid := range emitted {
		if !keys[sid] {
			t.Errorf("emitted SymbolicId %q is not a key in the ModelDesign CSV", sid)
		}
	}
}

// TestModelledSubsetHasExpectedIDs cross-checks the reverse for the subset we
// model: the types, instances, and composed objects we expect are all emitted.
func TestModelledSubsetHasExpectedIDs(t *testing.T) {
	m := loadExampleModel(t)
	b, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	emitted := map[string]bool{}
	for _, name := range FileNames {
		for _, mm := range modelNSElementRe.FindAllStringSubmatch(string(b.File(name)), -1) {
			emitted[mm[1]] = true
		}
	}
	for _, want := range []string{
		"EquipmentType", "HeatingZoneType", "FurnaceType", "PressType", // types
		"Furnace01", "Press01", "Furnace02", // instances
		"Furnace01_Zones", "Furnace02_Zones", "Furnace02_Zone1", "Furnace02_Zone2", // composed
	} {
		if !emitted[want] {
			t.Errorf("expected SymbolicId %q to be emitted", want)
		}
	}
}
