//go:build !sdk

package main

import (
	"context"

	"microsandbox-dashboard/internal/sandbox"
)

// Default build: pure Go, no cgo. Uses an in-memory demo backend so the UI
// can run anywhere. Build with `-tags sdk` for the real Microsandbox backend.
func newBackend(context.Context) (sandbox.Backend, string, error) {
	return sandbox.NewDemoFake(), "DEMO MODE: in-memory fake backend (build with -tags sdk for real Microsandbox)", nil
}
