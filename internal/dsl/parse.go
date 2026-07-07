package dsl

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse reads the YAML DSL into a Model. It returns an error for structural
// problems (malformed YAML, wrong shapes, unknown keys, missing required
// fields). Semantic checks live in Validate. Source line numbers are captured so
// diagnostics can report file:line.
func Parse(filename string, data []byte) (*Model, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("%s: empty document", filename)
	}
	root := doc.Content[0]
	if err := expectMapping(filename, "top level", root); err != nil {
		return nil, err
	}

	m := &Model{Pos: Pos{File: filename}}
	m.HeadComment = doc.HeadComment
	var sawModel bool
	for _, e := range mapEntries(root) {
		var err error
		switch e.key {
		case "model":
			sawModel = true
			m.Pos = Pos{File: filename, Line: e.keyLine, Col: e.keyCol}
			err = parseModelHeader(filename, e.val, m)
		case "imports":
			m.Imports, err = parseImports(filename, e.val)
		case "enums":
			m.Enums, err = parseEnums(filename, e.val)
		case "object_types":
			m.ObjectTypes, err = parseObjectTypes(filename, e.val)
		case "instances":
			m.Instances, err = parseInstances(filename, e.val)
		case "hierarchy":
			h, err := parseHierarchy(filename, e.val)
			if err != nil {
				return nil, err
			}
			m.Hierarchy = h
		case "perspectives":
			ps, err := parsePerspectives(filename, e.val)
			if err != nil {
				return nil, err
			}
			m.Perspectives = ps
		default:
			err = perr(filename, e.keyLine, "unknown top-level key %q", e.key)
		}
		if err != nil {
			return nil, err
		}
	}
	if !sawModel {
		return nil, fmt.Errorf("%s: missing required `model:` block", filename)
	}
	return m, nil
}

func parseModelHeader(file string, n *yaml.Node, m *Model) error {
	if err := expectMapping(file, "model", n); err != nil {
		return err
	}
	for _, e := range mapEntries(n) {
		switch e.key {
		case "name":
			m.Name = e.val.Value
		case "namespace":
			m.Namespace = e.val.Value
		case "prefix":
			m.Prefix = e.val.Value
		case "version":
			m.Version = e.val.Value
		case "publication_date":
			m.PublicationDate = e.val.Value
		default:
			return perr(file, e.keyLine, "unknown model key %q", e.key)
		}
	}
	return nil
}

func parseImports(file string, n *yaml.Node) ([]Import, error) {
	if err := expectMapping(file, "imports", n); err != nil {
		return nil, err
	}
	var out []Import
	for _, e := range mapEntries(n) {
		im := Import{Pos: Pos{File: file, Line: e.keyLine, Col: e.keyCol}, Alias: e.key}
		switch e.val.Kind {
		case yaml.ScalarNode:
			im.URI = e.val.Value
		case yaml.MappingNode:
			for _, f := range mapEntries(e.val) {
				switch f.key {
				case "uri":
					im.URI = f.val.Value
				case "version":
					im.Version = f.val.Value
				default:
					return nil, perr(file, f.keyLine, "unknown import key %q in %q", f.key, e.key)
				}
			}
			if im.URI == "" {
				return nil, perr(file, e.keyLine, "import %q needs `uri:`", e.key)
			}
		default:
			return nil, perr(file, e.val.Line, "import %q must be a URI or {uri, version}", e.key)
		}
		out = append(out, im)
	}
	return out, nil
}

func parseEnums(file string, n *yaml.Node) ([]*Enum, error) {
	if err := expectMapping(file, "enums", n); err != nil {
		return nil, err
	}
	var out []*Enum
	for _, e := range mapEntries(n) {
		if err := expectMapping(file, fmt.Sprintf("enum %q", e.key), e.val); err != nil {
			return nil, err
		}
		en := &Enum{Pos: Pos{File: file, Line: e.keyLine, Col: e.keyCol}, Name: e.key}
		for _, f := range mapEntries(e.val) {
			switch f.key {
			case "doc":
				en.Doc = cleanDoc(f.val.Value)
			case "values":
				vals, err := parseEnumValues(file, f.val)
				if err != nil {
					return nil, err
				}
				en.Values = vals
			default:
				return nil, perr(file, f.keyLine, "unknown enum key %q in %q", f.key, e.key)
			}
		}
		out = append(out, en)
	}
	return out, nil
}

func parseEnumValues(file string, n *yaml.Node) ([]EnumValue, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, perr(file, n.Line, "enum values must be a list")
	}
	out := make([]EnumValue, 0, len(n.Content))
	for i, item := range n.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			out = append(out, EnumValue{Pos: posOf(file, item), Name: item.Value, Identifier: i})
		case yaml.MappingNode:
			if len(item.Content) != 2 {
				return nil, perr(file, item.Line, "enum value mapping must be a single `Name: id` pair")
			}
			k, val := item.Content[0], item.Content[1]
			id, err := strconv.Atoi(strings.TrimSpace(val.Value))
			if err != nil {
				return nil, perr(file, val.Line, "enum value id must be an integer, got %q", val.Value)
			}
			out = append(out, EnumValue{Pos: posOf(file, k), Name: k.Value, Identifier: id, Explicit: true})
		default:
			return nil, perr(file, item.Line, "enum value must be a name or `Name: id`")
		}
	}
	return out, nil
}

func parseObjectTypes(file string, n *yaml.Node) ([]*ObjectType, error) {
	if err := expectMapping(file, "object_types", n); err != nil {
		return nil, err
	}
	var out []*ObjectType
	for _, e := range mapEntries(n) {
		if err := expectMapping(file, fmt.Sprintf("object type %q", e.key), e.val); err != nil {
			return nil, err
		}
		ot := &ObjectType{Pos: Pos{File: file, Line: e.keyLine, Col: e.keyCol}, Name: e.key}
		for _, f := range mapEntries(e.val) {
			switch f.key {
			case "doc":
				ot.Doc = cleanDoc(f.val.Value)
			case "base":
				ot.Base = parseTypeRef(f.val.Value, posOf(file, f.val))
			case "abstract":
				b, err := strconv.ParseBool(strings.TrimSpace(f.val.Value))
				if err != nil {
					return nil, perr(file, f.val.Line, "abstract must be true or false, got %q", f.val.Value)
				}
				ot.Abstract = b
			case "members":
				ms, err := parseMembers(file, f.val)
				if err != nil {
					return nil, err
				}
				ot.Members = ms
			default:
				return nil, perr(file, f.keyLine, "unknown object type key %q in %q", f.key, e.key)
			}
		}
		out = append(out, ot)
	}
	return out, nil
}

func parseMembers(file string, n *yaml.Node) ([]*Member, error) {
	if err := expectMapping(file, "members", n); err != nil {
		return nil, err
	}
	var out []*Member
	for _, e := range mapEntries(n) {
		keyPos := Pos{File: file, Line: e.keyLine, Col: e.keyCol}
		mem, err := parseMember(file, e.key, keyPos, e.val)
		if err != nil {
			return nil, err
		}
		out = append(out, mem)
	}
	return out, nil
}

func parseMember(file, key string, keyPos Pos, val *yaml.Node) (*Member, error) {
	m := &Member{Pos: keyPos, Kind: KindVariable, Rule: RuleMandatory, Access: AccessRead}
	if base, suffix, ok := splitPlaceholder(key); ok {
		m.Name = base
		m.BrowseName = "<" + base + suffix + ">"
	} else {
		m.Name = key
	}
	if err := expectMapping(file, fmt.Sprintf("member %q", key), val); err != nil {
		return nil, err
	}
	for _, f := range mapEntries(val) {
		switch f.key {
		case "kind":
			m.Kind = Kind(f.val.Value)
		case "type":
			m.Type = parseTypeRef(f.val.Value, posOf(file, f.val))
		case "rule":
			m.Rule = Rule(f.val.Value)
		case "access":
			m.Access = Access(f.val.Value)
		case "unit":
			m.Unit = f.val.Value
		case "doc":
			m.Doc = cleanDoc(f.val.Value)
		case "children":
			ch, err := parseMembers(file, f.val)
			if err != nil {
				return nil, err
			}
			m.Children = ch
		case "in":
			args, err := parseArguments(file, f.val)
			if err != nil {
				return nil, err
			}
			m.In = args
		case "out":
			args, err := parseArguments(file, f.val)
			if err != nil {
				return nil, err
			}
			m.Out = args
		default:
			return nil, perr(file, f.keyLine, "unknown member key %q in %q", f.key, key)
		}
	}
	return m, nil
}

func parseArguments(file string, n *yaml.Node) ([]Argument, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, perr(file, n.Line, "arguments must be a list")
	}
	var out []Argument
	for _, item := range n.Content {
		if err := expectMapping(file, "argument", item); err != nil {
			return nil, fmt.Errorf("%w (want {name, type})", err)
		}
		a := Argument{Pos: posOf(file, item)}
		for _, f := range mapEntries(item) {
			switch f.key {
			case "name":
				a.Name = f.val.Value
			case "type":
				a.Type = parseTypeRef(f.val.Value, posOf(file, f.val))
			default:
				return nil, perr(file, f.keyLine, "unknown argument key %q", f.key)
			}
		}
		out = append(out, a)
	}
	return out, nil
}

func parseHierarchy(file string, n *yaml.Node) (Hierarchy, error) {
	if err := expectMapping(file, "hierarchy", n); err != nil {
		return Hierarchy{}, err
	}
	h := Hierarchy{Set: true}
	for _, f := range mapEntries(n) {
		switch f.key {
		case "allowLevelSkip":
			h.AllowLevelSkip = f.val.Value == "true"
		default:
			return Hierarchy{}, perr(file, f.keyLine, "unknown hierarchy key %q", f.key)
		}
	}
	return h, nil
}

func parseInstances(file string, n *yaml.Node) ([]*Instance, error) {
	if err := expectMapping(file, "instances", n); err != nil {
		return nil, err
	}
	var out []*Instance
	for _, e := range mapEntries(n) {
		if err := expectMapping(file, fmt.Sprintf("instance %q", e.key), e.val); err != nil {
			return nil, err
		}
		inst := &Instance{Pos: Pos{File: file, Line: e.keyLine, Col: e.keyCol}, Name: e.key}
		for _, f := range mapEntries(e.val) {
			switch f.key {
			case "type":
				inst.Type = parseTypeRef(f.val.Value, posOf(file, f.val))
			case "under":
				inst.Under = parseTypeRef(f.val.Value, posOf(file, f.val))
			case "level":
				inst.Level = f.val.Value
				inst.LevelPos = posOf(file, f.val)
			case "values":
				vals, err := parseInstanceValues(file, f.val)
				if err != nil {
					return nil, err
				}
				inst.Values = vals
			case "children":
				ch, err := parseInstanceChildren(file, f.val)
				if err != nil {
					return nil, err
				}
				inst.Children = ch
			default:
				return nil, perr(file, f.keyLine, "unknown instance key %q in %q", f.key, e.key)
			}
		}
		out = append(out, inst)
	}
	return out, nil
}

func parseInstanceValues(file string, n *yaml.Node) ([]InstanceValue, error) {
	if err := expectMapping(file, "instance values", n); err != nil {
		return nil, err
	}
	var out []InstanceValue
	for _, e := range mapEntries(n) {
		if e.val.Kind != yaml.ScalarNode {
			return nil, perr(file, e.val.Line, "instance value %q must be a scalar", e.key)
		}
		out = append(out, InstanceValue{
			Pos:    Pos{File: file, Line: e.keyLine, Col: e.keyCol},
			Member: e.key,
			Raw:    e.val.Value,
		})
	}
	return out, nil
}

func parseInstanceChildren(file string, n *yaml.Node) ([]InstanceChild, error) {
	if err := expectMapping(file, "instance children", n); err != nil {
		return nil, err
	}
	var out []InstanceChild
	for _, e := range mapEntries(n) {
		if err := expectMapping(file, fmt.Sprintf("instance child %q", e.key), e.val); err != nil {
			return nil, err
		}
		ch := InstanceChild{Pos: Pos{File: file, Line: e.keyLine, Col: e.keyCol}, Name: e.key}
		for _, f := range mapEntries(e.val) {
			switch f.key {
			case "of":
				ch.Of = parseTypeRef(f.val.Value, posOf(file, f.val))
			default:
				return nil, perr(file, f.keyLine, "unknown instance child key %q in %q", f.key, e.key)
			}
		}
		if ch.Of.IsZero() {
			return nil, perr(file, e.keyLine, "instance child %q needs `of: <placeholder>`", e.key)
		}
		out = append(out, ch)
	}
	return out, nil
}

func parsePerspectives(file string, n *yaml.Node) ([]*Perspective, error) {
	if err := expectMapping(file, "perspectives", n); err != nil {
		return nil, err
	}
	var out []*Perspective
	for _, e := range mapEntries(n) {
		if err := expectMapping(file, fmt.Sprintf("perspective %q", e.key), e.val); err != nil {
			return nil, err
		}
		p := &Perspective{Pos: Pos{File: file, Line: e.keyLine, Col: e.keyCol}, ID: e.key}
		for _, f := range mapEntries(e.val) {
			switch f.key {
			case "label":
				p.Label = f.val.Value
			case "membership":
				p.Membership = f.val.Value
			case "export":
				p.Export = f.val.Value == "true"
			case "nodes":
				nodes, err := parsePerspectiveNodes(file, f.val)
				if err != nil {
					return nil, err
				}
				p.Nodes = nodes
			default:
				return nil, perr(file, f.keyLine, "unknown perspective key %q in %q", f.key, e.key)
			}
		}
		out = append(out, p)
	}
	return out, nil
}

func parsePerspectiveNodes(file string, n *yaml.Node) ([]*PerspectiveNode, error) {
	if err := expectMapping(file, "perspective nodes", n); err != nil {
		return nil, err
	}
	var out []*PerspectiveNode
	for _, e := range mapEntries(n) {
		if err := expectMapping(file, fmt.Sprintf("perspective node %q", e.key), e.val); err != nil {
			return nil, err
		}
		nd := &PerspectiveNode{Pos: Pos{File: file, Line: e.keyLine, Col: e.keyCol}, ID: e.key}
		for _, f := range mapEntries(e.val) {
			switch f.key {
			case "label":
				nd.Label = f.val.Value
			case "children":
				nd.Children = scalarSeq(f.val)
			case "members":
				nd.Members = scalarSeq(f.val)
			default:
				return nil, perr(file, f.keyLine, "unknown perspective node key %q in %q", f.key, e.key)
			}
		}
		out = append(out, nd)
	}
	return out, nil
}

// scalarSeq reads a YAML flow/block sequence of scalars into a []string. A
// missing/non-sequence node yields nil (validation reports semantic problems).
func scalarSeq(n *yaml.Node) []string {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(n.Content))
	for _, c := range n.Content {
		out = append(out, c.Value)
	}
	return out
}

// --- helpers -----------------------------------------------------------------

type kvEntry struct {
	key     string
	keyLine int
	keyCol  int
	val     *yaml.Node
}

// mapEntries returns the key/value pairs of a mapping node in source order.
func mapEntries(n *yaml.Node) []kvEntry {
	out := make([]kvEntry, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		out = append(out, kvEntry{key: k.Value, keyLine: k.Line, keyCol: k.Column, val: v})
	}
	return out
}

// posOf builds a Pos from a yaml node.
func posOf(file string, n *yaml.Node) Pos {
	return Pos{File: file, Line: n.Line, Col: n.Column}
}

// expectMapping returns a positioned error if n is not a YAML mapping.
func expectMapping(file, what string, n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return perr(file, n.Line, "%s must be a mapping", what)
	}
	return nil
}

// parseTypeRef splits a written type reference into alias + name.
func parseTypeRef(s string, pos Pos) TypeRef {
	s = strings.TrimSpace(s)
	if alias, name, ok := strings.Cut(s, ":"); ok {
		return TypeRef{Pos: pos, Alias: alias, Name: name, Raw: s}
	}
	return TypeRef{Pos: pos, Name: s, Raw: s}
}

var placeholderRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)<([A-Za-z_][A-Za-z0-9_]*)>$`)

// splitPlaceholder recognises a `Name<Suffix>` member key.
func splitPlaceholder(key string) (base, suffix string, ok bool) {
	m := placeholderRe.FindStringSubmatch(key)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// cleanDoc collapses whitespace (incl. newlines) in doc text to single spaces, so
// descriptions always emit on one line — keeping the deterministic, line-oriented
// output invariant intact even for multi-line YAML scalars.
func cleanDoc(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func perr(file string, line int, format string, args ...any) error {
	return fmt.Errorf("%s:%d: "+format, append([]any{file, line}, args...)...)
}
