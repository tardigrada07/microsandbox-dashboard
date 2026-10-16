# Microsandbox Dashboard

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

## Security

- Only loopback listen addresses are accepted.
- Requests need a random per-run token (exchanged for an HttpOnly, SameSite=Strict cookie).
- The `Host` header must be loopback (DNS-rebinding defense); unsafe methods need a same-origin `Origin`.

## Platforms

The Microsandbox Go SDK is a cgo wrapper and supports linux/amd64, linux/arm64 and darwin/arm64 only.
Each binary must be built natively (see `.github/workflows/release.yml`). Windows is not supported yet.

## Test

```sh
go test ./...
```

## License

Copyright 2026 Tardigrada07. Licensed under the [Apache License 2.0](LICENSE).
Free for private and commercial use. If you redistribute it (source or binaries), keep the
`LICENSE` and `NOTICE` files and state your changes.

This is an independent project, not affiliated with the Microsandbox authors.
