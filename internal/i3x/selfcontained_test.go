package i3x

import (
	"testing"
)

// TestBundleIsSelfContained is the Level-A guarantee: every elementId used as a
// reference (sourceTypeId, typeElementId, parentId, composition typeDefinition,
// cross-document $ref) resolves to an entity emitted somewhere in the bundle —
// so the export loads with zero external namespace catalog. Standard OPC UA (ns0)
// nodes are included as minimal reference stubs.
func TestBundleIsSelfContained(t *testing.T) {
	m := loadExampleModel(t)
	b, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}

	var objectTypes, objects, relTypes []map[string]any
	mustUnmarshal(t, b.File("objecttypes.json"), &objectTypes)
	mustUnmarshal(t, b.File("objects.json"), &objects)
	mustUnmarshal(t, b.File("relationshiptypes.json"), &relTypes)

	emitted := map[string]bool{}
	for _, group := range [][]map[string]any{objectTypes, objects, relTypes} {
		for _, e := range group {
			if id, ok := e["elementId"].(string); ok {
				emitted[id] = true
			}
		}
	}

	// Collect every referenced elementId (skip empty and local #/$defs pointers).
	referenced := map[string]bool{}
	note := func(v any) {
		if s, ok := v.(string); ok && s != "" && !isLocalRef(s) {
			referenced[s] = true
		}
	}
	for _, ot := range objectTypes {
		note(ot["sourceTypeId"])
		walkElementRefs(ot["schema"], note)
	}
	for _, o := range objects {
		note(o["typeElementId"])
		note(o["parentId"])
	}

	if len(referenced) == 0 {
		t.Fatal("no references collected — test is not exercising anything")
	}
	for ref := range referenced {
		if !emitted[ref] {
			t.Errorf("referenced elementId %q does not resolve to an emitted entity", ref)
		}
	}

	// And the ns0 stubs we expect are present.
	for _, want := range []string{
		"nsu=http://opcfoundation.org/UA/;s=BaseObjectType",
		"nsu=http://opcfoundation.org/UA/;s=FolderType",
		"nsu=http://opcfoundation.org/UA/;s=ObjectsFolder",
	} {
		if !emitted[want] {
			t.Errorf("expected ns0 stub %q to be emitted", want)
		}
	}
}

func isLocalRef(s string) bool { return len(s) > 0 && s[0] == '#' }

// walkElementRefs invokes fn for elementId-valued reference fields inside a schema
// (composition typeDefinition and additionalProperties $ref). It ignores the
// schema's own $id and local #/$defs $refs.
func walkElementRefs(v any, fn func(any)) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			switch k {
			case "typeDefinition":
				fn(val)
			case "$ref":
				fn(val) // fn skips #/$defs via isLocalRef
			case "$id":
				// identity, not a reference
			default:
				walkElementRefs(val, fn)
			}
		}
	case []any:
		for _, e := range t {
			walkElementRefs(e, fn)
		}
	}
}
