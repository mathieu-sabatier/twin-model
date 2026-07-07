package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

func TestOutputName(t *testing.T) {
	if got := outputName(&dsl.Model{Prefix: "Acme.Equipment"}); got != "Equipment.ModelDesign.xml" {
		t.Errorf("outputName from prefix = %q, want Equipment.ModelDesign.xml", got)
	}
	if got := outputName(&dsl.Model{Name: "PlainModel"}); got != "PlainModel.ModelDesign.xml" {
		t.Errorf("outputName name fallback = %q, want PlainModel.ModelDesign.xml", got)
	}
}

func TestCLIBuildProducesGolden(t *testing.T) {
	out := t.TempDir()
	var so, se bytes.Buffer
	if code := run([]string{"build", "-i", "../../examples", "-o", out}, &so, &se); code != 0 {
		t.Fatalf("build exit %d: %s", code, se.String())
	}
	got, err := os.ReadFile(filepath.Join(out, "Equipment.ModelDesign.xml"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	want, err := os.ReadFile("../../examples/Equipment.ModelDesign.xml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("built file != golden file")
	}
}

func TestCLIExportProducesGoldens(t *testing.T) {
	out := t.TempDir()
	var so, se bytes.Buffer
	if code := run([]string{"export", "--format", "i3x", "-i", "../../examples", "-o", out}, &so, &se); code != 0 {
		t.Fatalf("export exit %d: %s", code, se.String())
	}
	for _, name := range []string{"info.json", "namespaces.json", "relationshiptypes.json", "objecttypes.json", "objects.json"} {
		got, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join("../../examples/i3x", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("exported %s != golden", name)
		}
	}
}

// export defaults --format to i3x when the flag is omitted.
func TestCLIExportDefaultFormat(t *testing.T) {
	out := t.TempDir()
	var so, se bytes.Buffer
	if code := run([]string{"export", "-i", "../../examples", "-o", out}, &so, &se); code != 0 {
		t.Fatalf("export (default format) exit %d: %s", code, se.String())
	}
	if _, err := os.Stat(filepath.Join(out, "objecttypes.json")); err != nil {
		t.Errorf("expected objecttypes.json: %v", err)
	}
}

func TestCLIExportUnknownFormat(t *testing.T) {
	out := t.TempDir()
	var so, se bytes.Buffer
	if code := run([]string{"export", "--format", "bogus", "-i", "../../examples", "-o", out}, &so, &se); code != 2 {
		t.Errorf("unknown format exit=%d, want 2; stderr=%q", code, se.String())
	}
}

func TestCLILintCleanExample(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"lint", "-i", "../../examples"}, &so, &se); code != 0 {
		t.Errorf("lint of clean example exit=%d, out=%q err=%q", code, so.String(), se.String())
	}
}

func TestCLILintFailsOnBadModel(t *testing.T) {
	dir := t.TempDir()
	// namespace missing trailing slash -> error
	bad := "model: { name: M, namespace: https://x, version: 1.0.0, publication_date: 2026-07-02 }\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"lint", "-i", dir}, &so, &se)
	if code != 1 {
		t.Errorf("lint of bad model exit=%d, want 1; err=%q", code, se.String())
	}
	if !strings.Contains(se.String(), "namespace-trailing-slash") {
		t.Errorf("expected diagnostic code in stderr, got %q", se.String())
	}
}

func TestCLIBuildFailsOnBadModel(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	bad := "model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }\n" +
		"object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { type: Nope } } } }\n"
	os.WriteFile(filepath.Join(in, "m.yaml"), []byte(bad), 0o644)
	var so, se bytes.Buffer
	if code := run([]string{"build", "-i", in, "-o", out}, &so, &se); code != 1 {
		t.Errorf("build of bad model exit=%d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(out, "T.ModelDesign.xml")); err == nil {
		t.Errorf("build should not write output when validation fails")
	}
}

func TestCLISchema(t *testing.T) {
	var so, se bytes.Buffer
	code := run([]string{"schema"}, &so, &se)
	if code != 0 || !strings.Contains(so.String(), "$schema") {
		t.Errorf("schema exit=%d, out=%q", code, so.String())
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"frobnicate"}, &so, &se); code != 2 {
		t.Errorf("unknown command exit=%d, want 2", code)
	}
}

func TestServeRequiresRepo(t *testing.T) {
	t.Setenv("GIT_REPO", "")
	var out, errb bytes.Buffer
	if code := run([]string{"serve"}, &out, &errb); code == 0 {
		t.Fatalf("serve without GIT_REPO should fail, got exit 0")
	}
	if !strings.Contains(errb.String(), "GIT_REPO") {
		t.Errorf("stderr should mention GIT_REPO, got %q", errb.String())
	}
}

func TestServeConfigDefaults(t *testing.T) {
	t.Setenv("GIT_REPO", "https://github.com/o/r.git")
	t.Setenv("GIT_TOKEN", "tok")
	t.Setenv("DRAFT_TTL", "")
	t.Setenv("ADDR", "")
	cfg, err := buildServeConfig()
	if err != nil {
		t.Fatalf("buildServeConfig: %v", err)
	}
	if cfg.addr != ":8080" {
		t.Errorf("addr = %q, want :8080", cfg.addr)
	}
	if cfg.ttl != 2*time.Hour {
		t.Errorf("ttl = %v, want 2h", cfg.ttl)
	}
	if cfg.host.Owner != "o" || cfg.host.Repo != "r" {
		t.Errorf("owner/repo = %s/%s, want o/r", cfg.host.Owner, cfg.host.Repo)
	}
}

// A local filesystem path in GIT_REPO is accepted as a dev backend (no GitHub
// URL, no owner/repo parse) so the SPA can be developed against a local checkout.
func TestServeConfigLocalRepo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_REPO", dir)
	t.Setenv("GIT_TOKEN", "")
	t.Setenv("DRAFT_TTL", "")
	t.Setenv("ADDR", "")
	cfg, err := buildServeConfig()
	if err != nil {
		t.Fatalf("buildServeConfig(local path): %v", err)
	}
	if cfg.host.RepoURL != dir {
		t.Errorf("RepoURL = %q, want %q", cfg.host.RepoURL, dir)
	}
	if cfg.host.Owner != "" || cfg.host.Repo != "" {
		t.Errorf("owner/repo = %q/%q, want empty for a local repo", cfg.host.Owner, cfg.host.Repo)
	}
}

func TestLintFlagsUnbundledImport(t *testing.T) {
	dir := t.TempDir()
	yaml := `model:
  name: M
  namespace: https://ex/UA/M/
  version: 1.0.0
  publication_date: 2026-07-04
imports:
  X: http://example.com/UA/X/
object_types:
  T: { base: X:Thing }
`
	if err := os.WriteFile(filepath.Join(dir, "m.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"lint", "-i", dir}, &out, &errb)
	if code == 0 {
		t.Fatalf("expected non-zero exit; stderr:\n%s", errb.String())
	}
	if !strings.Contains(errb.String(), "import-not-bundled") {
		t.Errorf("expected import-not-bundled diagnostic; got:\n%s", errb.String())
	}
}

func TestFmtWritesCanonical(t *testing.T) {
	dir := t.TempDir()
	src := "model:\n  name: M\n  namespace: https://x/\n  version: 1.0.0\n  publication_date: 2026-07-02\n" +
		"object_types:\n  T:\n    base: OpcUa:BaseObjectType\n    members:\n      X: { kind: variable, type: Double }\n"
	path := filepath.Join(dir, "m.yaml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"fmt", "-i", dir, "-w"}, &out, &errb); code != 0 {
		t.Fatalf("fmt -w exit %d, stderr=%s", code, errb.String())
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "kind: variable") {
		t.Errorf("fmt did not drop default kind:\n%s", got)
	}
	// Running fmt without -w on the now-canonical file exits 0 (already formatted).
	out.Reset()
	errb.Reset()
	if code := run([]string{"fmt", "-i", dir}, &out, &errb); code != 0 {
		t.Errorf("fmt (check) on canonical file exit %d, want 0", code)
	}
}
