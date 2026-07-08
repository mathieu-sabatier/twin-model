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
	return s.mutateDraftFile(id, file, opts, func(m *dsl.Model) error {
		for _, im := range m.Imports {
			if im.Alias == alias {
				return fmt.Errorf("%w: import %q already present", ErrConflict, alias)
			}
		}
		m.Imports = append(m.Imports, dsl.Import{Alias: alias, URI: namespace})
		return nil
	})
}

// mutateDraftFile is the atomic read-modify-write behind every patch op. It
// resolves file, parses the current stored bytes, runs apply (which does the
// op-specific conflict check + splice/removal against that current model),
// re-emits the mutated model (faithful round-trip), validates it, and stores it
// back — ALL inside one s.store.Mutate critical section. Running the whole cycle
// under one lock is what makes concurrent edits to the same draft safe: two
// concurrent add_* calls can no longer both read the same base and blindly
// overwrite each other (the pre-fix lost-update hazard).
//
// Strict by default: the mutated file is validated BEFORE anything is written.
// If it carries any error-severity diagnostic (pre-existing in the file or
// introduced by this edit) and opts.Force is false, the write is refused
// entirely — mutateDraftFile returns a *ValidationError and the draft is left
// untouched. This closes the gap where an error-severity issue (unknown unit/
// base/type/under, ...) was only caught later at propose_pr. opts.Force is the
// escape hatch for incremental/forward-reference construction (e.g. adding an
// instance before the sibling it nests under exists yet).
//
// opts.DryRun short-circuits before the strict gate and before any store write:
// it always returns the computed diagnostics (Stored: false), never refuses,
// Force is irrelevant, and it does not bump UpdatedAt (it stores nothing).
//
// On a real store, the write bumps UpdatedAt (via Mutate), keeping the draft
// alive against the TTL sweeper for agent sessions that only read + add_*.
//
// Diagnostics are computed against a re-parse of the formatted output under the
// real path rather than against m directly: m's freshly-spliced nodes carry Pos
// from parseFragment's throwaway "_fragment.yaml" envelope, and reporting
// diagnostics at that fake path would be useless to the caller.
func (s *Service) mutateDraftFile(id, file string, opts WriteOpts, apply func(*dsl.Model) error) (dto.DraftWriteResponse, error) {
	// Catalog() is loaded off-lock (sync.Once, cached) so we never hold the store
	// mutex across the one-time companion-spec LoadAll.
	cat, catErr := s.Catalog()

	var resp dto.DraftWriteResponse
	found, opErr := s.store.Mutate(id, func(d *Draft) (bool, error) {
		path, data, ok := selectFile(d.Files, file)
		if !ok {
			return false, fmt.Errorf("%w: file not found in draft", ErrNotFound)
		}
		m, perr := dsl.Parse(path, data)
		if perr != nil {
			return false, fmt.Errorf("%w: parse: %s", ErrParse, perr.Error())
		}
		if err := apply(m); err != nil {
			return false, err
		}
		out, err := dsl.Format(m)
		if err != nil {
			return false, fmt.Errorf("%w: %s", ErrParse, err.Error())
		}

		// Validate what WOULD be stored, under the real path. Format's output
		// should always re-parse cleanly; fall back to m defensively so a
		// diagnostic is never silently dropped.
		vm, verr := dsl.Parse(path, out)
		if verr != nil {
			vm = m
		}
		if catErr == nil {
			vm.Catalog = cat
		}
		dslDiags := dsl.Validate(vm)

		if opts.DryRun {
			resp = dto.DraftWriteResponse{
				Files:       SortedKeys(d.Files), // unchanged file set (nothing stored)
				Diagnostics: map[string][]dto.Diagnostic{path: dto.FromDiagnostics(dslDiags)},
				Stored:      false,
			}
			return false, nil // no store, no UpdatedAt bump
		}

		var blocking []dto.Diagnostic
		for _, dg := range dslDiags {
			if dg.Severity == dsl.SeverityError {
				blocking = append(blocking, dto.FromDiagnostic(dg))
			}
		}
		if !opts.Force && len(blocking) > 0 {
			return false, &ValidationError{Blocking: blocking}
		}

		d.Files[path] = out
		resp = dto.DraftWriteResponse{
			Files:       SortedKeys(d.Files),
			Diagnostics: map[string][]dto.Diagnostic{path: dto.FromDiagnostics(dslDiags)},
			Stored:      true,
		}
		return true, nil
	})
	if !found {
		return dto.DraftWriteResponse{}, fmt.Errorf("%w: draft not found", ErrNotFound)
	}
	if opErr != nil {
		return dto.DraftWriteResponse{}, opErr
	}
	return resp, nil
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
		b.WriteString(pad)
		b.WriteString(ln)
		b.WriteByte('\n')
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
	return s.mutateDraftFile(id, file, opts, func(m *dsl.Model) error {
		for _, ot := range m.ObjectTypes {
			if ot.Name == name {
				return fmt.Errorf("%w: type %q already present", ErrConflict, name)
			}
		}
		frag, err := parseFragment("object_types", name, body)
		if err != nil {
			return err
		}
		if len(frag.ObjectTypes) != 1 {
			return fmt.Errorf("%w: body did not define exactly one type", ErrParse)
		}
		frag.ObjectTypes[0].Name = name
		m.ObjectTypes = append(m.ObjectTypes, frag.ObjectTypes[0])
		return nil
	})
}

// AddInstance splices a new Instance (name + DSL body carrying type/under/level/
// values/children) into a draft file. Additive: a duplicate instance name is
// ErrConflict; an unparseable body is ErrParse. Strict by default: refuses
// (stores nothing) if the resulting file has any error-severity validation
// diagnostic, unless opts.Force is true. opts.DryRun validates without storing.
func (s *Service) AddInstance(id, file, name, body string, opts WriteOpts) (dto.DraftWriteResponse, error) {
	return s.mutateDraftFile(id, file, opts, func(m *dsl.Model) error {
		for _, inst := range m.Instances {
			if inst.Name == name {
				return fmt.Errorf("%w: instance %q already present", ErrConflict, name)
			}
		}
		frag, err := parseFragment("instances", name, body)
		if err != nil {
			return err
		}
		if len(frag.Instances) != 1 {
			return fmt.Errorf("%w: body did not define exactly one instance", ErrParse)
		}
		frag.Instances[0].Name = name
		m.Instances = append(m.Instances, frag.Instances[0])
		return nil
	})
}

// RemoveType deletes an ObjectType by name from a draft file. ErrNotFound if the
// type is absent. The removal is validated+stored via mutateDraftFile, so it
// honors the strict gate (a still-referenced type — e.g. an instance whose
// type: points at it — is CodeUnknownType, error-severity, refused unless
// opts.Force) and opts.DryRun (preview without storing).
func (s *Service) RemoveType(id, file, name string, opts WriteOpts) (dto.DraftWriteResponse, error) {
	return s.mutateDraftFile(id, file, opts, func(m *dsl.Model) error {
		idx := -1
		for i, ot := range m.ObjectTypes {
			if ot.Name == name {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("%w: type %q not present", ErrNotFound, name)
		}
		m.ObjectTypes = append(m.ObjectTypes[:idx], m.ObjectTypes[idx+1:]...)
		return nil
	})
}

// RemoveInstance deletes an Instance by name from a draft file. ErrNotFound if
// the instance is absent. The removal is validated+stored via mutateDraftFile,
// so it honors the strict gate (a still-referenced instance — e.g. one nested
// under it via under: — is error-severity, refused unless opts.Force) and
// opts.DryRun (preview without storing).
func (s *Service) RemoveInstance(id, file, name string, opts WriteOpts) (dto.DraftWriteResponse, error) {
	return s.mutateDraftFile(id, file, opts, func(m *dsl.Model) error {
		idx := -1
		for i, inst := range m.Instances {
			if inst.Name == name {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("%w: instance %q not present", ErrNotFound, name)
		}
		m.Instances = append(m.Instances[:idx], m.Instances[idx+1:]...)
		return nil
	})
}

// RemoveImport deletes an import (by alias) from a draft file. ErrNotFound if
// the alias is absent. The removal is validated+stored via mutateDraftFile, so
// it honors the strict gate (a still-referenced alias — e.g. in a base:/
// under:/member type: — is CodeUnknownImportAlias, error-severity, refused
// unless opts.Force) and opts.DryRun (preview without storing).
func (s *Service) RemoveImport(id, file, alias string, opts WriteOpts) (dto.DraftWriteResponse, error) {
	return s.mutateDraftFile(id, file, opts, func(m *dsl.Model) error {
		idx := -1
		for i, im := range m.Imports {
			if im.Alias == alias {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("%w: import %q not present", ErrNotFound, alias)
		}
		m.Imports = append(m.Imports[:idx], m.Imports[idx+1:]...)
		return nil
	})
}
