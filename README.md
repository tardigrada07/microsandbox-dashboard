# Microsandbox Dashboard

[![ci](https://github.com/tardigrada07/microsandbox-dashboard/actions/workflows/ci.yml/badge.svg)](https://github.com/tardigrada07/microsandbox-dashboard/actions/workflows/ci.yml)
[![codeql](https://github.com/tardigrada07/microsandbox-dashboard/actions/workflows/codeql.yml/badge.svg)](https://github.com/tardigrada07/microsandbox-dashboard/actions/workflows/codeql.yml)

A single-binary, localhost-only web dashboard for [Microsandbox](https://microsandbox.dev) sandboxes:
create, edit (live CPU/memory resize, env, workdir), start, stop and delete sandboxes, and see their
status, live metrics and mounted directories.

See `docs/plan.md` for the design.

## Run

```sh
# Real Microsandbox (needs cgo + a C toolchain; Linux with KVM, or macOS on Apple Silicon)
go build -tags sdk -o msbdash .
./msbdash

# Demo mode (pure Go, in-memory fake sandboxes; works anywhere)
go build -o msbdash . && ./msbdash
```

On start it prints `http://127.0.0.1:8484/?token=...` and opens it in your browser. Flags: `--addr`
(loopback only), `--no-browser`.

## Creating a sandbox

Click **New sandbox** and fill in the form (hover the **i** icons for field help):

- **Image:** pick a preset (Ubuntu latest/LTS, Debian, Python, Java) or type any image reference.
- **CPUs / Memory:** 1-64 CPUs and at least 128 MiB. Both can be resized live after creation.
- **Workdir:** absolute directory where commands start; empty uses the image default.
- **Env:** one `KEY=VALUE` per line.
- **Mounts:** optional, one `HOST:GUEST[:ro]` per line with absolute paths, e.g. `/home/me/project:/work`.
  Leave it empty for no shared host folders. Mounts, name and image are fixed once created.

## Requirements

Docker is not required.

- **Images:** the images (`ubuntu:latest`, `python:3.13-slim` and so on) are OCI images, the same format
  Docker uses. Microsandbox pulls them from a registry (Docker Hub by default) itself, so you don't need a
  Docker daemon or the `docker` CLI.
- **Runtime:** each sandbox is a microVM, not a container. There is no daemon; the dashboard talks to
  sandboxes directly. The SDK downloads `msb` and libkrunfw to `~/.microsandbox` on first use
  (`EnsureRuntime`).
- **Real build:** Linux with KVM (or macOS on Apple Silicon) and a C toolchain, because the SDK uses cgo.
  The demo build needs none of this.

## Security

- Only loopback listen addresses are accepted.
- Requests need a random per-run token (exchanged for an HttpOnly, SameSite=Strict cookie).
- The `Host` header must be loopback (DNS-rebinding defense); unsafe methods need a same-origin `Origin`.

## Platforms

The Microsandbox Go SDK is a cgo wrapper and supports linux/amd64, linux/arm64 and darwin/arm64 only.
Each binary must be built natively (see `.github/workflows/release.yml`). Windows is not supported yet.

## Test

```sh
go vet ./...
go test ./...
```

The tests use the in-memory backend, so they need no KVM or cgo. They do not cover `web/`; after editing
`web/app.js`, also run `node --check web/app.js` and reload the page.

## Contributing

`master` is protected: create a branch, push it and open a pull request. CI must pass before merging.

## License

Apache License 2.0, see `LICENSE` and `NOTICE`.
