// Package api is the dashboard's HTTP layer. It only serves loopback clients.
package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const cookieName = "msb_dash"

// NewToken returns a random access token.
func NewToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CheckLoopbackAddr rejects listen addresses that are not loopback IPs
// (or "localhost"). The dashboard must never be reachable from the network.
func CheckLoopbackAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("refusing to listen on %q: only loopback addresses are allowed", host)
	}
	return nil
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// guard enforces: loopback Host header (DNS-rebinding defense), same-origin
// Origin on unsafe methods (CSRF), and the access token cookie.
func guard(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Referrer-Policy", "no-referrer") // the token is in the first URL
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			o := r.Header.Get("Origin")
			if o == "" {
				http.Error(w, "missing origin", http.StatusForbidden)
				return
			}
			u, err := url.Parse(o)
			if err != nil || u.Host != r.Host {
				http.Error(w, "cross-origin request refused", http.StatusForbidden)
				return
			}
		}
		// First visit: ?token=... sets the cookie and drops the token from the URL.
		if t := r.URL.Query().Get("token"); t != "" && r.Method == http.MethodGet {
			if tokenEqual(t, token) {
				// Not Secure: the dashboard is plain HTTP on loopback (Safari drops Secure cookies there).
				http.SetCookie(w, &http.Cookie{ // #nosec G124 -- HttpOnly + SameSite=Strict, loopback HTTP only
					Name: cookieName, Value: token, Path: "/",
					HttpOnly: true, SameSite: http.SameSiteStrictMode,
				})
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
		c, err := r.Cookie(cookieName)
		if err != nil || !tokenEqual(c.Value, token) {
			http.Error(w, "unauthorized: open the URL printed by the dashboard on start", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func tokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
