package api

import "testing"

func TestSelectFile_EmptyNameMultipleFilesResolvesFirstSorted(t *testing.T) {
	files := map[string][]byte{
		"examples/sensors.yaml":   []byte("s"),
		"examples/equipment.yaml": []byte("e"),
	}
	path, data, ok := selectFile(files, "")
	if !ok {
		t.Fatal("empty name with multiple files must resolve (H1), not return ok=false")
	}
	if path != "examples/equipment.yaml" {
		t.Errorf("path = %q, want examples/equipment.yaml (first sorted key)", path)
	}
	if string(data) != "e" {
		t.Errorf("data = %q, want %q", data, "e")
	}
}

func TestSelectFile_EmptyNameSingleFileStillWorks(t *testing.T) {
	files := map[string][]byte{"only.yaml": []byte("x")}
	path, _, ok := selectFile(files, "")
	if !ok || path != "only.yaml" {
		t.Fatalf("single-file empty name: got (%q, ok=%v), want (only.yaml, true)", path, ok)
	}
}

func TestSelectFile_EmptyNameEmptyMapFails(t *testing.T) {
	if _, _, ok := selectFile(map[string][]byte{}, ""); ok {
		t.Error("empty map must return ok=false")
	}
}

func TestModelFilesOnly_ParseGatesNonModelYAML(t *testing.T) {
	tree := map[string][]byte{
		"equipment.yaml":              []byte("model:\n  name: X\n  prefix: X\n"),
		"broken-model.yaml":           []byte("model:\n  name: X\nobject_types:\n  T: { members: { bad } }\n"), // still a model (has model:) even if it won't fully parse
		".github/workflows/model.yml": []byte("name: model\non: [push]\njobs:\n  build:\n    runs-on: ubuntu\n"),
		"Taskfile.yml":                []byte("version: '3'\ntasks:\n  build:\n    cmds: [go build]\n"),
		"README.md":                   []byte("# not yaml"),
	}
	got := modelFilesOnly(tree)

	if _, ok := got["equipment.yaml"]; !ok {
		t.Errorf("equipment.yaml (has model:) should be kept")
	}
	if _, ok := got["broken-model.yaml"]; !ok {
		t.Errorf("broken-model.yaml (has model:, belt-and-suspenders) should be kept so its diagnostics surface")
	}
	if _, ok := got[".github/workflows/model.yml"]; ok {
		t.Errorf(".github/workflows/model.yml (no model: key) must be excluded")
	}
	if _, ok := got["Taskfile.yml"]; ok {
		t.Errorf("Taskfile.yml (no model: key) must be excluded")
	}
	if _, ok := got["README.md"]; ok {
		t.Errorf("README.md (not .yaml/.yml) must be excluded")
	}
}
