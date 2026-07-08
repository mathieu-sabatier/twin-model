package api

import "github.com/mathieu-sabatier/twin-model/internal/core"

// Compatibility seam: the domain now lives in internal/core. These aliases let
// the HTTP adapters and the existing black-box tests keep referring to the
// short names while the concrete types live one layer down.
type (
	Store         = core.Store
	GitHost       = core.GitHost
	GitHubHost    = core.GitHubHost
	ProposeParams = core.ProposeParams
	PRError       = core.PRError
)

// NewStore constructs an in-memory draft store (re-exported for tests).
var NewStore = core.NewStore

// Constant forwarders for repo_test.go.
const (
	commitAuthorName  = core.CommitAuthorName
	commitAuthorEmail = core.CommitAuthorEmail
)
