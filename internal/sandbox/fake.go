package sandbox

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// Fake is an in-memory Backend used for tests and for running the UI on
// machines without KVM or the Microsandbox runtime.
type Fake struct {
	mu      sync.Mutex
	boxes   map[string]*Sandbox
	started map[string]time.Time
}

func NewFake() *Fake {
	return &Fake{boxes: map[string]*Sandbox{}, started: map[string]time.Time{}}
}

// NewDemoFake returns a Fake pre-populated with two sandboxes.
func NewDemoFake() *Fake {
	f := NewFake()
	ctx := context.Background()
	_, _ = f.Create(ctx, CreateSpec{
		Name: "demo-node", Image: "node:24-bookworm-slim", CPUs: 2, MemoryMiB: 4096,
		Workdir: "/work", Mounts: []MountSpec{{HostPath: "/home/user/project", GuestPath: "/work"}},
	})
	_, _ = f.Create(ctx, CreateSpec{Name: "demo-python", Image: "python:3.13-slim", CPUs: 1, MemoryMiB: 512})
	_ = f.Start(ctx, "demo-node")
	return f
}

func (f *Fake) List(_ context.Context) ([]Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Sandbox, 0, len(f.boxes))
	for _, b := range f.boxes {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *Fake) Get(_ context.Context, name string) (Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.boxes[name]
	if !ok {
		return Sandbox{}, ErrNotFound
	}
	return *b, nil
}

func (f *Fake) Create(_ context.Context, s CreateSpec) (Sandbox, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.boxes[s.Name]; ok {
		return Sandbox{}, ErrExists
	}
	b := &Sandbox{
		Name: s.Name, Image: s.Image, Status: "stopped", CPUs: s.CPUs, MemoryMiB: s.MemoryMiB,
		Workdir: s.Workdir, Env: s.Env, Mounts: []Mount{}, CreatedAt: time.Now(),
	}
	if b.Env == nil {
		b.Env = map[string]string{}
	}
	for _, m := range s.Mounts {
		b.Mounts = append(b.Mounts, Mount{GuestPath: m.GuestPath, Kind: "bind", Source: m.HostPath, Readonly: m.Readonly})
	}
	f.boxes[s.Name] = b
	return *b, nil
}

func (f *Fake) Modify(_ context.Context, name string, s ModifySpec) (Plan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.boxes[name]
	if !ok {
		return Plan{}, ErrNotFound
	}
	p := Plan{Changes: []string{}, Conflicts: []string{}, Warnings: []string{}}
	if s.CPUs != 0 && s.CPUs != b.CPUs {
		p.Changes = append(p.Changes, fmt.Sprintf("cpus: %d -> %d", b.CPUs, s.CPUs))
	}
	if s.MemoryMiB != 0 && s.MemoryMiB != b.MemoryMiB {
		p.Changes = append(p.Changes, fmt.Sprintf("memoryMiB: %d -> %d", b.MemoryMiB, s.MemoryMiB))
	}
	if s.Workdir != nil && *s.Workdir != b.Workdir {
		p.Changes = append(p.Changes, fmt.Sprintf("workdir: %q -> %q", b.Workdir, *s.Workdir))
	}
	for k := range s.Env {
		p.Changes = append(p.Changes, "env set: "+k)
	}
	for _, k := range s.EnvRemove {
		p.Changes = append(p.Changes, "env remove: "+k)
	}
	if s.DryRun {
		return p, nil
	}
	if s.CPUs != 0 {
		b.CPUs = s.CPUs
	}
	if s.MemoryMiB != 0 {
		b.MemoryMiB = s.MemoryMiB
	}
	if s.Workdir != nil {
		b.Workdir = *s.Workdir
	}
	for k, v := range s.Env {
		b.Env[k] = v
	}
	for _, k := range s.EnvRemove {
		delete(b.Env, k)
	}
	p.Applied = true
	return p, nil
}

func (f *Fake) Start(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.boxes[name]
	if !ok {
		return ErrNotFound
	}
	b.Status = "running"
	f.started[name] = time.Now()
	return nil
}

func (f *Fake) Stop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.boxes[name]
	if !ok {
		return ErrNotFound
	}
	b.Status = "stopped"
	delete(f.started, name)
	return nil
}

func (f *Fake) Remove(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.boxes[name]
	if !ok {
		return ErrNotFound
	}
	if b.Status == "running" {
		return ErrRunning
	}
	delete(f.boxes, name)
	return nil
}

func (f *Fake) Metrics(_ context.Context) (map[string]Metrics, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]Metrics{}
	for name, t := range f.started {
		b := f.boxes[name]
		limit := float64(b.MemoryMiB) * (1 << 20)
		up := time.Since(t).Seconds()
		// Random values are demo data only, not security sensitive.
		out[name] = Metrics{
			CPUPercent:       rand.Float64() * 60,                      // #nosec G404 -- demo data
			MemoryBytes:      uint64(limit/4 + rand.Float64()*limit/8), // #nosec G404 -- demo data
			MemoryLimitBytes: uint64(limit),
			DiskReadBytes:    uint64(up * 1e5),
			DiskWriteBytes:   uint64(up * 5e4),
			NetRxBytes:       uint64(up * 2e4),
			NetTxBytes:       uint64(up * 1e4),
			UptimeSeconds:    up,
		}
	}
	return out, nil
}
