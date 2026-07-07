package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/api/dto"
)

// TestAPIValidateFlagsUnknownNS0Ref asserts that a draft model importing
// OpcUa (ns0) with a type whose base resolves to an unknown OpcUa name
// produces an "unknown-import-type" diagnostic from the validate endpoint.
func TestAPIValidateFlagsUnknownNS0Ref(t *testing.T) {
	ts, host, _ := newTestServer(t)
	const ref = "main"
	host.trees[ref] = map[string][]byte{
		"model.yaml": []byte("model:\n  name: TestModel\n  namespace: https://test.example/UA/\n  version: 1.0.0\n  publication_date: 2026-01-01\n" +
			"imports:\n  OpcUa: http://opcfoundation.org/UA/\n" +
			"object_types:\n  Widget:\n    base: OpcUa:Nonexistent\n"),
	}

	// Create a draft from the ref.
	cresp, err := http.Post(ts.URL+"/api/drafts", "application/json", strings.NewReader(`{"baseRef":"`+ref+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	var cd dto.CreateDraftResponse
	if err := json.NewDecoder(cresp.Body).Decode(&cd); err != nil {
		t.Fatal(err)
	}
	cresp.Body.Close()

	if cd.ID == "" {
		t.Fatal("create draft returned empty ID")
	}

	// Validate the draft file.
	vresp, err := http.Post(ts.URL+"/api/drafts/"+cd.ID+"/validate?file=model.yaml", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer vresp.Body.Close()

	if vresp.StatusCode != http.StatusOK {
		t.Fatalf("validate status = %d, want 200", vresp.StatusCode)
	}

	var vr dto.ValidateResponse
	if err := json.NewDecoder(vresp.Body).Decode(&vr); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, d := range vr.Diagnostics {
		if d.Code == "unknown-import-type" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown-import-type for OpcUa:Nonexistent, got %+v", vr.Diagnostics)
	}
}
