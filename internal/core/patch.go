package core

import (
	"fmt"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

// AddImport appends an import (alias -> namespace URI) to a draft file. Additive:
// a duplicate alias is ErrConflict.
func (s *Service) AddImport(id, file, alias, namespace string) (dto.DraftWriteResponse, error) {
	d, path, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	for _, im := range m.Imports {
		if im.Alias == alias {
			return dto.DraftWriteResponse{}, fmt.Errorf("%w: import %q already present", ErrConflict, alias)
		}
	}
	m.Imports = append(m.Imports, dsl.Import{Alias: alias, URI: namespace})
	return s.storeMutatedDraft(d, path, m)
}

// storeMutatedDraft re-emits the mutated model (faithful round-trip), stores it
// back into the draft under path, and returns the file list + this file's
// validation diagnostics.
//
// The store write goes through s.store.Update so it is lock-guarded (the
// stdio app, the mounted /mcp server, and the web editor's PUT /files all
// share one *Store) and so UpdatedAt is bumped, keeping the draft alive
// against the TTL sweeper for agent sessions that only read + add_*.
//
// Diagnostics are computed against a re-parse of the stored output under the
// real path rather than against m directly: m's freshly-spliced nodes carry
// Pos from parseFragment's throwaway "_fragment.yaml" envelope, and reporting
// diagnostics at that fake path would be useless to the caller.
func (s *Service) storeMutatedDraft(d *Draft, path string, m *dsl.Model) (dto.DraftWriteResponse, error) {
	out, err := dsl.Format(m)
	if err != nil {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: %s", ErrParse, err.Error())
	}

	var files []string
	if _, ok := s.store.Update(d.ID, func(dr *Draft) {
		dr.Files[path] = out
		files = SortedKeys(dr.Files)
	}); !ok {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: draft not found", ErrNotFound)
	}

	// Validate what was actually stored, under the real path, so any echoed
	// diagnostics point at real positions. Format's output should always
	// re-parse cleanly; fall back to m defensively so a diagnostic is never
	// silently dropped.
	vm, perr := dsl.Parse(path, out)
	if perr != nil {
		vm = m
	}
	if c, err := s.Catalog(); err == nil {
		vm.Catalog = c
	}
	diags := dto.FromDiagnostics(dsl.Validate(vm))
	return dto.DraftWriteResponse{
		Files:       files,
		Diagnostics: map[string][]dto.Diagnostic{path: diags},
	}, nil
}

// indentLines prefixes every non-empty line of s with n spaces.
func indentLines(s string, n int) string {
	pad := strings.Repeat(" ", n)
	var b strings.Builder
	for _, ln := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if ln == "" {
			b.WriteByte('\n')
			continue
		}
		b.WriteString(pad + ln + "\n")
	}
	return b.String()
}

// parseFragment parses a single named node body (a type or instance) inside a
// throwaway minimal model, reusing the real parser so defaults and structure
// come from one place. section is "object_types" or "instances".
func parseFragment(section, name, body string) (*dsl.Model, error) {
	const envelope = "model:\n  name: _frag\n  namespace: urn:frag\n  version: 0.0.0\n  publication_date: 2000-01-01\n"
	src := envelope + section + ":\n  " + name + ":\n" + indentLines(body, 4)
	m, err := dsl.Parse("_fragment.yaml", []byte(src))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParse, err.Error())
	}
	return m, nil
}

// AddType splices a new ObjectType (name + DSL body) into a draft file. Additive:
// a duplicate type name is ErrConflict; an unparseable body is ErrParse.
func (s *Service) AddType(id, file, name, body string) (dto.DraftWriteResponse, error) {
	d, path, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	for _, ot := range m.ObjectTypes {
		if ot.Name == name {
			return dto.DraftWriteResponse{}, fmt.Errorf("%w: type %q already present", ErrConflict, name)
		}
	}
	frag, err := parseFragment("object_types", name, body)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	if len(frag.ObjectTypes) != 1 {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: body did not define exactly one type", ErrParse)
	}
	frag.ObjectTypes[0].Name = name
	m.ObjectTypes = append(m.ObjectTypes, frag.ObjectTypes[0])
	return s.storeMutatedDraft(d, path, m)
}

// AddInstance splices a new Instance (name + DSL body carrying type/under/level/
// values/children) into a draft file. Additive: a duplicate instance name is
// ErrConflict; an unparseable body is ErrParse.
func (s *Service) AddInstance(id, file, name, body string) (dto.DraftWriteResponse, error) {
	d, path, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	for _, inst := range m.Instances {
		if inst.Name == name {
			return dto.DraftWriteResponse{}, fmt.Errorf("%w: instance %q already present", ErrConflict, name)
		}
	}
	frag, err := parseFragment("instances", name, body)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	if len(frag.Instances) != 1 {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: body did not define exactly one instance", ErrParse)
	}
	frag.Instances[0].Name = name
	m.Instances = append(m.Instances, frag.Instances[0])
	return s.storeMutatedDraft(d, path, m)
}
