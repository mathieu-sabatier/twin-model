package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func seedDraft(t *testing.T, s *Service, src string) *Draft {
	t.Helper()
	return s.Store().Create("main", map[string][]byte{"demo.yaml": []byte(src)})
}

// baseSrc's namespace ("urn:x") deliberately lacks the required trailing slash
// (see CodeNamespaceSlash in internal/dsl/validate.go), so it is already
// error-red on its own, independent of anything an add_* call splices in. Under
// strict-by-default writes this means every "happy path" test below that seeds
// from baseSrc must now pass force:true to storeMutatedDraft's whole-file gate
// — not because the type/instance/import being added is itself invalid, but
// because the shared fixture is. Tests whose point is the splice mechanics
// (not strictness) are annotated with why force:true is needed; tests that
// return before ever reaching storeMutatedDraft (conflict/parse-error) pass
// force:false since the gate is never evaluated.
const baseSrc = "model:\n  name: Demo\n  namespace: urn:x\n  version: 1.0.0\n  publication_date: 2026-01-01\n"

// cleanBaseSrc is baseSrc with a spec-compliant namespace, used by the new
// strict-refusal/force tests below so the only error-severity diagnostic in
// play is the one each test is actually about.
const cleanBaseSrc = "model:\n  name: Demo\n  namespace: urn:x/\n  version: 1.0.0\n  publication_date: 2026-01-01\n"

// mixerErrorBody is a type body that parses fine but carries exactly one
// error-severity validation diagnostic: CodeUnknownUnit ("unit: notaunit" is not
// a recognized engineering-unit symbol per internal/dsl/units.go's LookupUnit).
// Double is a numeric type (isNumericType), so CodeUnitOnNonNumeric does not
// also fire — isolating the single blocker the strict-refusal tests assert on.
// Verified via internal/dsl.Validate against cleanBaseSrc before relying on it.
const mixerErrorBody = "doc: A mixer\nbase: OpcUa:BaseObjectType\nmembers:\n  Weight: { type: Double, unit: notaunit }\n"

// cleanWidgetBody is mixerErrorBody's error-clean twin: same shape (base +
// one numeric member with a unit), but "kg" is a real engineering-unit symbol
// (see internal/dsl/units.go's LookupUnit), so it carries no error-severity
// validation diagnostic against cleanBaseSrc+the OpcUa import.
const cleanWidgetBody = "doc: A widget\nbase: OpcUa:BaseObjectType\nmembers:\n  Weight: { type: Double, unit: kg }\n"

func TestAddImport_AppendsAndCanonicalizes(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc)
	// force:true — baseSrc alone is error-red (missing namespace trailing
	// slash); this test is about import splicing, not strictness.
	resp, err := s.AddImport(d.ID, "demo.yaml", "DI", "http://opcfoundation.org/UA/DI/", WriteOpts{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("files = %v", resp.Files)
	}
	// dsl.Format's scalar() quotes any value containing ':' (see needsQuote in
	// internal/dsl/format.go), so a URI import comes back double-quoted.
	if !strings.Contains(string(d.Files["demo.yaml"]), `DI: "http://opcfoundation.org/UA/DI/"`) {
		t.Fatalf("import not stored:\n%s", d.Files["demo.yaml"])
	}
}

func TestAddImport_DuplicateAlias_IsConflict(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"imports:\n  DI: http://opcfoundation.org/UA/DI/\n")
	// force:false — the duplicate-alias conflict is returned before the strict
	// gate is ever reached, so its value doesn't matter here.
	_, err := s.AddImport(d.ID, "demo.yaml", "DI", "http://x/", WriteOpts{})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

// TestAddImport_BumpsUpdatedAt guards against storeMutatedDraft writing to the
// draft directly instead of routing through store.Update: a direct write races
// concurrent writers on the shared Files map and never refreshes UpdatedAt,
// letting the TTL sweeper evict a draft mid agent-session.
func TestAddImport_BumpsUpdatedAt(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc)
	before := d.UpdatedAt
	s.Store().now = func() time.Time { return before.Add(time.Minute) }
	// force:true — baseSrc alone is error-red (missing namespace trailing
	// slash); this test is about the UpdatedAt/store.Update routing, not
	// strictness.
	if _, err := s.AddImport(d.ID, "demo.yaml", "DI", "http://opcfoundation.org/UA/DI/", WriteOpts{Force: true}); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Store().Get(d.ID)
	if !ok {
		t.Fatal("draft went missing")
	}
	if !got.UpdatedAt.After(before) {
		t.Errorf("UpdatedAt = %v, want strictly after %v (storeMutatedDraft must route through store.Update)", got.UpdatedAt, before)
	}
}

func TestAddImport_UnknownDraft_IsNotFound(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	if _, err := s.AddImport("nope", "demo.yaml", "DI", "http://x/", WriteOpts{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestAddType_SplicesTypeAndPreservesFile(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"imports:\n  OpcUa: http://opcfoundation.org/UA/\n")
	body := "doc: A widget\nbase: OpcUa:BaseObjectType\nmembers:\n  Weight: { type: OpcUa:Double, unit: kg }\n"
	// force:true — baseSrc alone is error-red (missing namespace trailing
	// slash), and OpcUa:Double is not locally numeric per isNumericType (it
	// only recognizes unprefixed builtin names), so "unit: kg" here also trips
	// CodeUnitOnNonNumeric. This test is about splice/round-trip mechanics, not
	// strictness.
	resp, err := s.AddType(d.ID, "demo.yaml", "WidgetType", body, WriteOpts{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	stored := string(d.Files["demo.yaml"])
	if !strings.Contains(stored, "WidgetType:") || !strings.Contains(stored, "Weight") {
		t.Fatalf("type not spliced:\n%s", stored)
	}
	// dsl.Format's scalar() quotes any value containing ':' (see needsQuote in
	// internal/dsl/format.go and TestAddImport_AppendsAndCanonicalizes above), so
	// the pre-existing URI import comes back double-quoted after the round-trip.
	if !strings.Contains(stored, `OpcUa: "http://opcfoundation.org/UA/"`) {
		t.Fatalf("existing content dropped:\n%s", stored)
	}
	if _, ok := resp.Diagnostics["demo.yaml"]; !ok {
		t.Fatal("expected per-file diagnostics key")
	}
}

func TestAddType_DuplicateName_IsConflict(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n")
	// force:false — the duplicate-name conflict is returned before the strict
	// gate is ever reached.
	_, err := s.AddType(d.ID, "demo.yaml", "WidgetType", "base: OpcUa:BaseObjectType\n", WriteOpts{})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestAddType_UnparseableBody_IsParseError(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc)
	// force:false — parseFragment fails before the strict gate is ever reached.
	_, err := s.AddType(d.ID, "demo.yaml", "WidgetType", "members: [this is not a mapping", WriteOpts{})
	if !errors.Is(err, ErrParse) {
		t.Fatalf("err = %v, want ErrParse", err)
	}
}

func TestAddInstance_SplicesInstance(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n")
	body := "type: WidgetType\nunder: Cell1\nlevel: Unit\n"
	// force:true — baseSrc is error-red (missing namespace trailing slash), and
	// this fixture also references "under: Cell1", an instance that is never
	// declared (CodeUnknownUnder) and sets a level the type has no
	// EquipmentLevel member for (CodeLevelOnUnsupportedType). This test is about
	// splice mechanics, not strictness/forward-references.
	if _, err := s.AddInstance(d.ID, "demo.yaml", "Widget1", body, WriteOpts{Force: true}); err != nil {
		t.Fatal(err)
	}
	stored := string(d.Files["demo.yaml"])
	if !strings.Contains(stored, "Widget1:") || !strings.Contains(stored, "type: WidgetType") {
		t.Fatalf("instance not spliced:\n%s", stored)
	}
}

func TestAddInstance_DuplicateName_IsConflict(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, baseSrc+"instances:\n  Widget1:\n    type: WidgetType\n")
	// force:false — the duplicate-name conflict is returned before the strict
	// gate is ever reached.
	_, err := s.AddInstance(d.ID, "demo.yaml", "Widget1", "type: WidgetType\n", WriteOpts{})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

// TestAddType_ErrorSeverity_RefusesWithoutForce is the core of Task 8: adding a
// type whose body carries an error-severity validation diagnostic (unknown
// unit) must refuse — as a *ValidationError with the blocker(s) attached — and
// must not mutate the draft at all.
func TestAddType_ErrorSeverity_RefusesWithoutForce(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc+"imports:\n  OpcUa: http://opcfoundation.org/UA/\n")
	before := string(d.Files["demo.yaml"])

	_, err := s.AddType(d.ID, "demo.yaml", "MixerType", mixerErrorBody, WriteOpts{})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v (%T), want *ValidationError", err, err)
	}
	if len(ve.Blocking) != 1 || ve.Blocking[0].Code != "unknown-unit" {
		t.Fatalf("Blocking = %+v, want exactly one unknown-unit diagnostic", ve.Blocking)
	}
	if string(d.Files["demo.yaml"]) != before {
		t.Fatalf("draft was mutated despite refusal:\n%s", d.Files["demo.yaml"])
	}
}

// TestAddType_ErrorSeverity_StoresWithForce is the force:true escape hatch for
// the same input: it must store (no error) and the node must be present.
func TestAddType_ErrorSeverity_StoresWithForce(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc+"imports:\n  OpcUa: http://opcfoundation.org/UA/\n")

	resp, err := s.AddType(d.ID, "demo.yaml", "MixerType", mixerErrorBody, WriteOpts{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("type not stored:\n%s", d.Files["demo.yaml"])
	}
	diags, ok := resp.Diagnostics["demo.yaml"]
	if !ok || len(diags) != 1 || diags[0].Code != "unknown-unit" {
		t.Fatalf("expected the unknown-unit diagnostic echoed on forced success, got %+v", resp.Diagnostics)
	}
	if !resp.Stored {
		t.Fatal("Stored = false, want true on a forced real write")
	}
}

// TestAddType_CleanBody_StoresWithoutForce is the folded Task 8 coverage test:
// it exercises the !Force && no-blocking → store branch of storeMutatedDraft,
// which every other test so far has skipped by either forcing past a
// pre-existing error (baseSrc) or returning before the strict gate
// (conflict/parse-error). cleanBaseSrc+cleanWidgetBody has no error-severity
// diagnostic at all, so the write must succeed with force:false.
func TestAddType_CleanBody_StoresWithoutForce(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc+"imports:\n  OpcUa: http://opcfoundation.org/UA/\n")

	resp, err := s.AddType(d.ID, "demo.yaml", "WidgetType", cleanWidgetBody, WriteOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Stored {
		t.Fatal("Stored = false, want true on a clean, unforced write")
	}
	if !strings.Contains(string(d.Files["demo.yaml"]), "WidgetType:") {
		t.Fatalf("type not stored:\n%s", d.Files["demo.yaml"])
	}
	for _, dg := range resp.Diagnostics["demo.yaml"] {
		if dg.Severity == "error" {
			t.Fatalf("unexpected error-severity diagnostic on a clean body: %+v", dg)
		}
	}
}

// TestAddType_DryRun_ValidatesWithoutStoring: a dry-run add_type with a body
// that carries an error-severity diagnostic must still return that diagnostic
// (dry-run never refuses, Force is irrelevant) but must leave the draft
// completely untouched — no ErrConflict re-check bypass, no store.Update.
func TestAddType_DryRun_ValidatesWithoutStoring(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc+"imports:\n  OpcUa: http://opcfoundation.org/UA/\n")
	before := string(d.Files["demo.yaml"])

	resp, err := s.AddType(d.ID, "demo.yaml", "MixerType", mixerErrorBody, WriteOpts{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stored {
		t.Fatal("Stored = true, want false on a dry-run")
	}
	diags, ok := resp.Diagnostics["demo.yaml"]
	if !ok || len(diags) != 1 || diags[0].Code != "unknown-unit" {
		t.Fatalf("expected the unknown-unit diagnostic previewed on dry-run, got %+v", resp.Diagnostics)
	}
	if string(d.Files["demo.yaml"]) != before {
		t.Fatalf("draft was mutated by a dry-run:\n%s", d.Files["demo.yaml"])
	}
}

// TestAddType_DryRun_CleanBody: a dry-run add_type with a clean body returns
// no error-severity diagnostics and, as with any dry-run, stores nothing.
func TestAddType_DryRun_CleanBody(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc+"imports:\n  OpcUa: http://opcfoundation.org/UA/\n")
	before := string(d.Files["demo.yaml"])

	resp, err := s.AddType(d.ID, "demo.yaml", "WidgetType", cleanWidgetBody, WriteOpts{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stored {
		t.Fatal("Stored = true, want false on a dry-run")
	}
	for _, dg := range resp.Diagnostics["demo.yaml"] {
		if dg.Severity == "error" {
			t.Fatalf("unexpected error-severity diagnostic on a clean dry-run body: %+v", dg)
		}
	}
	if string(d.Files["demo.yaml"]) != before {
		t.Fatalf("draft was mutated by a dry-run:\n%s", d.Files["demo.yaml"])
	}
}

// TestRemoveType_RemovesUnreferenced is the basic happy path for RemoveType:
// removing a type nothing else references leaves the file clean (no
// error-severity diagnostic introduced), so it stores without force.
func TestRemoveType_RemovesUnreferenced(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	src := cleanBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
		"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n"
	d := seedDraft(t, s, src)

	resp, err := s.RemoveType(d.ID, "demo.yaml", "WidgetType", WriteOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Stored {
		t.Fatal("Stored = false, want true on a clean, unforced removal")
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "WidgetType") {
		t.Fatalf("type still present after removal:\n%s", d.Files["demo.yaml"])
	}
}

// TestRemoveType_Unknown_IsNotFound: removing an absent type is ErrNotFound.
func TestRemoveType_Unknown_IsNotFound(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc)
	_, err := s.RemoveType(d.ID, "demo.yaml", "NopeType", WriteOpts{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// mixerTypeAndInstanceSrc seeds a type (MixerType) and an instance (Mixer1)
// whose `type:` field references it. Removing MixerType leaves Mixer1's type
// reference dangling: dsl.ResolveType classifies the unprefixed name as
// RefUnknownName, and checkTypeRef (called from checkInstances for
// inst.Type, internal/dsl/validate.go) emits CodeUnknownType
// ("unknown-type") at error severity — verified against
// internal/dsl/validate_test.go's own unknown-type fixture and by running
// the removal below. No `under:` is set (Instance.Under is the zero
// TypeRef, so checkUnder's IsZero short-circuit skips it entirely),
// isolating this to exactly one blocking diagnostic.
const mixerTypeAndInstanceSrc = cleanBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
	"object_types:\n  MixerType:\n    base: OpcUa:BaseObjectType\n" +
	"instances:\n  Mixer1:\n    type: MixerType\n"

// TestRemoveType_StillReferenced_RefusedWithoutForce: removing a type still
// referenced by an instance's `type:` must refuse (a *ValidationError naming
// the unknown-type blocker) and leave the draft byte-unchanged; force:true
// must then store it (the instance's reference is left dangling, which is
// the caller's problem, not storeMutatedDraft's).
func TestRemoveType_StillReferenced_RefusedWithoutForce(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, mixerTypeAndInstanceSrc)
	before := string(d.Files["demo.yaml"])

	_, err := s.RemoveType(d.ID, "demo.yaml", "MixerType", WriteOpts{})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v (%T), want *ValidationError", err, err)
	}
	if len(ve.Blocking) != 1 || ve.Blocking[0].Code != "unknown-type" {
		t.Fatalf("Blocking = %+v, want exactly one unknown-type diagnostic", ve.Blocking)
	}
	if string(d.Files["demo.yaml"]) != before {
		t.Fatalf("draft was mutated despite refusal:\n%s", d.Files["demo.yaml"])
	}

	resp, err := s.RemoveType(d.ID, "demo.yaml", "MixerType", WriteOpts{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Stored {
		t.Fatal("Stored = false, want true on a forced removal")
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "MixerType:") {
		t.Fatalf("type still present after forced removal:\n%s", d.Files["demo.yaml"])
	}
}

// TestRemoveType_DryRun: dryRun on a still-referenced type previews the
// unknown-type diagnostic without ever refusing (dry-run never refuses) and
// without storing.
func TestRemoveType_DryRun(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, mixerTypeAndInstanceSrc)
	before := string(d.Files["demo.yaml"])

	resp, err := s.RemoveType(d.ID, "demo.yaml", "MixerType", WriteOpts{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Stored {
		t.Fatal("Stored = true, want false on a dry-run")
	}
	diags, ok := resp.Diagnostics["demo.yaml"]
	if !ok || len(diags) != 1 || diags[0].Code != "unknown-type" {
		t.Fatalf("expected the unknown-type diagnostic previewed on dry-run, got %+v", resp.Diagnostics)
	}
	if string(d.Files["demo.yaml"]) != before {
		t.Fatalf("draft was mutated by a dry-run:\n%s", d.Files["demo.yaml"])
	}
}

// TestRemoveInstance_RemovesInstance is the basic happy path for
// RemoveInstance: nothing nests under Widget1 and nothing else references
// it, so removal stores cleanly without force.
func TestRemoveInstance_RemovesInstance(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	src := cleanBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
		"object_types:\n  WidgetType:\n    base: OpcUa:BaseObjectType\n" +
		"instances:\n  Widget1:\n    type: WidgetType\n"
	d := seedDraft(t, s, src)

	resp, err := s.RemoveInstance(d.ID, "demo.yaml", "Widget1", WriteOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Stored {
		t.Fatal("Stored = false, want true on a clean, unforced removal")
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "Widget1") {
		t.Fatalf("instance still present after removal:\n%s", d.Files["demo.yaml"])
	}
}

// TestRemoveInstance_Unknown_IsNotFound: removing an absent instance is
// ErrNotFound.
func TestRemoveInstance_Unknown_IsNotFound(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc)
	_, err := s.RemoveInstance(d.ID, "demo.yaml", "Nope1", WriteOpts{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestRemoveImport_RemovesImport is the basic happy path for RemoveImport:
// DI is declared but never referenced (only OpcUa is used in base:), so
// removing it stores cleanly without force.
func TestRemoveImport_RemovesImport(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	src := cleanBaseSrc + "imports:\n  OpcUa: http://opcfoundation.org/UA/\n  DI: http://opcfoundation.org/UA/DI/\n"
	d := seedDraft(t, s, src)

	resp, err := s.RemoveImport(d.ID, "demo.yaml", "DI", WriteOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Stored {
		t.Fatal("Stored = false, want true on a clean, unforced removal")
	}
	if strings.Contains(string(d.Files["demo.yaml"]), "DI:") {
		t.Fatalf("import still present after removal:\n%s", d.Files["demo.yaml"])
	}
}

// TestRemoveImport_Unknown_IsNotFound: removing an absent import alias is
// ErrNotFound.
func TestRemoveImport_Unknown_IsNotFound(t *testing.T) {
	s := New(nil, NewStore(time.Hour))
	d := seedDraft(t, s, cleanBaseSrc)
	_, err := s.RemoveImport(d.ID, "demo.yaml", "Nope", WriteOpts{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
