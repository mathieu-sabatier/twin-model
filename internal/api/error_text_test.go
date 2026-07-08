package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/mathieu-sabatier/twin-model/internal/dsl"
)

// This file locks the two Task 3 review findings down at the HTTP boundary:
//
//   - Finding 1: the adapter's msg() helper used to strip everything up to the
//     first ": " in the wrapped error, which mangled any Service detail that
//     itself carried a "label: " (e.g. "parse: ...", "read tree: ...") and
//     dropped words from bare details (e.g. "draft" instead of "draft not
//     found"). writeErr/msg now strip only the matched sentinel's own prefix.
//   - Finding 2: handlePropose used to validate branch/title in the adapter
//     before the draft lookup, so an unknown draft + empty branch returned 400
//     instead of the original handler's 404. The adapter no longer pre-checks;
//     core.Service.Propose orders draft-lookup (404) before branch/title (400)
//     internally, as the original handler did.
//
// None of the existing *_test.go files assert exact body text, so these
// regressions were invisible to `task check`.

// errBody decodes the {"error": "..."} shape written by httpErr.
func errBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var out struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return out.Error
}

// TestErrorTextUnknownDraft locks GET /api/drafts/{id} for an unknown id to the
// original handler's literal 404 body: `draft()` wrote "draft not found", not
// the sentinel's own text ("not found") or a mangled fragment ("draft").
func TestErrorTextUnknownDraft(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/drafts/deadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if got := errBody(t, resp); got != "draft not found" {
		t.Errorf("error = %q, want %q", got, "draft not found")
	}
}

// TestErrorTextFileNotFoundInDraft locks the "file not found in draft" 404 body
// for an existing draft whose ?file= does not resolve — distinct from the
// "draft not found" case above, and previously mangled to "file in draft" by
// the generic first-": "-strip.
func TestErrorTextFileNotFoundInDraft(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/model?file=nope.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if got := errBody(t, resp); got != "file not found in draft" {
		t.Errorf("error = %q, want %q", got, "file not found in draft")
	}
}

// TestErrorTextParseRoute locks the 422 body for a preview route hitting an
// unparseable draft file: the original handler prefixed the parse error with
// "parse: ", a label the sentinel-wrap used to drop entirely (leaving the bare
// dsl error with no "parse: " prefix at all).
func TestErrorTextParseRoute(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	garbage := "model: [this is : not valid yaml"
	putFiles(t, ts, id, map[string]string{"equipment.yaml": garbage}).Body.Close()

	resp, err := http.Get(ts.URL + "/api/drafts/" + id + "/preview/modeldesign?file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	_, parseErr := dsl.Parse("equipment.yaml", []byte(garbage))
	if parseErr == nil {
		t.Fatal("expected garbage YAML to fail to parse")
	}
	want := "parse: " + parseErr.Error()
	if got := errBody(t, resp); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

// TestErrorTextReadTreeFailure locks the 502 body for a git-host ReadTree
// failure on GET /api/model: the original handler wrote "read tree: "+err, a
// prefix the generic strip used to swallow because the sentinel word ("read
// tree") collides with the label itself.
func TestErrorTextReadTreeFailure(t *testing.T) {
	ts, host, _ := newTestServer(t)
	host.readErr = errors.New("boom")

	resp, err := http.Get(ts.URL + "/api/model?ref=main&file=equipment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	want := "read tree: boom"
	if got := errBody(t, resp); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

// TestErrorTextFileNotFoundInRef locks the "file not found in ref" 404 body for
// GET /api/model against a real ref whose ?file= does not resolve.
func TestErrorTextFileNotFoundInRef(t *testing.T) {
	ts, host, _ := newTestServer(t)
	seedEquipment(t, host, "main")

	resp, err := http.Get(ts.URL + "/api/model?ref=main&file=nope.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if got := errBody(t, resp); got != "file not found in ref" {
		t.Errorf("error = %q, want %q", got, "file not found in ref")
	}
}

// TestErrorTextProposeUnknownDraftEmptyBranch is Finding 2: an unknown draft id
// combined with an empty branch must still 404 ("draft not found"), because the
// original handler checked draft existence before body validation. A adapter
// that validates branch/title before calling the Service would wrongly return
// 400 here instead.
func TestErrorTextProposeUnknownDraftEmptyBranch(t *testing.T) {
	ts, _, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"branch": "", "title": "", "message": ""})
	resp, err := http.Post(ts.URL+"/api/drafts/deadbeefdeadbeefdeadbeefdeadbeef/propose", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (draft lookup must precede branch/title validation)", resp.StatusCode)
	}
	if got := errBody(t, resp); got != "draft not found" {
		t.Errorf("error = %q, want %q", got, "draft not found")
	}
}

// TestErrorTextProposeKnownDraftEmptyBranch is the companion 400 case: once the
// draft exists, an empty branch/title still 400s with the original combined
// message, now driven entirely by core.Service.Propose's own validation.
func TestErrorTextProposeKnownDraftEmptyBranch(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	resp := propose(t, ts, id, "", "", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if got := errBody(t, resp); got != "branch and title are required" {
		t.Errorf("error = %q, want %q", got, "branch and title are required")
	}
}

// TestErrorTextProposeKnownDraftMalformedBody is a second Finding 2 regression:
// encoding/json partially populates its target before failing, so a body like
// {"branch":"b","title":"t","message":42} decodes Branch/Title successfully and
// only fails on the Message field. A handler that ignores the decode error (as
// the adapter briefly did) would let this malformed-but-partially-valid body
// through to core.Service.Propose with a non-empty branch/title, diverging from
// the original handler's `err != nil || ...` check, which 400s on ANY decode
// error regardless of what partially decoded. handlePropose now zeroes
// Branch/Title on a decode error so this still 400s for a known draft.
func TestErrorTextProposeKnownDraftMalformedBody(t *testing.T) {
	ts, host, _ := newTestServer(t)
	id := createDraft(t, ts, host)
	body := []byte(`{"branch":"b","title":"t","message":42}`)
	resp, err := http.Post(ts.URL+"/api/drafts/"+id+"/propose", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if got := errBody(t, resp); got != "branch and title are required" {
		t.Errorf("error = %q, want %q", got, "branch and title are required")
	}
}
