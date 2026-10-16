# Microsandbox Dashboard – Plan

## Context
A small native app that serves a browser UI for managing Microsandbox sandboxes (named VM instances created from an OCI image, e.g. `node:24-bookworm-slim`). 
Goal: create/modify/delete sandboxes and show status, metrics and mounted directories. Decisions made with the user: Go + official Go SDK (`github.com/superradcompany/microsandbox/sdk/go`), sandboxes only (no image-cache management), Windows deferred until the SDK supports it. Confirmed: the app works localhost-only (binds 127.0.0.1, no remote access, no login).

## Key constraints (from SDK docs)
- SDK is a cgo wrapper over a Rust library: needs CGO + C toolchain, so no cross-compiling from one host. Build natively per OS in CI.
- SDK supported targets: linux/amd64, linux/arm64, darwin/arm64. No Windows, no Intel Mac. Binaries now: Linux + macOS; Windows is a later phase.
- No daemon: the dashboard process talks to sandboxes directly. Needs KVM on Linux / Apple Silicon on macOS. `EnsureRuntime` downloads `msb` + libkrunfw to `~/.microsandbox` on first use.
- Metrics APIs and `Modify` are local-only (fine for this app).
- Resolved from the SDK source (v0.7.8): bind mounts use `Mount.Bind`; stored mounts are not decoded by `SandboxHandle.Config()`, so the dashboard parses `ConfigJSON()` itself (`internal/sandbox/mounts.go`).

## Architecture
- Single Go binary: HTTP server on `127.0.0.1:<port>` (flag `--addr`), random token in the URL or cookie as CSRF guard, opens the browser on start.
- UI: static SPA (vanilla JS or small Preact build) embedded via `go:embed`; no runtime Node dependency.
- Backend layer `internal/sandbox` wraps the SDK behind an interface (`List, Get, Create, Modify, Start, Stop, Remove, Metrics`) so the later Windows/`msb` CLI backend can drop in and handlers can be tested with a fake.
- REST API (JSON):
  - `GET /api/sandboxes` – name, image, status, cpus, memory, mounts
  - `POST /api/sandboxes` – create (image, cpus, memory, env, workdir, mounts)
  - `PATCH /api/sandboxes/{name}` – Modify (cpus/memory/env/workdir/disk); supports `dryRun` to show the plan
  - `POST /api/sandboxes/{name}/start|stop`
  - `DELETE /api/sandboxes/{name}` (must be stopped; UI offers stop-then-delete)
  - `GET /api/sandboxes/{name}/metrics` (snapshot), `GET /api/metrics/stream` (SSE using `AllSandboxMetrics` or `MetricsStream`)
- UI views: list with status badge and CPU/mem sparklines; detail page (config, mounts table host→guest, live metrics charts); create/edit form with Modify dry-run preview; delete confirm.

## Files
- `go.mod`, `main.go` (flags, server start, open browser)
- `internal/sandbox/{backend.go,sdk.go,fake.go}`
- `internal/api/{handlers.go,sse.go,auth.go}`
- `web/` (index.html, app.js, style.css) embedded by `web/embed.go`
- `.github/workflows/release.yml` – matrix: ubuntu-latest (amd64), ubuntu-24.04-arm, macos-14 (arm64); later windows-latest
- `README.md`

## Steps
1. Read SDK source/pkg docs to confirm mounts, metrics, status and Modify types; write a throwaway spike that creates, lists, metrics and removes a sandbox.
2. Scaffold module, backend interface, fake backend, API handlers + tests against the fake.
3. SDK backend implementation.
4. Web UI, with SSE metrics.
5. Localhost/token guard, input validation (names, paths) – it can create and delete VMs and mount host dirs.
6. CI release matrix producing the binaries.

## Verification
- `go test ./...` (handlers against fake backend).
- On a Linux KVM host: run the binary, create a sandbox from `node:24-bookworm-slim` with a bind mount, verify it shows running, live metrics, and mounts; Modify CPU/memory (dry run then apply); stop; delete; confirm removal in `msb ls`.
- Verify the binary is rejected for non-local origins/missing token.
- Run macOS arm64 build smoke test via CI artifact.

## Open items
- Localhost-only is a hard requirement: reject non-loopback `--addr` values, and check Host/Origin headers (DNS-rebinding/CSRF guard).
- Windows: revisit when the SDK ships a Windows library, or add an `msb` CLI backend.
