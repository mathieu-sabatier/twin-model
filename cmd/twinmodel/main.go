// Command twinmodel transpiles the YAML DSL to a UA-ModelCompiler ModelDesign.xml,
// lints and formats it, prints the DSL JSON Schema, and serves the HTTP API.
//
//	twinmodel build  -i model/ -o out/   # *.yaml -> *.ModelDesign.xml
//	twinmodel export -i model/ -o out/   # *.yaml -> i3X JSON (--format i3x)
//	twinmodel lint   -i model/           # semantic checks, CI exit codes
//	twinmodel fmt    -i model/ -w        # canonically format *.yaml in place
//	twinmodel schema                     # print the DSL JSON Schema
//	twinmodel serve                      # serve the HTTP API (env: GIT_REPO, ...)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/app"
	"github.com/mathieu-sabatier/twin-model/internal/core"
	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/i3x"
	"github.com/mathieu-sabatier/twin-model/internal/modeldesign"
	"github.com/mathieu-sabatier/twin-model/internal/nodeset"
	"github.com/mathieu-sabatier/twin-model/schema"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "build":
		return cmdBuild(args[1:], stdout, stderr)
	case "export":
		return cmdExport(args[1:], stdout, stderr)
	case "lint":
		return cmdLint(args[1:], stdout, stderr)
	case "schema":
		return cmdSchema(args[1:], stdout, stderr)
	case "fmt":
		return cmdFmt(args[1:], stdout, stderr)
	case "serve":
		return cmdServe(args[1:], stdout, stderr)
	case "mcp":
		return cmdMCP(args[1:], stdout, stderr)
	case "catalog":
		return cmdCatalog(args[1:], stdout, stderr)
	case "compile":
		return cmdCompile(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "twinmodel: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `twinmodel — YAML -> OPC UA ModelDesign transpiler

Usage:
  twinmodel build  -i <dir> -o <dir>   Transpile *.yaml to *.ModelDesign.xml
  twinmodel export -i <dir> -o <dir>   Export the model to i3X JSON (--format i3x)
  twinmodel lint   -i <dir>            Semantic checks only (exit 1 on error)
  twinmodel schema                     Print the DSL JSON Schema to stdout
  twinmodel fmt    -i <dir> [-w]        Canonically format *.yaml (in place with -w)
  twinmodel serve                      Serve the HTTP API (env: GIT_REPO, GIT_TOKEN, DRAFT_TTL, ADDR)
  twinmodel mcp                        Serve the MCP tools over stdio (env: GIT_REPO, GIT_TOKEN, ...)
  twinmodel catalog list|types|show|search   Explore bundled companion specs (DI, Machinery, ISA-95)
  twinmodel compile -i <dir> -o <dir>        Transpile + run the ModelCompiler -> NodeSet2 (needs .NET; --print-cmd to preview)
`)
}

// cmdServe builds the fx-wired serve app from env and serves until interrupted.
func cmdServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// Gate on config (and eager host construction) before starting the fx app so
	// any config-derived error -- missing GIT_REPO, a malformed GIT_REPO URL,
	// etc. -- exits 2 with a unified message (matching the pre-fx
	// buildServeConfig behavior) instead of surfacing only via fx's invoke chain
	// as a raw error dump on os.Exit(1). RunServe still builds its own graph;
	// this call is purely a validation gate.
	cfg, err := core.ConfigFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel serve: %v\n", err)
		return 2
	}
	if _, err := core.NewGitHost(cfg); err != nil {
		fmt.Fprintf(stderr, "twinmodel serve: %v\n", err)
		return 2
	}
	if err := app.RunServe(); err != nil {
		fmt.Fprintf(stderr, "twinmodel serve: %v\n", err)
		return 1
	}
	return 0
}

// cmdMCP builds the fx-wired stdio MCP app from env and serves until stdin
// closes. Mirrors cmdServe's config gate: any config-derived error (missing
// GIT_REPO, a malformed GIT_REPO URL, etc.) exits 2 with a unified message
// instead of surfacing only via fx's invoke chain as a raw error dump.
func cmdMCP(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, err := core.ConfigFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel mcp: %v\n", err)
		return 2
	}
	if _, err := core.NewGitHost(cfg); err != nil {
		fmt.Fprintf(stderr, "twinmodel mcp: %v\n", err)
		return 2
	}
	if err := app.RunMCPStdio(); err != nil {
		fmt.Fprintf(stderr, "twinmodel mcp: %v\n", err)
		return 1
	}
	return 0
}

// cmdBuild parses, validates, and emits every model in the input directory.
func cmdBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("i", "", "input directory containing *.yaml (required)")
	out := fs.String("o", "", "output directory for *.ModelDesign.xml (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" || *out == "" {
		fmt.Fprintln(stderr, "twinmodel build: -i and -o are required")
		return 2
	}

	files, err := modelFiles(*in)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel build: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "twinmodel build: no *.yaml files in %s\n", *in)
		return 2
	}

	type built struct {
		name string
		xml  []byte
	}
	var outputs []built
	errors := 0
	for _, f := range files {
		m, diags, err := loadModel(f)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			errors++
			continue
		}
		e, _ := printDiagnostics(stderr, diags)
		errors += e
		if e > 0 {
			continue // don't emit an invalid model
		}
		xmlBytes, err := modeldesign.Emit(m)
		if err != nil {
			fmt.Fprintf(stderr, "%s: emit: %v\n", f, err)
			errors++
			continue
		}
		outputs = append(outputs, built{name: outputName(m), xml: xmlBytes})
	}
	if errors > 0 {
		return 1
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "twinmodel build: %v\n", err)
		return 1
	}
	for _, b := range outputs {
		dst := filepath.Join(*out, b.name)
		if err := os.WriteFile(dst, b.xml, 0o644); err != nil {
			fmt.Fprintf(stderr, "twinmodel build: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "wrote %s\n", dst)
	}
	return 0
}

// cmdExport transpiles the model(s) in -i to i3X JSON documents under -o. It
// mirrors cmdBuild's load/validate flow but writes the five-file i3X bundle
// (info, namespaces, relationshiptypes, objecttypes, objects). --format defaults
// to i3x; any other value is rejected. Fixed filenames mean -i is expected to
// hold a single model.
func cmdExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "i3x", "export format (i3x)")
	in := fs.String("i", "", "input directory containing *.yaml (required)")
	out := fs.String("o", "", "output directory for the i3X JSON documents (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *format != "i3x" {
		fmt.Fprintf(stderr, "twinmodel export: unsupported --format %q (only i3x)\n", *format)
		return 2
	}
	if *in == "" || *out == "" {
		fmt.Fprintln(stderr, "twinmodel export: -i and -o are required")
		return 2
	}

	files, err := modelFiles(*in)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel export: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "twinmodel export: no *.yaml files in %s\n", *in)
		return 2
	}

	var bundles []i3x.Bundle
	errors := 0
	for _, f := range files {
		m, diags, err := loadModel(f)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			errors++
			continue
		}
		e, _ := printDiagnostics(stderr, diags)
		errors += e
		if e > 0 {
			continue // don't emit an invalid model
		}
		b, err := i3x.Emit(m)
		if err != nil {
			fmt.Fprintf(stderr, "%s: export: %v\n", f, err)
			errors++
			continue
		}
		bundles = append(bundles, b)
	}
	if errors > 0 {
		return 1
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "twinmodel export: %v\n", err)
		return 1
	}
	for _, b := range bundles {
		for _, name := range i3x.FileNames {
			dst := filepath.Join(*out, name)
			if err := os.WriteFile(dst, b.File(name), 0o644); err != nil {
				fmt.Fprintf(stderr, "twinmodel export: %v\n", err)
				return 1
			}
			fmt.Fprintf(stdout, "wrote %s\n", dst)
		}
	}
	return 0
}

// cmdLint parses and validates every model, printing diagnostics.
func cmdLint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("i", "", "input directory or file (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" {
		fmt.Fprintln(stderr, "twinmodel lint: -i is required")
		return 2
	}

	files, err := modelFiles(*in)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel lint: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "twinmodel lint: no *.yaml files in %s\n", *in)
		return 2
	}

	errors, warnings := 0, 0
	for _, f := range files {
		_, diags, err := loadModel(f)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			errors++
			continue
		}
		e, w := printDiagnostics(stderr, diags)
		errors += e
		warnings += w
	}
	fmt.Fprintf(stdout, "checked %d file(s): %d error(s), %d warning(s)\n", len(files), errors, warnings)
	if errors > 0 {
		return 1
	}
	return 0
}

func cmdSchema(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fmt.Fprint(stdout, schema.JSON)
	return 0
}

// cmdFmt canonically formats every model file. Without -w it prints the
// formatted output; with -w it rewrites files in place. Exit 1 if a file is not
// already canonical (and -w was not given), so CI can enforce canonical form.
func cmdFmt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("i", "", "input directory or file (required)")
	write := fs.Bool("w", false, "rewrite files in place")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" {
		fmt.Fprintln(stderr, "twinmodel fmt: -i is required")
		return 2
	}
	files, err := modelFiles(*in)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel fmt: %v\n", err)
		return 2
	}
	unformatted := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		m, err := dsl.Parse(f, data)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		out, err := dsl.Format(m)
		if err != nil {
			fmt.Fprintf(stderr, "%s: format: %v\n", f, err)
			return 1
		}
		switch {
		case *write:
			if !bytes.Equal(data, out) {
				if err := os.WriteFile(f, out, 0o644); err != nil {
					fmt.Fprintf(stderr, "twinmodel fmt: %v\n", err)
					return 1
				}
				fmt.Fprintf(stdout, "formatted %s\n", f)
			}
		case !bytes.Equal(data, out):
			unformatted++
			fmt.Fprintf(stderr, "%s: not canonical (run `twinmodel fmt -w`)\n", f)
		default:
			stdout.Write(out)
		}
	}
	if unformatted > 0 {
		return 1
	}
	return 0
}

// --- helpers -----------------------------------------------------------------

// loadModel reads, parses, attaches a companion-spec catalog, and validates a
// single file. A parse error is fatal (returned as err); semantic findings come
// back as diags.
func loadModel(path string) (*dsl.Model, []dsl.Diagnostic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	m, err := dsl.Parse(path, data)
	if err != nil {
		return nil, nil, err
	}
	var uris []string
	for _, im := range m.Imports {
		uris = append(uris, im.URI)
	}
	cat, err := nodeset.LoadForImports(uris)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: load companion specs: %w", path, err)
	}
	m.Catalog = cat
	return m, dsl.Validate(m), nil
}

// modelFiles returns the sorted *.yaml/*.yml files for a directory, or the file
// itself if a file path was given.
func modelFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	var files []string
	for _, pat := range []string{"*.yaml", "*.yml"} {
		matches, err := filepath.Glob(filepath.Join(path, pat))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	return files, nil
}

// printDiagnostics writes each diagnostic and returns the error and warning counts.
func printDiagnostics(w io.Writer, diags []dsl.Diagnostic) (errs, warns int) {
	for _, d := range diags {
		fmt.Fprintln(w, d.String())
		if d.Severity == dsl.SeverityError {
			errs++
		} else {
			warns++
		}
	}
	return errs, warns
}

// outputName derives <ShortName>.ModelDesign.xml from the model prefix's last
// dotted segment (e.g. Acme.Equipment -> Equipment), falling back to the
// model name.
func outputName(m *dsl.Model) string {
	short := m.Name
	if m.Prefix != "" {
		parts := strings.Split(m.Prefix, ".")
		short = parts[len(parts)-1]
	}
	return short + ".ModelDesign.xml"
}
