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
	"os/signal"
	"path/filepath"
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

	shutErr := lifecycle.WaitFinalizeThenShutdown(ctx, sess.Done(), httpServer, lifecycle.Options{
		Linger:          time.Duration(cfg.Server.LingerSeconds) * time.Second,
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
