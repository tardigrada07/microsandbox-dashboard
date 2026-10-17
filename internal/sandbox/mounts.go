package sandbox

import (
	"encoding/json"
	"sort"
	"strings"
)

// storedMount mirrors the persisted Microsandbox config: a "mounts" array of
// objects tagged by "type" (Bind, Named, Tmpfs, DiskImage, Owned). The Go SDK's
// SandboxConfig does not decode this format, so we parse ConfigJSON ourselves.
type storedMount struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Guest    string `json:"guest"`
	Name     string `json:"name"`
	Readonly bool   `json:"readonly"`
	Options  *struct {
		Readonly bool `json:"readonly"`
	} `json:"options"`
}

// ParseStoredMounts extracts mounts from a sandbox's persisted config JSON and
// returns them sorted by guest path. A config without a "mounts" key yields an
// empty list, not an error.
func ParseStoredMounts(configJSON string) ([]Mount, error) {
	var cfg struct {
		Mounts []storedMount `json:"mounts"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, err
	}
	out := make([]Mount, 0, len(cfg.Mounts))
	for _, m := range cfg.Mounts {
		mt := Mount{GuestPath: m.Guest, Kind: strings.ToLower(m.Type), Readonly: m.Readonly}
		if m.Options != nil && m.Options.Readonly {
			mt.Readonly = true
		}
		switch mt.Kind {
		case "bind", "diskimage":
			mt.Source = m.Host
			if mt.Kind == "diskimage" {
				mt.Kind = "disk"
			}
		case "named":
			mt.Source = m.Name
		}
		out = append(out, mt)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GuestPath < out[j].GuestPath })
	return out, nil
}
