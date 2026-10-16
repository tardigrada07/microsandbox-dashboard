package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"microsandbox-dashboard/internal/sandbox"
)

const tok = "secret"

func newSrv() http.Handler {
	return (&Server{Backend: sandbox.NewFake(), Token: tok}).Handler()
}

func do(h http.Handler, method, target, body string, mut func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host = "127.0.0.1:8484"
	if method != http.MethodGet {
		r.Header.Set("Origin", "http://127.0.0.1:8484")
		r.Header.Set("Content-Type", "application/json")
	}
	r.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	if mut != nil {
		mut(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestCheckLoopbackAddr(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:1": true, "[::1]:1": true, "localhost:1": true,
		"0.0.0.0:1": false, ":1": false, "192.168.1.5:1": false, "example.com:1": false,
	} {
		if err := CheckLoopbackAddr(addr); (err == nil) != ok {
			t.Errorf("%s: err=%v, want ok=%v", addr, err, ok)
		}
	}
}

func TestGuard(t *testing.T) {
	h := newSrv()
	cases := []struct {
		name string
		mut  func(*http.Request)
		code int
	}{
		{"ok", nil, 200},
		{"foreign host", func(r *http.Request) { r.Host = "evil.example.com" }, 403},
		{"no cookie", func(r *http.Request) { r.Header.Del("Cookie") }, 401},
		{"wrong cookie", func(r *http.Request) { r.Header.Set("Cookie", cookieName+"=nope") }, 401},
	}
	for _, c := range cases {
		if w := do(h, "GET", "/api/sandboxes", "", c.mut); w.Code != c.code {
			t.Errorf("%s: got %d want %d", c.name, w.Code, c.code)
		}
	}
	// Unsafe methods need a same-origin Origin header.
	body := `{"name":"a","image":"x","cpus":1,"memoryMiB":256}`
	if w := do(h, "POST", "/api/sandboxes", body, func(r *http.Request) { r.Header.Set("Origin", "http://evil.example.com") }); w.Code != 403 {
		t.Errorf("cross-origin POST: got %d", w.Code)
	}
	if w := do(h, "POST", "/api/sandboxes", body, func(r *http.Request) { r.Header.Del("Origin") }); w.Code != 403 {
		t.Errorf("origin-less POST: got %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	w := do(newSrv(), "GET", "/api/sandboxes", "", nil)
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'self'") ||
		w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing security headers: %v", w.Header())
	}
}

func TestTokenExchange(t *testing.T) {
	h := newSrv()
	w := do(h, "GET", "/?token="+tok, "", func(r *http.Request) { r.Header.Del("Cookie") })
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Set-Cookie"), cookieName+"="+tok) {
		t.Fatalf("got %d, cookie %q", w.Code, w.Header().Get("Set-Cookie"))
	}
	if w := do(h, "GET", "/?token=bad", "", func(r *http.Request) { r.Header.Del("Cookie") }); w.Code != 401 {
		t.Errorf("bad token: got %d", w.Code)
	}
}

func TestLifecycle(t *testing.T) {
	h := newSrv()
	create := `{"name":"box1","image":"node:24","cpus":2,"memoryMiB":1024,"mounts":[{"hostPath":"/a","guestPath":"/b"}]}`
	if w := do(h, "POST", "/api/sandboxes", create, nil); w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if w := do(h, "POST", "/api/sandboxes", create, nil); w.Code != 409 {
		t.Errorf("duplicate create: %d", w.Code)
	}
	if w := do(h, "POST", "/api/sandboxes/box1/start", "", nil); w.Code != 204 {
		t.Fatalf("start: %d", w.Code)
	}
	if w := do(h, "DELETE", "/api/sandboxes/box1", "", nil); w.Code != 409 {
		t.Errorf("delete running: %d", w.Code)
	}
	if w := do(h, "GET", "/api/metrics", "", nil); !strings.Contains(w.Body.String(), "box1") {
		t.Errorf("metrics missing box1: %s", w.Body)
	}
	// dry run does not apply
	if w := do(h, "PATCH", "/api/sandboxes/box1", `{"cpus":4,"dryRun":true}`, nil); w.Code != 200 || !strings.Contains(w.Body.String(), "cpus: 2 -> 4") {
		t.Errorf("dry run: %d %s", w.Code, w.Body)
	}
	if w := do(h, "GET", "/api/sandboxes/box1", "", nil); !strings.Contains(w.Body.String(), `"cpus":2`) {
		t.Errorf("dry run changed state: %s", w.Body)
	}
	if w := do(h, "PATCH", "/api/sandboxes/box1", `{"cpus":4}`, nil); w.Code != 200 {
		t.Errorf("modify: %d", w.Code)
	}
	if w := do(h, "GET", "/api/sandboxes/box1", "", nil); !strings.Contains(w.Body.String(), `"cpus":4`) || !strings.Contains(w.Body.String(), `"source":"/a"`) {
		t.Errorf("after modify: %s", w.Body)
	}
	do(h, "POST", "/api/sandboxes/box1/stop", "", nil)
	if w := do(h, "DELETE", "/api/sandboxes/box1", "", nil); w.Code != 204 {
		t.Errorf("delete: %d", w.Code)
	}
	if w := do(h, "GET", "/api/sandboxes/box1", "", nil); w.Code != 404 {
		t.Errorf("get deleted: %d", w.Code)
	}
}

func TestValidation(t *testing.T) {
	h := newSrv()
	bad := []string{
		`{"name":"../x","image":"i","cpus":1,"memoryMiB":256}`,
		`{"name":"a","image":"bad image","cpus":1,"memoryMiB":256}`,
		`{"name":"a","image":"i","cpus":0,"memoryMiB":256}`,
		`{"name":"a","image":"i","cpus":1,"memoryMiB":1}`,
		`{"name":"a","image":"i","cpus":1,"memoryMiB":256,"workdir":"rel"}`,
		`{"name":"a","image":"i","cpus":1,"memoryMiB":256,"env":{"1BAD":"x"}}`,
		`{"name":"a","image":"i","cpus":1,"memoryMiB":256,"mounts":[{"hostPath":"rel","guestPath":"/b"}]}`,
		`{"name":"a","image":"i","cpus":1,"memoryMiB":256,"mounts":[{"hostPath":"/a/../b","guestPath":"/b"}]}`,
		`{"name":"a","image":"i","cpus":1,"memoryMiB":256,"extra":1}`,
		`not json`,
	}
	for _, b := range bad {
		if w := do(h, "POST", "/api/sandboxes", b, nil); w.Code != 400 {
			t.Errorf("%s: got %d", b, w.Code)
		}
	}
}
