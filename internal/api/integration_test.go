package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mathieu-sabatier/twin-model/internal/semdiff"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// makeEquipmentFixtureRepo creates a temporary local git repo seeded with the
// real examples/equipment.yaml.  It uses go-git (no exec dependency) so the
// test stays hermetic on any machine that has the go module cache populated.
// The caller gets the repo path and the default branch name ("master").
func makeEquipmentFixtureRepo(t *testing.T) (repoPath, defaultBranch string) {
	t.Helper()

	// Read the real equipment.yaml so the model has FurnaceType, instances, etc.
	equipment, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatalf("read equipment.yaml: %v", err)
	}

	dir := t.TempDir()
	repo, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.Master,
		},
	})
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	if err := os.WriteFile(dir+"/equipment.yaml", equipment, 0o644); err != nil {
		t.Fatalf("write equipment.yaml: %v", err)
	}
	if _, err := wt.Add("equipment.yaml"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	sig := &object.Signature{Name: "fixture", Email: "fixture@test", When: time.Now()}
	if _, err := wt.Commit("init", &git.CommitOptions{
		Author:    sig,
		Committer: sig,
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	return dir, head.Name().Short()
}

// fakePRServer returns an httptest.Server that stubs the GitHub REST "create PR"
// endpoint and returns a well-known html_url.  The returned URL is what the
// propose handler should return.
func fakePRServer(t *testing.T) (ts *httptest.Server, prURL string) {
	t.Helper()
	const want = "https://github.com/demo/model/pull/99"
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST /repos/.../pulls
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/pulls") {
			// ListPRs may GET /repos/.../pulls - return empty list
			if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/pulls") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "[]")
				return
			}
			http.Error(w, "unexpected", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"html_url": want})
	}))
	t.Cleanup(ts.Close)
	return ts, want
}

// TestDraftLifecycleAgainstLocalGitFixture exercises the complete happy path:
// create draft → edit (add Pressure variable with unit) → validate (0 errors) →
// diff (MemberAdded) → raw-file (canonical, contains new member + unit) →
// propose (PR URL returned, lint-clean draft).
func TestDraftLifecycleAgainstLocalGitFixture(t *testing.T) {
	repoPath, baseRef := makeEquipmentFixtureRepo(t)

	fakeGH, prURL := fakePRServer(t)

	host := &GitHubHost{
		RepoURL: repoPath,
		APIBase: fakeGH.URL,
		Owner:   "demo",
		Repo:    "model",
	}
	ts := httptest.NewServer(NewServer(host, NewStore(time.Hour)).Routes())
	t.Cleanup(ts.Close)

	// ── 1. POST /api/drafts → 201 ──────────────────────────────────────────────
	body, _ := json.Marshal(map[string]string{"baseRef": baseRef})
	resp, err := http.Post(ts.URL+"/api/drafts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create draft: status %d, want 201", resp.StatusCode)
	}
	var created struct {
		ID    string   `json:"id"`
		Files []string `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create draft returned empty id")
	}
	if len(created.Files) == 0 || created.Files[0] != "equipment.yaml" {
		t.Fatalf("create draft files = %v, want [equipment.yaml]", created.Files)
	}
	draftID := created.ID

	// ── 2. GET /api/drafts/{id}/model → 200, model has FurnaceType ────────────
	modelResp, err := http.Get(ts.URL + "/api/drafts/" + draftID + "/model?file=equipment.yaml")
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	defer modelResp.Body.Close()
	if modelResp.StatusCode != http.StatusOK {
		t.Fatalf("get model: status %d, want 200", modelResp.StatusCode)
	}
	var mr struct {
		Model struct {
			ObjectTypes []struct {
				Name string `json:"name"`
			} `json:"objectTypes"`
		} `json:"model"`
		ParseError string `json:"parseError"`
	}
	if err := json.NewDecoder(modelResp.Body).Decode(&mr); err != nil {
		t.Fatalf("decode model response: %v", err)
	}
	if mr.ParseError != "" {
		t.Fatalf("model has parse error: %s", mr.ParseError)
	}
	hasFurnace := false
	for _, ot := range mr.Model.ObjectTypes {
		if ot.Name == "FurnaceType" {
			hasFurnace = true
		}
	}
	if !hasFurnace {
		t.Errorf("model missing FurnaceType; got types: %v", mr.Model.ObjectTypes)
	}

	// ── 3. GET the raw file, then PUT back with Pressure added to FurnaceType ─
	rawResp, err := http.Get(ts.URL + "/api/drafts/" + draftID + "/file?file=equipment.yaml")
	if err != nil {
		t.Fatalf("get raw file: %v", err)
	}
	rawBytes, _ := io.ReadAll(rawResp.Body)
	rawResp.Body.Close()
	if rawResp.StatusCode != http.StatusOK {
		t.Fatalf("get raw file: status %d", rawResp.StatusCode)
	}
	// Inject "      Pressure: { type: Double, unit: bar }" after the DoorClosed member.
	// The canonical form of FurnaceType has DoorClosed as first member — we
	// inject the new variable right after that line.
	original := string(rawBytes)
	inject := "\n      Pressure: { type: Double, unit: bar }"
	edited := strings.Replace(original, "      DoorClosed: { type: Boolean }", "      DoorClosed: { type: Boolean }"+inject, 1)
	if edited == original {
		t.Fatalf("injection did not change the file — check that DoorClosed line matches; got:\n%.400s", original)
	}

	putBody, _ := json.Marshal(map[string]any{"files": map[string]string{"equipment.yaml": edited}})
	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/drafts/"+draftID+"/files", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("put files: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("put files: status %d, want 200", putResp.StatusCode)
	}

	// ── 4. POST /api/drafts/{id}/validate → 200, zero error-severity entries ──
	valResp, err := http.Post(ts.URL+"/api/drafts/"+draftID+"/validate?file=equipment.yaml", "application/json", nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	defer valResp.Body.Close()
	if valResp.StatusCode != http.StatusOK {
		t.Fatalf("validate: status %d, want 200", valResp.StatusCode)
	}
	var vr struct {
		Diagnostics []struct {
			Severity string `json:"severity"`
			Code     string `json:"code"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
		ParseError string `json:"parseError"`
	}
	if err := json.NewDecoder(valResp.Body).Decode(&vr); err != nil {
		t.Fatalf("decode validate response: %v", err)
	}
	if vr.ParseError != "" {
		t.Fatalf("validate: unexpected parse error: %s", vr.ParseError)
	}
	for _, d := range vr.Diagnostics {
		if d.Severity == "error" {
			t.Errorf("validate: unexpected error diagnostic: code=%q msg=%q", d.Code, d.Message)
		}
	}

	// ── 5. GET /api/drafts/{id}/diff → MemberAdded for Pressure, text non-empty
	diffResp, err := http.Get(ts.URL + "/api/drafts/" + draftID + "/diff?file=equipment.yaml")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	defer diffResp.Body.Close()
	if diffResp.StatusCode != http.StatusOK {
		t.Fatalf("diff: status %d, want 200", diffResp.StatusCode)
	}
	var dr struct {
		Changes []semdiff.Change `json:"changes"`
		Text    string           `json:"text"`
	}
	if err := json.NewDecoder(diffResp.Body).Decode(&dr); err != nil {
		t.Fatalf("decode diff response: %v", err)
	}
	if dr.Text == "" {
		t.Error("diff: text is empty")
	}
	foundMemberAdded := false
	for _, c := range dr.Changes {
		if c.Kind == semdiff.MemberAdded && c.Type == "FurnaceType" && c.Member == "Pressure" {
			foundMemberAdded = true
		}
	}
	if !foundMemberAdded {
		t.Errorf("diff: no MemberAdded for FurnaceType.Pressure; got %+v", dr.Changes)
	}

	// ── 6. GET /api/drafts/{id}/file → canonical text/plain, contains Pressure + unit: bar
	fileResp, err := http.Get(ts.URL + "/api/drafts/" + draftID + "/file?file=equipment.yaml")
	if err != nil {
		t.Fatalf("file raw: %v", err)
	}
	defer fileResp.Body.Close()
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("file raw: status %d, want 200", fileResp.StatusCode)
	}
	if ct := fileResp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("file raw: Content-Type = %q, want text/plain", ct)
	}
	rawOut, _ := io.ReadAll(fileResp.Body)
	if !bytes.Contains(rawOut, []byte("Pressure")) {
		t.Errorf("file raw: canonical output missing Pressure:\n%s", rawOut)
	}
	if !bytes.Contains(rawOut, []byte("unit: bar")) {
		t.Errorf("file raw: canonical output missing 'unit: bar':\n%s", rawOut)
	}

	// ── 7. POST /api/drafts/{id}/propose → 200, returns PR URL (lint-clean) ──
	propBody, _ := json.Marshal(map[string]string{
		"branch":  "model/add-pressure",
		"title":   "Add Pressure variable to FurnaceType",
		"message": "Adds Pressure (Double, bar) to FurnaceType members.",
	})
	propResp, err := http.Post(ts.URL+"/api/drafts/"+draftID+"/propose", "application/json", bytes.NewReader(propBody))
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	defer propResp.Body.Close()
	if propResp.StatusCode == http.StatusConflict {
		var cr struct {
			Diagnostics []struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"diagnostics"`
		}
		json.NewDecoder(propResp.Body).Decode(&cr)
		t.Fatalf("propose: 409 blocking diagnostics: %+v", cr.Diagnostics)
	}
	if propResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(propResp.Body)
		t.Fatalf("propose: status %d, want 200; body: %s", propResp.StatusCode, body)
	}
	var pr struct {
		URL string `json:"url"`
	}
	json.NewDecoder(propResp.Body).Decode(&pr)
	if pr.URL != prURL {
		t.Errorf("propose: url = %q, want %q", pr.URL, prURL)
	}
}

// TestProposeBlockedWhenLintRed asserts that a draft with a lint error (unknown
// type reference) is rejected with 409 before any PR is opened.
func TestProposeBlockedWhenLintRed(t *testing.T) {
	repoPath, baseRef := makeEquipmentFixtureRepo(t)

	fakeGH, _ := fakePRServer(t)
	host := &GitHubHost{
		RepoURL: repoPath,
		APIBase: fakeGH.URL,
		Owner:   "demo",
		Repo:    "model",
	}
	ts := httptest.NewServer(NewServer(host, NewStore(time.Hour)).Routes())
	t.Cleanup(ts.Close)

	// Create a draft.
	body, _ := json.Marshal(map[string]string{"baseRef": baseRef})
	resp, err := http.Post(ts.URL+"/api/drafts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" {
		t.Fatal("empty draft id")
	}
	draftID := created.ID

	// PUT a file with a known error: member type that does not exist.
	bad := "model:\n  name: M\n  namespace: https://x/\n  version: 1.0.0\n  publication_date: 2026-07-02\n" +
		"object_types:\n  T:\n    base: OpcUa:BaseObjectType\n    members:\n      X: { type: NoSuchType }\n"
	putBody, _ := json.Marshal(map[string]any{"files": map[string]string{"equipment.yaml": bad}})
	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/drafts/"+draftID+"/files", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("put files: %v", err)
	}
	putResp.Body.Close()

	// Propose should be rejected with 409 and carry blocking diagnostics.
	propBody, _ := json.Marshal(map[string]string{"branch": "model/broken", "title": "Broken", "message": "x"})
	propResp, err := http.Post(ts.URL+"/api/drafts/"+draftID+"/propose", "application/json", bytes.NewReader(propBody))
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	defer propResp.Body.Close()
	if propResp.StatusCode != http.StatusConflict {
		t.Fatalf("propose: status %d, want 409 (lint-red draft)", propResp.StatusCode)
	}
	var cr struct {
		Diagnostics []struct {
			Code     string `json:"code"`
			Severity string `json:"severity"`
		} `json:"diagnostics"`
	}
	json.NewDecoder(propResp.Body).Decode(&cr)
	if len(cr.Diagnostics) == 0 {
		t.Error("propose 409 body missing diagnostics")
	}
	found := false
	for _, d := range cr.Diagnostics {
		if d.Code == "unknown-type" {
			found = true
		}
	}
	if !found {
		t.Errorf("propose 409 diagnostics missing unknown-type; got %+v", cr.Diagnostics)
	}
}
