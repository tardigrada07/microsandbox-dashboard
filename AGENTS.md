# AGENTS.md

Localhost-only web dashboard for Microsandbox sandboxes, shipped as one Go binary with an embedded UI. Design: `docs/plan.md`. Usage and contribution rules: `README.md`. Security policy: `SECURITY.md`.

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

Requires Go 1.26+.

```sh
go test ./...                              # default build, runs anywhere
go vet ./...
node --check web/app.js                    # go test does not cover web/
go build -o msbdash .                      # demo mode
go build -tags sdk -o msbdash . && ./msbdash   # real Microsandbox (cgo, KVM/Apple Silicon)
./msbdash --no-browser --addr 127.0.0.1:8484
```

## Rules

Follow the Contributing rules in `README.md` (protected `master`, localhost guards, server-side validation, `textContent` rendering, pure-Go default build, operation order, no new dependencies). Details for agents:

- **Never relax the guards** in `internal/api/auth.go`: `CheckLoopbackAddr`, the Host check, the Origin check on non-GET, the token cookie. Any change near them needs tests in `internal/api/api_test.go`.
- **Validate in `handlers.go`** before input reaches a backend: `decode` for bodies, `checkName` for the `{name}` path value, `validateCreate`/`validateModify` (name, image, env keys, `validAbs` paths, `validateResources`).
- **New Microsandbox operations:** add to the `Backend` interface, implement in both `fake.go` and `sdk.go`, then add the handler and validation.
- **Client checks mirror the server:** `parseMounts` and `parseEnv` in `app.js` only give early feedback. Change them together with `handlers.go` and add the server case to `TestValidation`.
- **SDK code** goes in `-tags sdk` files only. The SDK is cgo and supports linux/amd64, linux/arm64 and darwin/arm64 (no Windows); do not cross-compile it, CI builds natively per OS. If you cannot build with `-tags sdk` (no C toolchain or KVM), say so instead of guessing, and ask for a native build.
- **`app.js`:** render dynamic data with `textContent` or the `h()` helper, never `innerHTML`. Look up each DOM element once (top-level `const`, as `listBody`, `planBox`, `form`) instead of repeating `$('#id')`. A runtime error (e.g. a stale variable) passes `node --check`, so reload the page (or ask the user to) after edits.
- **`index.html`:** form field help is a CSS-only `.info` icon with `data-tip` (hover only, anchored to the label so the dialog does not clip it); keep tooltip text in line with the validation rules. Image presets are the `<datalist id="image-options">`.
- **CI:** actions are pinned to commit SHAs with a version comment (Dependabot updates them). `govulncheck` and `gosec` versions in `ci.yml` are pinned too and bumped by hand. Keep workflow permissions minimal; only the release `publish` job may write.

## SDK notes (v0.7.8)

- Use `WithDetached()` on create and `Detach(ctx)` afterwards so VMs outlive the dashboard; `Close()` on a detached sandbox stops it.
- Runtime install is `EnsureRuntime(ctx, RuntimeConfig{}, InstallOptions{})` (there is no `EnsureInstalled`).
- `Modify` and metrics APIs are local-only; `DryRun` previews a plan.
- `SandboxHandle.Config()` does not decode stored mounts (they are a `mounts` array tagged by `type`, not a `Volumes` map). Use `ParseStoredMounts(h.ConfigJSON())` in `internal/sandbox/mounts.go`.
- Map SDK errors with `msb.IsKind` (see `mapErr` in `sdk.go`).
