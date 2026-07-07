package api

import (
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// TestGitHubHostAuthMethod pins the shared auth used by clone/ls-remote/push.
// The no-token case MUST return a true nil interface (not a typed-nil
// *BasicAuth): a typed nil makes go-git attempt empty authentication, which
// breaks unauthenticated public-remote and local file-transport operations.
func TestGitHubHostAuthMethod(t *testing.T) {
	if auth := (&GitHubHost{}).authMethod(); auth != nil {
		t.Errorf("authMethod() with no token = %v, want nil interface", auth)
	}

	auth := (&GitHubHost{Token: "ghp_secret"}).authMethod()
	ba, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("authMethod() with token = %T, want *githttp.BasicAuth", auth)
	}
	if ba.Username != "x-access-token" || ba.Password != "ghp_secret" {
		t.Errorf("authMethod() = {%q, %q}, want {x-access-token, ghp_secret}", ba.Username, ba.Password)
	}
}
