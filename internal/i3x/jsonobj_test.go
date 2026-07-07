package i3x

import (
	"encoding/json"
	"testing"
)

// TestJobjPreservesInsertionOrder is the determinism guarantee: unlike a Go map,
// a jobj marshals keys in insertion order, not sorted order. Declared member
// order (e.g. Manufacturer, SerialNumber, State, CycleCount) must survive.
func TestJobjPreservesInsertionOrder(t *testing.T) {
	o := newObj().
		set("SerialNumber", "b").
		set("Manufacturer", "a").
		set("CycleCount", 3)
	got, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"SerialNumber":"b","Manufacturer":"a","CycleCount":3}`
	if string(got) != want {
		t.Errorf("got %s want %s", got, want)
	}
}

// TestJobjNestingAndIndent proves MarshalIndent re-indents nested jobj values
// uniformly while preserving key order — the property we rely on for goldens.
func TestJobjNestingAndIndent(t *testing.T) {
	o := newObj().
		set("type", "object").
		set("props", newObj().set("A", []string{"integer", "null"})).
		set("required", []string{"A"})
	got, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "type": "object",
  "props": {
    "A": [
      "integer",
      "null"
    ]
  },
  "required": [
    "A"
  ]
}`
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
