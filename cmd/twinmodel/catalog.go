package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/nodeset"
)

// cmdCatalog exposes read-only discovery over the bundled companion specs.
func cmdCatalog(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "twinmodel catalog: want a subcommand (list|types|show|search)")
		return 2
	}
	switch args[0] {
	case "list":
		specs := nodeset.Registry()
		width := 0
		for _, s := range specs {
			if len(s.Alias) > width {
				width = len(s.Alias)
			}
		}
		for _, s := range specs {
			fmt.Fprintf(stdout, "%-*s  %s\n", width, s.Alias, s.URI)
		}
		return 0
	case "types":
		return catalogTypes(args[1:], stdout, stderr)
	case "show":
		return catalogShow(args[1:], stdout, stderr)
	case "search":
		return catalogSearch(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "twinmodel catalog: unknown subcommand %q\n", args[0])
		return 2
	}
}

// uriForRef resolves an alias (DI) or a raw URI to a bundled spec URI.
func uriForRef(ref string) (string, bool) {
	s, ok := nodeset.SpecForRef(ref)
	return s.URI, ok
}

func catalogTypes(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: twinmodel catalog types <alias|uri>")
		return 2
	}
	uri, ok := uriForRef(args[0])
	if !ok {
		fmt.Fprintf(stderr, "twinmodel catalog: unknown spec %q\n", args[0])
		return 2
	}
	c, err := nodeset.LoadForImports([]string{uri})
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel catalog: %v\n", err)
		return 1
	}
	names := c.TypeNames(uri)
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintln(stdout, n)
	}
	return 0
}

func catalogShow(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || !strings.Contains(args[0], ":") {
		fmt.Fprintln(stderr, "usage: twinmodel catalog show <alias|uri>:<Type>")
		return 2
	}
	ref, name, _ := strings.Cut(args[0], ":")
	uri, ok := uriForRef(ref)
	if !ok {
		fmt.Fprintf(stderr, "twinmodel catalog: unknown spec %q\n", ref)
		return 2
	}
	c, err := nodeset.LoadForImports([]string{uri})
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel catalog: %v\n", err)
		return 1
	}
	t, ok := c.LookupType(uri, name)
	if !ok {
		fmt.Fprintf(stderr, "twinmodel catalog: %s not found in %s\n", name, uri)
		return 1
	}
	fmt.Fprintf(stdout, "%s (%s)\n", t.Name, t.NodeClass)
	if t.Abstract {
		fmt.Fprintln(stdout, "  abstract: true")
	}
	if t.BaseName != "" {
		fmt.Fprintf(stdout, "  base: %s\n", t.BaseName)
	}
	fmt.Fprintln(stdout, "  members:")
	var members []struct{ Name, Kind string }
	for _, m := range t.Members {
		members = append(members, struct{ Name, Kind string }{m.Name, string(m.Kind)})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	for _, m := range members {
		fmt.Fprintf(stdout, "    %-24s %s\n", m.Name, m.Kind)
	}
	return 0
}

func catalogSearch(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: twinmodel catalog search <keyword>")
		return 2
	}
	kw := strings.ToLower(args[0])
	var uris []string
	for _, s := range nodeset.Registry() {
		uris = append(uris, s.URI)
	}
	c, err := nodeset.LoadForImports(uris)
	if err != nil {
		fmt.Fprintf(stderr, "twinmodel catalog: %v\n", err)
		return 1
	}
	type hit struct{ alias, name string }
	var hits []hit
	for _, s := range nodeset.Registry() {
		for _, n := range c.TypeNames(s.URI) {
			if strings.Contains(strings.ToLower(n), kw) {
				hits = append(hits, hit{s.Alias, n})
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].alias != hits[j].alias {
			return hits[i].alias < hits[j].alias
		}
		return hits[i].name < hits[j].name
	})
	if len(hits) == 0 {
		fmt.Fprintf(stderr, "twinmodel catalog: no types match %q (searched %d bundled spec(s))\n", args[0], len(nodeset.Registry()))
		return 0
	}
	for _, h := range hits {
		fmt.Fprintf(stdout, "%s:%s\n", h.alias, h.name)
	}
	return 0
}
