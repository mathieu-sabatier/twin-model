package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestE2ECompileDIExample drives the full YAML -> ModelDesign -> NodeSet2 pipeline
// end to end, feeding the DI companion spec as a -d2 dependency and running the
// official UA-ModelCompiler via its Docker image (built with `task modelcompiler-image`).
// It skips when Docker or the image is unavailable so `go test ./...` stays hermetic.
func TestE2ECompileDIExample(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not on PATH; skipping e2e compile")
	}
	inspect := exec.Command("docker", "image", "inspect", "ua-modelcompiler:local")
	inspect.Stderr = io.Discard // keep `go test -v` output pristine when skipping
	if err := inspect.Run(); err != nil {
		t.Skip("ua-modelcompiler:local image absent; run `task modelcompiler-image` to build it")
	}
	// The output dir is bind-mounted into the container, so it must live under a
	// path the Docker daemon shares (/tmp -> /private/tmp is shared by default on
	// Docker Desktop; always fine on Linux). t.TempDir() (/var/folders on macOS)
	// is not reliably shared, so use /tmp explicitly.
	out, err := os.MkdirTemp("/tmp", "twinmodel-e2e-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(out) })

	var so, se bytes.Buffer
	code := run([]string{"compile", "-i", "../../examples/di", "-o", out,
		"--docker-image", "ua-modelcompiler:local"}, &so, &se)
	if code != 0 {
		t.Fatalf("compile exit %d\nstdout:%s\nstderr:%s", code, so.String(), se.String())
	}
	matches, _ := filepath.Glob(filepath.Join(out, "*.NodeSet2.xml"))
	if len(matches) == 0 {
		t.Fatalf("no NodeSet2 produced in %s\nstdout:%s\nstderr:%s", out, so.String(), se.String())
	}
}
