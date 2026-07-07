# Security Policy

## Reporting a vulnerability

Please report security issues **privately** — do not open a public GitHub issue,
pull request, or discussion for a suspected vulnerability.

Preferred: use GitHub's private vulnerability reporting on this repository
(**Security → Report a vulnerability**). This opens a private advisory visible
only to the maintainers.

Alternatively, email **mathieu.sabatier@me.com** with `SECURITY` in the subject.

Please include, where possible:

- affected version, release tag, or commit SHA;
- a description of the issue and its impact;
- steps to reproduce or a proof of concept;
- any suggested remediation.

**What to expect:** an acknowledgement within **5 business days** and, once
triaged, an assessment and a target timeline for a fix. Fixes are developed
privately and released together with a coordinated disclosure and credit to the
reporter (unless you prefer to remain anonymous). Please allow a reasonable
period to ship a fix before any public disclosure.

## Supported versions

twinmodel is pre-1.0 and moves fast. Security fixes land on `main` and in the
**latest release**; older tags do not receive backports. Always upgrade to the
latest release before reporting.

| Version        | Supported |
| -------------- | --------- |
| latest release | ✅        |
| `main`         | ✅        |
| older releases | ❌        |

## Security model

Understanding the intended deployment model helps distinguish a vulnerability
from expected behavior.

### `serve` is a trusted-network tool

`twinmodel serve` exposes an **unauthenticated** JSON API and web UI — there is
no login, session, or per-request authorization. Anyone who can reach the listen
address can read the configured repository and, if a push token is configured,
**propose pull requests using the server's credentials**.

This is by design: `serve` is meant to run on `localhost` or a trusted network,
or **behind a reverse proxy that provides authentication and TLS**. Operational
guidance:

- The server binds `ADDR` (default `:8080`, i.e. **all interfaces**). To limit
  it to the local machine, set `ADDR=127.0.0.1:8080`.
- Do **not** expose `serve` to an untrusted network while a `GIT_TOKEN` is
  configured unless it sits behind your own authenticating proxy.
- Scope `GIT_TOKEN` as narrowly as possible — a fine-grained token limited to
  the single target repository with only the contents (push) and pull-request
  permissions it needs.

### Token handling

`GIT_TOKEN` is held **server-side only**. It is used as the HTTPS basic-auth
password (`x-access-token`) when pushing, and as the `Authorization` bearer when
calling the GitHub API. It is **never sent to the browser**: the `/api/repo`
response exposes only whether proposing is enabled and why, never the token
itself. Reads work without a token on public repositories.

### External process execution

The `compile` subcommand invokes an external toolchain — the OPC Foundation
ModelCompiler as a native `.NET` tool or a Docker image. Commands are built as
an argument vector (no shell interpretation), and `compile` is a **local CLI
operation only** — it is not reachable through the `serve` API. Running a
compiler you specify (`--compiler` / `--docker-image`) executes on your machine
with your privileges, as expected of a local build tool.

### Persistence

There is no database. Drafts live in memory with a TTL (`DRAFT_TTL`, default 2h);
Git is the only durable store. No secrets are written to disk by twinmodel.

## Scope

**In scope:** the `twinmodel` binary and CLI, the `serve` HTTP API, the embedded
web UI, credential/token handling, and the transpile/lint/export/compile
pipelines.

**Out of scope** (report upstream, though a heads-up so we can bump a pin is
welcome):

- vulnerabilities in the OPC Foundation UA-ModelCompiler or the bundled
  companion-spec NodeSets;
- vulnerabilities in third-party Go modules or npm packages;
- issues that require misconfiguration explicitly warned against above — e.g.
  exposing `serve` to an untrusted network with a privileged `GIT_TOKEN`;
- your own repository, token, CI, or GitHub account configuration.

## Third-party components

twinmodel bundles and depends on third-party components under permissive
licenses; see [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md).
