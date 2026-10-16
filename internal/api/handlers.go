package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"microsandbox-dashboard/internal/sandbox"
)

var (
	nameRe  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
	imageRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_./:@-]{0,254}$`)
	envRe   = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,127}$`)
)

// Server serves the REST API and the embedded UI.
type Server struct {
	Backend sandbox.Backend
	Token   string
	Web     fs.FS
	// MetricsInterval is the SSE polling period (default 2s).
	MetricsInterval time.Duration
}

// Handler returns the full guarded handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sandboxes", s.list)
	mux.HandleFunc("POST /api/sandboxes", s.create)
	mux.HandleFunc("GET /api/sandboxes/{name}", s.get)
	mux.HandleFunc("PATCH /api/sandboxes/{name}", s.modify)
	mux.HandleFunc("DELETE /api/sandboxes/{name}", s.remove)
	mux.HandleFunc("POST /api/sandboxes/{name}/start", s.start)
	mux.HandleFunc("POST /api/sandboxes/{name}/stop", s.stop)
	mux.HandleFunc("GET /api/metrics", s.metrics)
	mux.HandleFunc("GET /api/metrics/stream", s.metricsStream)
	if s.Web != nil {
		mux.Handle("GET /", http.FileServerFS(s.Web))
	}
	return guard(s.Token, mux)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, sandbox.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, sandbox.ErrExists), errors.Is(err, sandbox.ErrRunning):
		code = http.StatusConflict
	case errors.Is(err, sandbox.ErrInvalid):
		code = http.StatusBadRequest
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", sandbox.ErrInvalid, fmt.Sprintf(format, a...))
}

func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return invalid("bad JSON body: %v", err)
	}
	return nil
}

func validAbs(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.ContainsRune(p, 0) && path.Clean(p) == p || p == "/"
}

func validateEnv(env map[string]string) error {
	for k := range env {
		if !envRe.MatchString(k) {
			return invalid("bad env name %q", k)
		}
	}
	return nil
}

func validateResources(cpus, mem int, required bool) error {
	if required || cpus != 0 {
		if cpus < 1 || cpus > 64 {
			return invalid("cpus must be 1-64")
		}
	}
	if required || mem != 0 {
		if mem < 128 || mem > 1<<20 {
			return invalid("memoryMiB must be 128-1048576")
		}
	}
	return nil
}

func validateCreate(c *sandbox.CreateSpec) error {
	if !nameRe.MatchString(c.Name) {
		return invalid("name must match %s", nameRe)
	}
	if !imageRe.MatchString(c.Image) {
		return invalid("bad image reference")
	}
	if err := validateResources(c.CPUs, c.MemoryMiB, true); err != nil {
		return err
	}
	if c.Workdir != "" && !validAbs(c.Workdir) {
		return invalid("workdir must be a clean absolute path")
	}
	if err := validateEnv(c.Env); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, m := range c.Mounts {
		if !validAbs(m.HostPath) || !validAbs(m.GuestPath) {
			return invalid("mount paths must be clean absolute paths")
		}
		if seen[m.GuestPath] {
			return invalid("duplicate guest path %q", m.GuestPath)
		}
		seen[m.GuestPath] = true
	}
	return nil
}

func validateModify(m *sandbox.ModifySpec) error {
	if err := validateResources(m.CPUs, m.MemoryMiB, false); err != nil {
		return err
	}
	if m.Workdir != nil && *m.Workdir != "" && !validAbs(*m.Workdir) {
		return invalid("workdir must be a clean absolute path")
	}
	if err := validateEnv(m.Env); err != nil {
		return err
	}
	for _, k := range m.EnvRemove {
		if !envRe.MatchString(k) {
			return invalid("bad env name %q", k)
		}
	}
	return nil
}

func checkName(r *http.Request) (string, error) {
	n := r.PathValue("name")
	if !nameRe.MatchString(n) {
		return "", invalid("bad sandbox name")
	}
	return n, nil
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	l, err := s.Backend.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	n, err := checkName(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.Backend.Get(r.Context(), n)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var c sandbox.CreateSpec
	if err := decode(r, &c); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateCreate(&c); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.Backend.Create(r.Context(), c)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) modify(w http.ResponseWriter, r *http.Request) {
	n, err := checkName(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	var m sandbox.ModifySpec
	if err := decode(r, &m); err != nil {
		writeErr(w, err)
		return
	}
	if err := validateModify(&m); err != nil {
		writeErr(w, err)
		return
	}
	p, err := s.Backend.Modify(r.Context(), n, m)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) lifecycle(w http.ResponseWriter, r *http.Request, f func(context.Context, string) error, code int) {
	n, err := checkName(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := f(r.Context(), n); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(code)
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	s.lifecycle(w, r, s.Backend.Start, http.StatusNoContent)
}
func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	s.lifecycle(w, r, s.Backend.Stop, http.StatusNoContent)
}
func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	s.lifecycle(w, r, s.Backend.Remove, http.StatusNoContent)
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	m, err := s.Backend.Metrics(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// metricsStream pushes the metrics map of all running sandboxes as SSE.
func (s *Server) metricsStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	iv := s.MetricsInterval
	if iv <= 0 {
		iv = 2 * time.Second
	}
	t := time.NewTicker(iv)
	defer t.Stop()
	for {
		m, err := s.Backend.Metrics(r.Context())
		if err == nil {
			b, _ := json.Marshal(m)
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
		}
	}
}
