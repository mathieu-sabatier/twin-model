package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilePrintCmd(t *testing.T) {
	dir := t.TempDir()
	yaml := `model:
  name: M
  namespace: https://ex/UA/M/
  prefix: Ex.M
  version: 1.0.0
  publication_date: 2026-07-04
imports:
  DI: http://opcfoundation.org/UA/DI/
object_types:
  PumpType: { base: DI:DeviceType }
`
	if err := os.WriteFile(filepath.Join(dir, "m.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	var so, se bytes.Buffer
	code := run([]string{"compile", "-i", dir, "-o", out, "--print-cmd"}, &so, &se)
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, se.String())
	}
	cmd := so.String()
	if !strings.Contains(cmd, "-d2") || !strings.Contains(cmd, "Opc.Ua.Di.NodeSet2.xml,Opc.Ua.DI,DI") {
		t.Errorf("expected DI -d2 dependency in printed cmd:\n%s", cmd)
	}
	if !strings.Contains(cmd, "M.ModelDesign.xml") {
		t.Errorf("expected the model design as primary -d2:\n%s", cmd)
	}
}

// TestCompileDockerPrintCmd verifies --docker-image builds a `docker run` invocation
// that mounts -o at /work and passes every compiler path relative to that mount.
func TestCompileDockerPrintCmd(t *testing.T) {
	dir := t.TempDir()
	yaml := `model:
  name: M
  namespace: https://ex/UA/M/
  prefix: Ex.M
  version: 1.0.0
  publication_date: 2026-07-04
imports:
  DI: http://opcfoundation.org/UA/DI/
object_types:
  PumpType: { base: DI:DeviceType }
`
	if err := os.WriteFile(filepath.Join(dir, "m.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	var so, se bytes.Buffer
	code := run([]string{"compile", "-i", dir, "-o", out, "--docker-image", "ua-modelcompiler:local", "--print-cmd"}, &so, &se)
	if code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, se.String())
	}
	cmd := so.String()
	// docker wrapper: run --rm, the mount, and the image name.
	for _, want := range []string{"docker run", "--rm", out + ":/work", "-w /work", "ua-modelcompiler:local"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("expected %q in docker invocation:\n%s", want, cmd)
		}
	}
	// Compiler paths are relative to the mount (./-prefixed), NOT absolute, and -o2 is the mount root.
	for _, want := range []string{"-d2 ./M.ModelDesign.xml", "./nodesets/Opc.Ua.Di.NodeSet2.xml,Opc.Ua.DI,DI", "-o2 ."} {
		if !strings.Contains(cmd, want) {
			t.Errorf("expected relative path %q in docker invocation:\n%s", want, cmd)
		}
	}
	if strings.Contains(cmd, out+"/M.ModelDesign.xml") {
		t.Errorf("compiler paths must be relative to the mount, not absolute:\n%s", cmd)
	}
}

func TestCompileRejectsCompilerAndDockerImage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.yaml"), []byte("model:\n  name: M\n  namespace: https://ex/UA/M/\n  version: 1.0.0\n  publication_date: 2026-07-04\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"compile", "-i", dir, "-o", filepath.Join(dir, "out"), "--compiler", "x", "--docker-image", "y", "--print-cmd"}, &so, &se)
	if code != 2 || !strings.Contains(se.String(), "mutually exclusive") {
		t.Fatalf("expected exit 2 + mutually-exclusive error; got exit %d stderr:\n%s", code, se.String())
	}
}
