package schema_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/schema"
)

// TestSchemaAcceptsFullFeatureCorpus proves the committed JSON Schema accepts
// every construct the parser accepts. If the parser grows a field the schema
// omits (as level/perspectives once were), this fails.
func TestSchemaAcceptsFullFeatureCorpus(t *testing.T) {
	raw, err := os.ReadFile("testdata/schema_corpus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// 1. The parser must accept the corpus (it is, by construction, valid source).
	if _, err := dsl.Parse("schema_corpus.yaml", raw); err != nil {
		t.Fatalf("corpus does not parse — fix the corpus: %v", err)
	}
	// 2. The schema must accept the same corpus.
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc = normalizeYAML(doc) // yaml.v3 yields map[string]any already; normalize nested maps for the validator

	compiler := jsonschema.NewCompiler()
	var schemaDoc any
	if err := json.Unmarshal([]byte(schema.JSON), &schemaDoc); err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("twinmodel.schema.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	sch, err := compiler.Compile("twinmodel.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(doc); err != nil {
		t.Fatalf("schema REJECTS a parser-valid corpus (schema is missing/too-strict):\n%v", err)
	}
}

// normalizeYAML converts any map[any]any (older yaml) to map[string]any so the
// JSON Schema validator can walk it. yaml.v3 already uses string keys; this is a
// defensive pass for nested content.
func normalizeYAML(v any) any {
	switch m := v.(type) {
	case map[string]any:
		for k, val := range m {
			m[k] = normalizeYAML(val)
		}
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[strings.TrimSpace(toKey(k))] = normalizeYAML(val)
		}
		return out
	case []any:
		for i := range m {
			m[i] = normalizeYAML(m[i])
		}
		return m
	default:
		return v
	}
}

func toKey(k any) string {
	if s, ok := k.(string); ok {
		return s
	}
	return ""
}
