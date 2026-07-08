package core

import (
	"fmt"
	"strings"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

// WriteOpts controls how an additive patch op (AddImport/AddType/AddInstance)
// stores its result.
type WriteOpts struct {
	// Force stores the result even if it has error-severity validation
	// diagnostics. Ignored on a dry-run.
	Force bool
	// DryRun validates the result in full draft context (accurate diagnostics
	// — an isolated fragment check would emit false unknown-reference errors)
	// but stores nothing. Always returns the diagnostics; never refuses.
	DryRun bool
}

// AddImport appends an import (alias -> namespace URI) to a draft file. Additive:
// a duplicate alias is ErrConflict. Strict by default: refuses (stores nothing)
// if the resulting file has any error-severity validation diagnostic, unless
// opts.Force is true. opts.DryRun validates without storing.
func (s *Service) AddImport(id, file, alias, namespace string, opts WriteOpts) (dto.DraftWriteResponse, error) {
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
	return s.storeMutatedDraft(d, path, m, opts)
}

// storeMutatedDraft re-emits the mutated model (faithful round-trip), stores it
// back into the draft under path, and returns the file list + this file's
// validation diagnostics.
//
// Strict by default: the mutated file is validated BEFORE anything is written.
// If it carries any error-severity diagnostic (pre-existing in the file or
// introduced by this edit) and opts.Force is false, the write is refused
// entirely — storeMutatedDraft returns a *ValidationError and the draft is left
// untouched. This closes the gap where an error-severity issue (unknown unit/
// base/type/under, ...) was only caught later at propose_pr. opts.Force is the
// escape hatch for incremental/forward-reference construction (e.g. adding an
// instance before the sibling it nests under exists yet).
//
// opts.DryRun short-circuits before the strict gate and before any store
// write: it always returns the computed diagnostics (Stored: false), never
// refuses, and Force is irrelevant. This lets a caller preview a body's
// diagnostics in full draft context without mutating anything.
//
// The store write goes through s.store.Update so it is lock-guarded (the
// stdio app, the mounted /mcp server, and the web editor's PUT /files all
// share one *Store) and so UpdatedAt is bumped, keeping the draft alive
// against the TTL sweeper for agent sessions that only read + add_*.
//
// Diagnostics are computed against a re-parse of the formatted output under the
// real path rather than against m directly: m's freshly-spliced nodes carry
// Pos from parseFragment's throwaway "_fragment.yaml" envelope, and reporting
// diagnostics at that fake path would be useless to the caller.
func (s *Service) storeMutatedDraft(d *Draft, path string, m *dsl.Model, opts WriteOpts) (dto.DraftWriteResponse, error) {
	out, err := dsl.Format(m)
	if err != nil {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: %s", ErrParse, err.Error())
	}

	// Validate what WOULD be stored, under the real path, before ever writing it,
	// so a strict refusal never touches the store. Format's output should always
	// re-parse cleanly; fall back to m defensively so a diagnostic is never
	// silently dropped.
	vm, perr := dsl.Parse(path, out)
	if perr != nil {
		vm = m
	}
	if c, err := s.Catalog(); err == nil {
		vm.Catalog = c
	}
	dslDiags := dsl.Validate(vm)

	if opts.DryRun {
		return dto.DraftWriteResponse{
			Files:       SortedKeys(d.Files), // unchanged file set (nothing stored)
			Diagnostics: map[string][]dto.Diagnostic{path: dto.FromDiagnostics(dslDiags)},
			Stored:      false,
		}, nil
	}

	var blocking []dto.Diagnostic
	for _, dg := range dslDiags {
		if dg.Severity == dsl.SeverityError {
			blocking = append(blocking, dto.FromDiagnostic(dg))
		}
	}
	if !opts.Force && len(blocking) > 0 {
		return dto.DraftWriteResponse{}, &ValidationError{Blocking: blocking}
	}

	var files []string
	if _, ok := s.store.Update(d.ID, func(dr *Draft) {
		dr.Files[path] = out
		files = SortedKeys(dr.Files)
	}); !ok {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: draft not found", ErrNotFound)
	}

	return dto.DraftWriteResponse{
		Files:       files,
		Diagnostics: map[string][]dto.Diagnostic{path: dto.FromDiagnostics(dslDiags)},
		Stored:      true,
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
// a duplicate type name is ErrConflict; an unparseable body is ErrParse. Strict
// by default: refuses (stores nothing) if the resulting file has any
// error-severity validation diagnostic, unless opts.Force is true. opts.DryRun
// validates without storing.
func (s *Service) AddType(id, file, name, body string, opts WriteOpts) (dto.DraftWriteResponse, error) {
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
	return s.storeMutatedDraft(d, path, m, opts)
}

// AddInstance splices a new Instance (name + DSL body carrying type/under/level/
// values/children) into a draft file. Additive: a duplicate instance name is
// ErrConflict; an unparseable body is ErrParse. Strict by default: refuses
// (stores nothing) if the resulting file has any error-severity validation
// diagnostic, unless opts.Force is true. opts.DryRun validates without storing.
func (s *Service) AddInstance(id, file, name, body string, opts WriteOpts) (dto.DraftWriteResponse, error) {
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
	return s.storeMutatedDraft(d, path, m, opts)
}

// RemoveType deletes an ObjectType by name from a draft file. ErrNotFound if the
// type is absent. The removal is validated+stored via storeMutatedDraft, so it
// honors the strict gate (a still-referenced type — e.g. an instance whose
// type: points at it — is CodeUnknownType, error-severity, refused unless
// opts.Force) and opts.DryRun (preview without storing).
func (s *Service) RemoveType(id, file, name string, opts WriteOpts) (dto.DraftWriteResponse, error) {
	d, path, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	idx := -1
	for i, ot := range m.ObjectTypes {
		if ot.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: type %q not present", ErrNotFound, name)
	}
	m.ObjectTypes = append(m.ObjectTypes[:idx], m.ObjectTypes[idx+1:]...)
	return s.storeMutatedDraft(d, path, m, opts)
}

// RemoveInstance deletes an Instance by name from a draft file. ErrNotFound if
// the instance is absent. The removal is validated+stored via
// storeMutatedDraft, so it honors the strict gate (a still-referenced
// instance — e.g. one nested under it via under: — is error-severity, refused
// unless opts.Force) and opts.DryRun (preview without storing).
func (s *Service) RemoveInstance(id, file, name string, opts WriteOpts) (dto.DraftWriteResponse, error) {
	d, path, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	idx := -1
	for i, inst := range m.Instances {
		if inst.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: instance %q not present", ErrNotFound, name)
	}
	m.Instances = append(m.Instances[:idx], m.Instances[idx+1:]...)
	return s.storeMutatedDraft(d, path, m, opts)
}

// RemoveImport deletes an import (by alias) from a draft file. ErrNotFound if
// the alias is absent. The removal is validated+stored via storeMutatedDraft,
// so it honors the strict gate (a still-referenced alias — e.g. in a base:/
// under:/member type: — is CodeUnknownImportAlias, error-severity, refused
// unless opts.Force) and opts.DryRun (preview without storing).
func (s *Service) RemoveImport(id, file, alias string, opts WriteOpts) (dto.DraftWriteResponse, error) {
	d, path, m, err := s.draftParsedModel(id, file)
	if err != nil {
		return dto.DraftWriteResponse{}, err
	}
	idx := -1
	for i, im := range m.Imports {
		if im.Alias == alias {
			idx = i
			break
		}
	}
	if idx == -1 {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: import %q not present", ErrNotFound, alias)
	}
	m.Imports = append(m.Imports[:idx], m.Imports[idx+1:]...)
	return s.storeMutatedDraft(d, path, m, opts)
}
