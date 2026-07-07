package i3x

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

// goldenDir holds the committed i3X documents generated from examples/equipment.yaml.
const goldenDir = "../../examples/i3x"

// loadExampleModel parses the canonical example model.
func loadExampleModel(t *testing.T) *dsl.Model {
	t.Helper()
	src, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := dsl.Parse("examples/equipment.yaml", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

// TestEmitGolden is the capstone: the example model must transpile byte-for-byte
// to the committed i3X goldens. Set UPDATE=1 to (re)generate them.
func TestEmitGolden(t *testing.T) {
	m := loadExampleModel(t)
	b, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	if os.Getenv("UPDATE") == "1" {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range FileNames {
			if err := os.WriteFile(filepath.Join(goldenDir, name), b.File(name), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("updated goldens in %s", goldenDir)
		return
	}

	for _, name := range FileNames {
		got := b.File(name)
		want, err := os.ReadFile(filepath.Join(goldenDir, name))
		if err != nil {
			t.Fatalf("read golden %s: %v (run with UPDATE=1 to generate)", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s != golden\n%s", name, firstDiff(string(got), string(want)))
		}
	}
}

// firstDiff reports the first differing line between got and want.
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
