package core

import (
	"log"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

// This file holds the helpers that operate on a draft/tree's file map
// (path→bytes): selecting a file by name, resolving a write target, filtering to
// model files, and deterministic key ordering. They are pure and independent of
// HTTP, so they are unit-testable on their own.

// selectFile picks a model file by exact repo-relative path or by unique
// basename. With an empty name it returns the first file in sorted-key order
// when the map is non-empty (so a no-?file= request always resolves — QA H1).
// ok is false only for an empty map, or a non-empty name that is unresolvable
// or an ambiguous basename.
func selectFile(files map[string][]byte, name string) (string, []byte, bool) {
	if name == "" {
		// No explicit ?file=: resolve to the first file in deterministic
		// (sorted) order — one file or many. Returning a bare 404 here for
		// multi-file drafts made the UI's no-file model fetch fail and spin
		// into an infinite draft-recreation loop (QA H1).
		keys := sortedKeys(files)
		if len(keys) == 0 {
			return "", nil, false
		}
		k := keys[0]
		return k, files[k], true
	}
	if path, ok := resolvePath(files, name); ok {
		return path, files[path], true
	}
	return "", nil, false
}

// resolveWriteKey maps an incoming file name to the key it should be written
// under: an existing exact path or unique basename, otherwise the name itself
// (a new file).
func resolveWriteKey(files map[string][]byte, name string) string {
	if path, ok := resolvePath(files, name); ok {
		return path
	}
	return name
}

// resolvePath resolves name to an existing key: an exact match first, else a
// unique basename match. ok is false when nothing matches or a basename is
// ambiguous. This is the shared core of selectFile and resolveWriteKey.
func resolvePath(files map[string][]byte, name string) (string, bool) {
	if _, ok := files[name]; ok {
		return name, true
	}
	var hit string
	n := 0
	for k := range files {
		if filepath.Base(k) == name {
			hit, n = k, n+1
		}
	}
	return hit, n == 1
}

// isModelFile reports whether raw YAML has a top-level `model:` key — the marker
// of a twinmodel model file. Infra YAML (.github/workflows/*, Taskfile.yml) has
// top-level keys like name:/on:/jobs:/version: and is rejected, so it never
// enters the editable/lintable surface. A model file with SEMANTIC errors still
// has a `model:` key and is kept (belt-and-suspenders), so its diagnostics
// surface normally rather than the whole file silently vanishing.
func isModelFile(raw []byte) bool {
	var top map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &top); err != nil {
		return false // not a YAML mapping → not a model
	}
	_, ok := top["model"]
	return ok
}

// modelFilesOnly keeps only *.yaml/*.yml entries that are twinmodel model files
// (top-level `model:` key). Non-model YAML is dropped and logged so the
// exclusion is observable, never silent at runtime.
func modelFilesOnly(tree map[string][]byte) map[string][]byte {
	out := map[string][]byte{}
	for k, v := range tree {
		switch filepath.Ext(k) {
		case ".yaml", ".yml":
			if isModelFile(v) {
				out[k] = v
			} else {
				log.Printf("twinmodel: skipping non-model YAML %q (no top-level model: key)", k)
			}
		}
	}
	return out
}

// canonicalize returns the canonical YAML for content when it parses; otherwise
// it returns the raw bytes, so the next model/validate read surfaces the
// structural error rather than silently dropping the edit.
func canonicalize(filename string, content []byte) []byte {
	m, err := dsl.Parse(filename, content)
	if err != nil {
		return content
	}
	formatted, err := dsl.Format(m)
	if err != nil {
		return content
	}
	return formatted
}

// sortedKeys returns the map's keys in deterministic (sorted) order.
func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CloneFiles returns a deep copy of a path→bytes file map. Exported for callers
// outside this package (test fakes, transport adapters).
func CloneFiles(in map[string][]byte) map[string][]byte { return copyFiles(in) }

// SortedKeys returns a file map's keys in deterministic (sorted) order.
func SortedKeys(m map[string][]byte) []string { return sortedKeys(m) }
