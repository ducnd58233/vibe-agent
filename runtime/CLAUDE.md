# Claude Code (runtime)

Read [`AGENTS.md`](AGENTS.md) in this directory before editing anything under `runtime/`.

That file covers module boundaries, `internal/shared` usage, HTTP/logging rules, and web UI constraints. Root [`AGENTS.md`](../AGENTS.md) still applies for charter-wide policy.

Claude loads this file **on demand** when you read files under `runtime/`, not at session start. Confirm with `/context` if unsure.
