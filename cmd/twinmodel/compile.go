package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/modeldesign"
	"github.com/mathieu-sabatier/twin-model/internal/nodeset"
)

// cmdCompile runs the full YAML -> ModelDesign -> NodeSet2 pipeline by
// orchestrating the official UA-ModelCompiler with the imported companion specs
// supplied as -d2 dependencies.
func cmdCompile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("i", "", "input directory containing a single *.yaml model (required)")
	out := fs.String("o", "", "output directory (required)")
	ids := fs.String("ids", "", "NodeId CSV (bootstrapped with -cg if absent, else -c)")
	compiler := fs.String("compiler", "", "path to Opc.Ua.ModelCompiler (default: found on PATH)")
	dockerImage := fs.String("docker-image", "", "run the ModelCompiler via this Docker image (mounts -o at /work) instead of a native binary")
	version := fs.String("version", "v105", "UA-ModelCompiler -version")
	printCmd := fs.Bool("print-cmd", false, "print the ModelCompiler invocation without running it")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" || *out == "" {
		fmt.Fprintln(stderr, "twinmodel compile: -i and -o are required")
		return 2
	}
	if *compiler != "" && *dockerImage != "" {
		fmt.Fprintln(stderr, "twinmodel compile: --compiler and --docker-image are mutually exclusive")
		return 2
	}
	files, err := modelFiles(*in)
	if err != nil || len(files) == 0 {
		fmt.Fprintf(stderr, "twinmodel compile: no *.yaml in %s\n", *in)
		return 2
	}
	if len(files) != 1 {
		fmt.Fprintf(stderr, "twinmodel compile: -i must hold exactly one model (found %d)\n", len(files))
		return 2
	}

	m, diags, err := loadModel(files[0])
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if e, _ := printDiagnostics(stderr, diags); e > 0 {
		fmt.Fprintln(stderr, "twinmodel compile: refusing to compile a model with errors")
		return 1
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
		return 1
	}
	xmlBytes, err := modeldesign.Emit(m)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: emit: %v\n", err)
		return 1
	}
	modelPath := filepath.Join(*out, outputName(m))
	if err := os.WriteFile(modelPath, xmlBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
		return 1
	}

	var uris []string
	for _, im := range m.Imports {
		uris = append(uris, im.URI)
	}
	deps, err := nodeset.Dependencies(uris)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
		return 1
	}
	depPaths, err := nodeset.Materialize(filepath.Join(*out, "nodesets"), deps)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
		return 1
	}

	csvFlag := "-cg"
	csv := *ids
	if csv == "" {
		csv = modelPath[:len(modelPath)-len(".ModelDesign.xml")] + ".ModelDesign.csv"
	}
	if _, err := os.Stat(csv); err == nil {
		csvFlag = "-c"
	}

	// In --docker-image mode the output dir is mounted at /work, so every path the
	// compiler sees must be expressed relative to it; natively they pass through.
	dockerMode := *dockerImage != ""
	absOut, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
		return 1
	}
	pathArg := func(p string) (string, error) {
		if !dockerMode {
			return p, nil
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(absOut, abs)
		if err != nil {
			return "", err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("path %s is outside -o (required for --docker-image; place it under %s)", p, *out)
		}
		// Prefix "./" so a bare filename still has a non-empty directory part: the
		// ModelCompiler calls Path.GetDirectoryName()+CreateDirectory() on the CSV
		// path and throws on an empty string. Container paths are always POSIX.
		return "./" + filepath.ToSlash(rel), nil
	}

	// Assemble the compiler arg vector: primary model first, then dependency specs.
	modelArg, err := pathArg(modelPath)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
		return 1
	}
	compilerArgv := []string{"compile", "-d2", modelArg}
	for i, d := range deps {
		depArg, err := pathArg(depPaths[i])
		if err != nil {
			fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
			return 1
		}
		compilerArgv = append(compilerArgv, "-d2", fmt.Sprintf("%s,%s,%s", depArg, d.Prefix, d.Alias))
	}
	csvArg, err := pathArg(csv)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: %v\n", err)
		return 1
	}
	o2Arg := *out
	if dockerMode {
		o2Arg = "."
	}
	compilerArgv = append(compilerArgv, csvFlag, csvArg, "-o2", o2Arg, "-version", *version)

	// Pick the executable and full arg vector for the chosen mode.
	name := *compiler
	argv := compilerArgv
	switch {
	case dockerMode:
		name = "docker"
		argv = append([]string{
			"run", "--rm",
			"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			"-e", "HOME=/tmp",
			"-v", absOut + ":/work",
			"-w", "/work",
			*dockerImage,
		}, compilerArgv...)
	case name == "":
		name = "Opc.Ua.ModelCompiler"
	}

	if *printCmd {
		fmt.Fprintf(stdout, "%s %s\n", name, quoteArgs(argv))
		return 0
	}

	path, lookErr := exec.LookPath(name)
	if lookErr != nil {
		if dockerMode {
			fmt.Fprintf(stderr, "twinmodel compile: %q not found on PATH (required for --docker-image).\n", name)
		} else {
			fmt.Fprintf(stderr, "twinmodel compile: %q not found on PATH.\n"+
				"Install it once with:\n  dotnet tool install --global OPCFoundation.Opc.Ua.ModelCompiler.Tool\n"+
				"or pass --compiler <path>, --docker-image <image>, or use --print-cmd to emit the invocation.\n", name)
		}
		return 1
	}
	cmd := exec.Command(path, argv...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "twinmodel compile: ModelCompiler failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "compiled %s -> NodeSet2 in %s\n", modelPath, *out)
	return 0
}

// quoteArgs renders an arg vector for display, quoting args with spaces/commas.
func quoteArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		if containsAny(a, " \t,") {
			out += fmt.Sprintf("%q", a)
		} else {
			out += a
		}
	}
	return out
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		for _, x := range s {
			if x == c {
				return true
			}
		}
	}
	return false
}
