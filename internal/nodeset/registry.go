package nodeset

import (
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Spec is a bundled companion specification: its namespace URI, the embedded
// NodeSet2 filename, the UA-ModelCompiler code prefix, and the default DSL alias.
type Spec struct {
	URI    string
	File   string // base name under specs/
	Prefix string // ModelCompiler -d2 prefix
	Alias  string // default import alias
}

// registry is the curated set of bundled companion specs. Adding one is a row
// here plus committing the file under specs/ (Task 7).
var registry = []Spec{
	// Base layer
	{URI: "http://opcfoundation.org/UA/DI/", File: "Opc.Ua.Di.NodeSet2.xml", Prefix: "Opc.Ua.DI", Alias: "DI"},
	{URI: "http://opcfoundation.org/UA/IA/", File: "Opc.Ua.IA.NodeSet2.xml", Prefix: "Opc.Ua.IA", Alias: "IA"},
	{URI: "http://opcfoundation.org/UA/Dictionary/IRDI", File: "Opc.Ua.IRDI.NodeSet2.xml", Prefix: "Opc.Ua.IRDI", Alias: "IRDI"},
	{URI: "http://opcfoundation.org/UA/PADIM/", File: "Opc.Ua.PADIM.NodeSet2.xml", Prefix: "Opc.Ua.PADIM", Alias: "PADIM"},
	{URI: "http://opcfoundation.org/UA/PackML/", File: "Opc.Ua.PackML.NodeSet2.xml", Prefix: "Opc.Ua.PackML", Alias: "PackML"},
	// Machinery + building blocks
	{URI: "http://opcfoundation.org/UA/Machinery/", File: "Opc.Ua.Machinery.NodeSet2.xml", Prefix: "Opc.Ua.Machinery", Alias: "Machinery"},
	{URI: "http://opcfoundation.org/UA/Machinery/ProcessValues/", File: "Opc.Ua.Machinery.ProcessValues.NodeSet2.xml", Prefix: "Opc.Ua.Machinery.ProcessValues", Alias: "MachineryProcessValues"},
	{URI: "http://opcfoundation.org/UA/ISA95-JOBCONTROL_V2/", File: "Opc.Ua.ISA95-JOBCONTROL.NodeSet2.xml", Prefix: "Opc.Ua.ISA95-JOBCONTROL", Alias: "ISA95JobControl"},
	{URI: "http://opcfoundation.org/UA/Machinery/Jobs/", File: "Opc.Ua.Machinery.Jobs.NodeSet2.xml", Prefix: "Opc.Ua.Machinery.Jobs", Alias: "MachineryJobs"},
	// Verticals
	{URI: "http://opcfoundation.org/UA/MachineTool/", File: "Opc.Ua.MachineTool.NodeSet2.xml", Prefix: "Opc.Ua.MachineTool", Alias: "MachineTool"},
	{URI: "http://opcfoundation.org/UA/Robotics/", File: "Opc.Ua.Robotics.NodeSet2.xml", Prefix: "Opc.Ua.Robotics", Alias: "Robotics"},
	{URI: "http://opcfoundation.org/UA/Scales/V2/", File: "Opc.Ua.Scales.NodeSet2.xml", Prefix: "Opc.Ua.Scales", Alias: "Scales"},
	{URI: "http://www.OPCFoundation.org/UA/2013/01/ISA95", File: "Opc.ISA95.NodeSet2.xml", Prefix: "Opc.ISA95", Alias: "ISA95"},
}

// ns0File is the embedded base OPC UA NodeSet (namespace OpcUaCoreURI). It is
// loaded into every catalog for resolution/validation but is intentionally NOT
// in `registry`, so Registry(), the catalog tree, and search never surface it.
const ns0File = "Opc.Ua.NodeSet2.xml"

var (
	ns0Once sync.Once
	ns0Set  *NodeSet
	ns0Err  error
)

// loadNS0 parses the embedded base NodeSet once and caches it. The file is
// several MB / ~15k nodes; the sync.Once keeps repeated in-process loads free
// (the API builds its catalog once at startup; the CLI pays one parse per run).
func loadNS0() (*NodeSet, error) {
	ns0Once.Do(func() {
		f, err := openSpec(ns0File)
		if err != nil {
			ns0Err = err
			return
		}
		ns0Set, ns0Err = Parse(f)
		f.Close()
	})
	return ns0Set, ns0Err
}

// Registry returns the bundled companion specs, in declaration order.
func Registry() []Spec { return append([]Spec(nil), registry...) }

// SpecForURI returns the bundled spec for a namespace URI, if any.
func SpecForURI(uri string) (Spec, bool) {
	for _, s := range registry {
		if s.URI == uri {
			return s, true
		}
	}
	return Spec{}, false
}

// LoadForImports builds a Catalog for the given import namespace URIs, pulling in
// each bundled spec plus its transitive RequiredModel dependencies. Unbundled or
// ns0 URIs are skipped (they are validated elsewhere / always available).
func LoadForImports(uris []string) (*Catalog, error) {
	want := map[string]bool{}
	var queue []string
	enqueue := func(uri string) {
		if uri == "" || uri == OpcUaCoreURI || want[uri] {
			return
		}
		if _, ok := SpecForURI(uri); !ok {
			return
		}
		want[uri] = true
		queue = append(queue, uri)
	}
	for _, u := range uris {
		enqueue(u)
	}
	var sets []*NodeSet
	for i := 0; i < len(queue); i++ {
		spec, _ := SpecForURI(queue[i])
		f, err := openSpec(spec.File)
		if err != nil {
			return nil, err
		}
		ns, err := Parse(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		sets = append(sets, ns)
		for _, m := range ns.Models {
			for _, rm := range m.RequiredModels {
				enqueue(rm.ModelURI)
			}
		}
	}
	ns0, err := loadNS0()
	if err != nil {
		return nil, err
	}
	return NewCatalog(append([]*NodeSet{ns0}, sets...)...)
}

// Dependency is a bundled companion spec a model transitively requires.
type Dependency struct {
	Spec
}

// Dependencies returns the bundled specs a model needs for the given import
// URIs, transitively, dependencies-first and de-duplicated. Unbundled/ns0 URIs
// are ignored.
func Dependencies(uris []string) ([]Dependency, error) {
	seen := map[string]bool{}
	var out []Dependency
	var visit func(uri string) error
	visit = func(uri string) error {
		if uri == "" || uri == OpcUaCoreURI || seen[uri] {
			return nil
		}
		spec, ok := SpecForURI(uri)
		if !ok {
			return nil
		}
		seen[uri] = true
		f, err := openSpec(spec.File)
		if err != nil {
			return err
		}
		ns, err := Parse(f)
		f.Close()
		if err != nil {
			return err
		}
		for _, m := range ns.Models { // emit dependencies before self
			for _, rm := range m.RequiredModels {
				if err := visit(rm.ModelURI); err != nil {
					return err
				}
			}
		}
		out = append(out, Dependency{Spec: spec})
		return nil
	}
	for _, u := range uris {
		if err := visit(u); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Materialize writes each dependency's embedded NodeSet2 into dir and returns
// the written paths (in dependency order) for the ModelCompiler -d2 flags.
func Materialize(dir string, deps []Dependency) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var paths []string
	for _, d := range deps {
		src, err := openSpec(d.File)
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(dir, d.File)
		w, err := os.Create(dst)
		if err != nil {
			src.Close()
			return nil, err
		}
		_, err = io.Copy(w, src)
		if cerr := src.Close(); err == nil {
			err = cerr
		}
		// w.Close() can surface a deferred write error (disk full, NFS flush);
		// a discarded error would leave a truncated spec that breaks -d2 wiring.
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return nil, err
		}
		paths = append(paths, dst)
	}
	return paths, nil
}

// LoadAll builds a Catalog over every bundled spec (transitive RequiredModel
// deps are deduped by LoadForImports). Used by the API to serve global,
// draft-independent catalog discovery.
func LoadAll() (*Catalog, error) {
	uris := make([]string, 0, len(registry))
	for _, s := range registry {
		uris = append(uris, s.URI)
	}
	return LoadForImports(uris)
}

// SpecForRef returns the bundled spec identified by an alias (e.g. "DI") or a
// raw namespace URI.
func SpecForRef(ref string) (Spec, bool) {
	for _, s := range registry {
		if s.Alias == ref || s.URI == ref {
			return s, true
		}
	}
	return Spec{}, false
}

// DependencyAliases returns the default aliases of the transitive bundled specs
// that the given namespace URI requires, dependencies-first, excluding the spec
// itself. Empty (not an error) for an unbundled URI.
func DependencyAliases(uri string) ([]string, error) {
	deps, err := Dependencies([]string{uri})
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, d := range deps {
		if d.URI == uri {
			continue
		}
		out = append(out, d.Alias)
	}
	return out, nil
}
