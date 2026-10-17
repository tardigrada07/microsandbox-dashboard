// Command microsandbox-dashboard serves a localhost-only web UI for managing
// Microsandbox sandboxes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"microsandbox-dashboard/internal/api"
	"microsandbox-dashboard/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8484", "listen address (loopback only)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser on start")
	flag.Parse()

	if err := api.CheckLoopbackAddr(*addr); err != nil {
		log.Fatal(err)
	}
	token, err := api.NewToken()
	if err != nil {
		log.Fatal(err)
	}
	backend, desc, err := newBackend(context.Background())
	if err != nil {
		log.Fatalf("backend: %v", err)
	}
	log.Print(desc)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Handler:           (&api.Server{Backend: backend, Token: token, Web: web.FS}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	url := fmt.Sprintf("http://%s/?token=%s", ln.Addr(), token)
	fmt.Printf("Dashboard: %s\n", url)
	if !*noBrowser {
		openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// openBrowser opens our own dashboard URL (loopback address + random token);
// no user-supplied input reaches the command line.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url) // #nosec G204 -- URL built from our listener and token
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url) // #nosec G204 -- same
	default:
		cmd = exec.Command("xdg-open", url) // #nosec G204 -- same
	}
	_ = cmd.Start()
}
