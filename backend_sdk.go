//go:build sdk

package main

import (
	"context"

	"microsandbox-dashboard/internal/sandbox"
)

func newBackend(ctx context.Context) (sandbox.Backend, string, error) {
	b, err := sandbox.NewSDK(ctx)
	if err != nil {
		return nil, "", err
	}
	return b, "Using the Microsandbox SDK backend", nil
}
