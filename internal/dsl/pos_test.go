package dsl

import (
	"os"
	"testing"
)

// TestDiagnosticsCarryPath: every diagnostic the example-with-an-error produces
// has a non-empty Path anchored at the offending node.
func TestDiagnosticsCarryPath(t *testing.T) {
	src := hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { type: Nope } } } }`
	m, err := Parse("t.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(m)
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	for _, d := range diags {
		if d.Path == "" {
			t.Errorf("diagnostic %s has empty Path", d.Code)
		}
	}
	// The unknown-type error must anchor at the member's type field.
	var found bool
	for _, d := range diags {
		if d.Code == CodeUnknownType {
			found = true
			if d.Path != "object_types/T/members/X/type" {
				t.Errorf("Path = %q, want object_types/T/members/X/type", d.Path)
			}
		}
	}
	if !found {
		t.Fatal("expected an unknown-type diagnostic")
	}
}

// TestExampleNodesHavePositions: no node in the parsed example ships a zero Pos.
func TestExampleNodesHavePositions(t *testing.T) {
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse("examples/equipment.yaml", data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Line == 0 || m.File == "" {
		t.Errorf("model Pos not set: %+v", m.Pos)
	}
	for _, ot := range m.ObjectTypes {
		if ot.Line == 0 {
			t.Errorf("object type %q has zero Line", ot.Name)
		}
		for _, mem := range ot.Members {
			if mem.Line == 0 {
				t.Errorf("member %q.%q has zero Line", ot.Name, mem.Name)
			}
		}
	}
	for _, inst := range m.Instances {
		if inst.Line == 0 {
			t.Errorf("instance %q has zero Line", inst.Name)
		}
	}
}
