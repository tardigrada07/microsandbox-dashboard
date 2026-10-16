//go:build sdk

package sandbox

import (
	"context"
	"fmt"

	msb "github.com/superradcompany/microsandbox/sdk/go"
)

// SDK is the real backend, built on the Microsandbox Go SDK (cgo; Linux with
// KVM or macOS on Apple Silicon).
type SDK struct{}

// NewSDK makes sure the Microsandbox runtime is installed and returns a backend.
func NewSDK(ctx context.Context) (*SDK, error) {
	if _, err := msb.EnsureRuntime(ctx, msb.RuntimeConfig{}, msb.InstallOptions{}); err != nil {
		return nil, fmt.Errorf("install microsandbox runtime: %w", err)
	}
	return &SDK{}, nil
}

func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case msb.IsKind(err, msb.ErrSandboxNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	case msb.IsKind(err, msb.ErrSandboxAlreadyExists):
		return fmt.Errorf("%w: %v", ErrExists, err)
	case msb.IsKind(err, msb.ErrSandboxStillRunning):
		return fmt.Errorf("%w: %v", ErrRunning, err)
	}
	return err
}

func toSandbox(h *msb.SandboxHandle) (Sandbox, error) {
	cfg, err := h.Config()
	if err != nil {
		return Sandbox{}, err
	}
	s := Sandbox{
		Name: h.Name(), Image: cfg.Image, Status: string(h.Status()),
		CPUs: int(cfg.CPUs), MemoryMiB: int(cfg.MemoryMiB), Workdir: cfg.Workdir,
		Env: cfg.Env, Mounts: []Mount{}, CreatedAt: h.CreatedAt(),
	}
	if s.Env == nil {
		s.Env = map[string]string{}
	}
	// cfg.Volumes stays empty for stored configs (different JSON shape), so
	// read the mounts from the raw persisted config instead.
	mounts, err := ParseStoredMounts(h.ConfigJSON())
	if err != nil {
		return Sandbox{}, err
	}
	s.Mounts = mounts
	return s, nil
}

func (*SDK) List(ctx context.Context) ([]Sandbox, error) {
	out := []Sandbox{}
	var opts []msb.SandboxListOption
	for {
		page, err := msb.ListSandboxesWith(ctx, opts...)
		if err != nil {
			return nil, mapErr(err)
		}
		for _, h := range page.Sandboxes {
			s, err := toSandbox(h)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		if page.NextCursor == nil {
			return out, nil
		}
		opts = []msb.SandboxListOption{msb.WithListCursor(*page.NextCursor)}
	}
}

func (*SDK) Get(ctx context.Context, name string) (Sandbox, error) {
	h, err := msb.GetSandbox(ctx, name)
	if err != nil {
		return Sandbox{}, mapErr(err)
	}
	return toSandbox(h)
}

// Create creates the sandbox and leaves it running detached.
func (b *SDK) Create(ctx context.Context, c CreateSpec) (Sandbox, error) {
	opts := []msb.SandboxOption{
		msb.WithImage(c.Image),
		msb.WithCPUs(uint8(c.CPUs)),
		msb.WithMemory(uint32(c.MemoryMiB)),
		msb.WithDetached(),
	}
	if c.Workdir != "" {
		opts = append(opts, msb.WithWorkdir(c.Workdir))
	}
	if len(c.Env) > 0 {
		opts = append(opts, msb.WithEnv(c.Env))
	}
	if len(c.Mounts) > 0 {
		mounts := map[string]msb.MountConfig{}
		for _, m := range c.Mounts {
			mounts[m.GuestPath] = msb.Mount.Bind(m.HostPath, msb.MountOptions{Readonly: m.Readonly})
		}
		opts = append(opts, msb.WithMounts(mounts))
	}
	sb, err := msb.CreateSandbox(ctx, c.Name, opts...)
	if err != nil {
		return Sandbox{}, mapErr(err)
	}
	// Release our handle without stopping the detached VM.
	if err := sb.Detach(ctx); err != nil {
		return Sandbox{}, err
	}
	return b.Get(ctx, c.Name)
}

func (*SDK) Modify(ctx context.Context, name string, m ModifySpec) (Plan, error) {
	h, err := msb.GetSandbox(ctx, name)
	if err != nil {
		return Plan{}, mapErr(err)
	}
	o := msb.ModifyOptions{
		CPUs: uint8(m.CPUs), MemoryMiB: uint32(m.MemoryMiB),
		Env: m.Env, EnvRemove: m.EnvRemove, DryRun: m.DryRun,
	}
	if m.Workdir != nil {
		o.Workdir = *m.Workdir
	}
	plan, err := h.Modify(ctx, o)
	if err != nil {
		return Plan{}, mapErr(err)
	}
	p := Plan{Applied: plan.Applied, Changes: []string{}, Conflicts: []string{}, Warnings: []string{}}
	for _, c := range plan.Changes {
		p.Changes = append(p.Changes, fmt.Sprintf("%s: %s -> %s (%s)", c.Field, deref(c.Before), deref(c.After), c.Disposition))
	}
	for _, c := range plan.Conflicts {
		p.Conflicts = append(p.Conflicts, c.Field+": "+c.Message)
	}
	for _, w := range plan.Warnings {
		p.Warnings = append(p.Warnings, w.Field+": "+w.Message)
	}
	return p, nil
}

func deref(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

func (*SDK) Start(ctx context.Context, name string) error {
	sb, err := msb.StartSandboxDetached(ctx, name)
	if err != nil {
		return mapErr(err)
	}
	return sb.Detach(ctx)
}

func (*SDK) Stop(ctx context.Context, name string) error {
	h, err := msb.GetSandbox(ctx, name)
	if err != nil {
		return mapErr(err)
	}
	return mapErr(h.Stop(ctx))
}

func (*SDK) Remove(ctx context.Context, name string) error {
	return mapErr(msb.RemoveSandbox(ctx, name))
}

func (*SDK) Metrics(ctx context.Context) (map[string]Metrics, error) {
	all, err := msb.AllSandboxMetrics(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(map[string]Metrics, len(all))
	for name, m := range all {
		out[name] = Metrics{
			CPUPercent: m.CPUPercent, MemoryBytes: m.MemoryBytes, MemoryLimitBytes: m.MemoryLimitBytes,
			DiskReadBytes: m.DiskReadBytes, DiskWriteBytes: m.DiskWriteBytes,
			NetRxBytes: m.NetRxBytes, NetTxBytes: m.NetTxBytes,
			UptimeSeconds: m.Uptime.Seconds(),
		}
	}
	return out, nil
}
