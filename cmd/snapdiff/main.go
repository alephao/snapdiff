// snapdiff is a human-in-the-loop review tool for agent-driven screenshot
// test workflows. See docs/spec.md for the system design.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/alephao/snapdiff/internal/apply"
	"github.com/alephao/snapdiff/internal/config"
	"github.com/alephao/snapdiff/internal/gitscan"
	"github.com/alephao/snapdiff/internal/imdiff"
	"github.com/alephao/snapdiff/internal/lifecycle"
	"github.com/alephao/snapdiff/internal/review"
	"github.com/alephao/snapdiff/internal/web"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `snapdiff — review screenshot-test diffs from an agent loop

Usage:
  snapdiff await         block until the reviewer finalizes; emit verdict JSON
  snapdiff serve         start the UI ad-hoc (no verdict JSON output)
  snapdiff version       print the version

Common flags:
  --repo <dir>           repo directory (default: cwd)
  --config <path>        path to snapdiff.toml (default: <repo>/snapdiff.toml)

See docs/spec.md and docs/adr/ for the design and decision log.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "snapdiff:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return errors.New("missing subcommand")
	}
	switch args[0] {
	case "await":
		return runDaemon(args[1:], true /*emitJSON*/)
	case "serve":
		return runDaemon(args[1:], false)
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	case "help", "--help", "-h":
		fmt.Print(usage)
		return nil
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runDaemon(args []string, emitJSON bool) error {
	fs := flag.NewFlagSet("snapdiff", flag.ContinueOnError)
	repoFlag := fs.String("repo", "", "repo directory (default: cwd)")
	cfgFlag := fs.String("config", "", "path to snapdiff.toml (default: <repo>/snapdiff.toml)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repoDir := *repoFlag
	if repoDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getwd: %w", err)
		}
		repoDir = cwd
	}
	cfgPath := *cfgFlag
	if cfgPath == "" {
		cfgPath = filepath.Join(repoDir, "snapdiff.toml")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	scanner := &gitscan.Scanner{
		RepoDir:   repoDir,
		BaseRef:   cfg.Snapshots.BaseRef,
		Globs:     cfg.Snapshots.Globs,
		AxisRegex: cfg.Snapshots.AxisRegex,
	}
	diffs, warnings, err := scanner.Scan(ctx)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "snapdiff: warning:", w)
	}

	// Empty-diff fast path: nothing to review, agent moves on.
	if len(diffs) == 0 {
		if emitJSON {
			return writeJSON(os.Stdout, apply.Result{Verdicts: []apply.FileVerdict{}})
		}
		fmt.Fprintln(os.Stderr, "snapdiff: no diffs to review.")
		return nil
	}

	sess := review.NewSession(diffs)
	differ := imdiff.New()

	var finalResult apply.Result
	onFinalize := func(items []*review.Item) error {
		r, err := apply.Apply(ctx, repoDir, cfg.Snapshots.BaseRef, items)
		if err != nil {
			return err
		}
		finalResult = r
		return nil
	}

	ln, err := net.Listen("tcp", cfg.Server.Bind)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Server.Bind, err)
	}
	publicURL := fmt.Sprintf("http://%s", ln.Addr().String())
	srv := web.NewServer(sess, differ, publicURL, onFinalize)
	srv.SetRepoLabel(filepath.Base(repoDir) + "/")
	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Fprintln(os.Stderr, "snapdiff: review at", publicURL)
	fmt.Fprintf(os.Stderr, "snapdiff: %d diff(s) pending.\n", len(diffs))

	serveErrCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
		}
		close(serveErrCh)
	}()

	// `await` is the agent-driven flow — pop the UI for the human reviewer as
	// soon as the listener is bound. `serve` is interactive already, so the
	// user opened the URL themselves. SNAPDIFF_NO_BROWSER suppresses it (used
	// by the acceptance test, and handy in headless contexts).
	if emitJSON && os.Getenv("SNAPDIFF_NO_BROWSER") == "" {
		if err := openBrowser(browserURL(ln.Addr())); err != nil {
			fmt.Fprintln(os.Stderr, "snapdiff: could not open browser:", err)
		}
	}

	// In `await` mode the agent is blocking on stdout, so we exit immediately
	// after finalize — no linger. `serve` mode keeps the configured linger so
	// the browser can render the success state.
	linger := time.Duration(cfg.Server.LingerSeconds) * time.Second
	if emitJSON {
		linger = 0
	}
	shutErr := lifecycle.WaitFinalizeThenShutdown(ctx, sess.Done(), httpServer, lifecycle.Options{
		Linger:          linger,
		ShutdownTimeout: 5 * time.Second,
	})
	if shutErr != nil && !errors.Is(shutErr, context.Canceled) && !errors.Is(shutErr, http.ErrServerClosed) {
		return shutErr
	}

	// Drain any late serve error.
	if err, ok := <-serveErrCh; ok && err != nil {
		return err
	}

	if errors.Is(shutErr, context.Canceled) {
		// User aborted (Ctrl-C). Do not emit verdicts; agent sees non-zero exit.
		return shutErr
	}

	if emitJSON {
		return writeJSON(os.Stdout, finalResult)
	}
	return nil
}

func writeJSON(w *os.File, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// browserURL converts a listener address into a URL safe for `open` /
// `xdg-open`. Wildcard binds (0.0.0.0, ::) are not reachable from a browser
// on every platform, so we point at the loopback instead.
func browserURL(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return fmt.Sprintf("http://%s", addr.String())
	}
	host := tcp.IP.String()
	if tcp.IP == nil || tcp.IP.IsUnspecified() {
		host = "127.0.0.1"
	} else if tcp.IP.To4() == nil {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%d", host, tcp.Port)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
