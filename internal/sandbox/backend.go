// Package sandbox defines the dashboard's view of Microsandbox and the
// backends that implement it.
package sandbox

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("sandbox not found")
	ErrExists   = errors.New("sandbox already exists")
	ErrRunning  = errors.New("sandbox is running; stop it first")
	ErrInvalid  = errors.New("invalid request")
)

// Mount is one mounted directory or volume inside a sandbox.
type Mount struct {
	GuestPath string `json:"guestPath"`
	Kind      string `json:"kind"` // bind, named, tmpfs, disk
	Source    string `json:"source"`
	Readonly  bool   `json:"readonly"`
}

// Sandbox is the state of one sandbox.
type Sandbox struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Status    string            `json:"status"`
	CPUs      int               `json:"cpus"`
	MemoryMiB int               `json:"memoryMiB"`
	Workdir   string            `json:"workdir"`
	Env       map[string]string `json:"env"`
	Mounts    []Mount           `json:"mounts"`
	CreatedAt time.Time         `json:"createdAt"`
}

// Metrics is a resource usage snapshot.
type Metrics struct {
	CPUPercent       float64 `json:"cpuPercent"`
	MemoryBytes      uint64  `json:"memoryBytes"`
	MemoryLimitBytes uint64  `json:"memoryLimitBytes"`
	DiskReadBytes    uint64  `json:"diskReadBytes"`
	DiskWriteBytes   uint64  `json:"diskWriteBytes"`
	NetRxBytes       uint64  `json:"netRxBytes"`
	NetTxBytes       uint64  `json:"netTxBytes"`
	UptimeSeconds    float64 `json:"uptimeSeconds"`
}

// MountSpec asks for a host directory to be bind-mounted.
type MountSpec struct {
	HostPath  string `json:"hostPath"`
	GuestPath string `json:"guestPath"`
	Readonly  bool   `json:"readonly"`
}

// CreateSpec describes a new sandbox.
type CreateSpec struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	CPUs      int               `json:"cpus"`
	MemoryMiB int               `json:"memoryMiB"`
	Workdir   string            `json:"workdir"`
	Env       map[string]string `json:"env"`
	Mounts    []MountSpec       `json:"mounts"`
}

// ModifySpec is a partial update. Zero values mean "leave unchanged".
type ModifySpec struct {
	CPUs      int               `json:"cpus"`
	MemoryMiB int               `json:"memoryMiB"`
	Workdir   *string           `json:"workdir"`
	Env       map[string]string `json:"env"`
	EnvRemove []string          `json:"envRemove"`
	DryRun    bool              `json:"dryRun"`
}

// Plan reports what a Modify call changes (or would change, for a dry run).
type Plan struct {
	Applied   bool     `json:"applied"`
	Changes   []string `json:"changes"`
	Conflicts []string `json:"conflicts"`
	Warnings  []string `json:"warnings"`
}

// Backend is everything the HTTP layer needs from Microsandbox.
type Backend interface {
	List(ctx context.Context) ([]Sandbox, error)
	Get(ctx context.Context, name string) (Sandbox, error)
	Create(ctx context.Context, spec CreateSpec) (Sandbox, error)
	Modify(ctx context.Context, name string, spec ModifySpec) (Plan, error)
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	// Metrics returns snapshots for all running sandboxes, keyed by name.
	Metrics(ctx context.Context) (map[string]Metrics, error)
}
