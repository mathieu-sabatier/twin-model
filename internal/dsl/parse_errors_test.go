package dsl

import "testing"

// Parser must reject unknown keys in every nested construct (typo protection),
// not just at the model header.
func TestParseRejectsUnknownNestedKeys(t *testing.T) {
	cases := map[string]string{
		"enum":     hdr + `enums: { E: { values: [A], bogus: 1 } }`,
		"type":     hdr + `object_types: { T: { base: OpcUa:BaseObjectType, wat: 1 } }`,
		"member":   hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: { type: Double, huh: 1 } } } }`,
		"argument": hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { M: { kind: method, in: [ { name: A, type: Double, oops: 1 } ] } } } }`,
		"instance": hdr + `instances: { i: { type: T, under: OpcUa:ObjectsFolder, extra: 1 } }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse("t.yaml", []byte(src)); err == nil {
				t.Errorf("expected a parse error for unknown %s key", name)
			}
		})
	}
}

// Wrong-shape values (scalar where a mapping is required) must be rejected with a
// positioned error, not panic.
func TestParseRejectsWrongShapes(t *testing.T) {
	cases := map[string]string{
		"model-scalar":  `model: nope`,
		"enum-scalar":   hdr + `enums: { E: 5 }`,
		"member-scalar": hdr + `object_types: { T: { base: OpcUa:BaseObjectType, members: { X: 5 } } }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse("t.yaml", []byte(src)); err == nil {
				t.Errorf("expected a parse error for %s", name)
			}
		})
	}
}
