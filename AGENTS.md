# AGENTS.md

Localhost-only web dashboard for Microsandbox sandboxes, shipped as one Go binary with an embedded UI. Design: `docs/plan.md`. Usage: `README.md`.

## Layout

- `main.go` – flags, loopback check, token, HTTP server, browser launch.
- `backend_demo.go` (`//go:build !sdk`) – default build; `newBackend` returns the in-memory fake (`sandbox.NewDemoFake`). Pure Go, no cgo.
- `backend_sdk.go` (`//go:build sdk`) – `newBackend` returns the real SDK backend. Keep both files' `newBackend` signatures identical.
- `internal/sandbox/backend.go` – `Backend` interface and shared types. The HTTP layer depends only on this.
- `internal/sandbox/fake.go` – in-memory backend (tests, demo mode).
- `internal/sandbox/sdk.go` (`//go:build sdk`) – real backend on `github.com/superradcompany/microsandbox/sdk/go` (v0.7.8).
- `internal/api/auth.go` – loopback address check, token cookie, Host/Origin guard.
- `internal/api/handlers.go` – REST routes, input validation, SSE metrics stream.
- `web/` – `index.html`, `style.css`, `app.js` (vanilla, no build step), embedded via `web/embed.go`.

## Commands

Go is not on PATH by default in some environments (check `/opt/gotool/go/bin`).

```sh
go test ./...                              # default build, runs anywhere
go vet ./...
go build -o msbdash .                      # demo mode
go build -tags sdk -o msbdash . && ./msbdash   # real Microsandbox (cgo, KVM/Apple Silicon)
./msbdash --no-browser --addr 127.0.0.1:8484
```

## Rules

- **Localhost only is a hard requirement.** Never relax `CheckLoopbackAddr`, the Host check, the Origin check on non-GET, or the token cookie. Add tests in `internal/api/api_test.go` for any change near them.
- The default build must stay cgo-free and testable without KVM. Anything touching the SDK goes in `-tags sdk` files only.
- The SDK is cgo and supports only linux/amd64, linux/arm64 and darwin/arm64 (no Windows). Do not cross-compile the `sdk` build; CI builds natively per OS.
- Add new Microsandbox operations to the `Backend` interface, then implement in both `fake.go` and `sdk.go`, then add the handler and validation.
- Validate all user input in `handlers.go` (names, image refs, env keys, absolute clean paths, resource bounds) before it reaches a backend.
- In `app.js`, render dynamic data with `textContent`/the `h()` helper, never `innerHTML` (names, env and paths are untrusted).
- The SDK backend builds and runs on the owner's machine (list, status, metrics and mounts confirmed). The agent sandbox has no C toolchain or KVM, so SDK changes can't be compiled here; ask the user to rebuild with `-tags sdk` and report back.

## SDK notes (v0.7.8)

- Use `WithDetached()` on create and `Detach(ctx)` afterwards so VMs outlive the dashboard; `Close()` on a detached sandbox stops it.
- Runtime install is `EnsureRuntime(ctx, RuntimeConfig{}, InstallOptions{})` (there is no `EnsureInstalled`).
- `Modify` and metrics APIs are local-only; `DryRun` previews a plan.
- `SandboxHandle.Config()` does not decode stored mounts (they are a `mounts` array tagged by `type`, not a `Volumes` map). Use `ParseStoredMounts(h.ConfigJSON())` in `internal/sandbox/mounts.go`.
- Map SDK errors with `msb.IsKind` (see `mapErr` in `sdk.go`).
