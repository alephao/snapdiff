//go:build acceptance

// Package acceptance is the V-Model "north-star" test: it drives the real
// snapdiff binary against a fixture git repo via HTTP (no browser) and
// asserts the working-tree side effects + stdout JSON contract from
// docs/spec.md § Data Flow.
//
// Run with: `make acceptance` (or `go test -tags acceptance ./test/acceptance/...`).
package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcceptance_endToEnd(t *testing.T) {
	repo, paths := setupFixtureRepo(t)
	binary := buildSnapdiff(t)

	cmd := exec.Command(binary, "await",
		"--repo", repo,
		"--config", filepath.Join(repo, "snapdiff.toml"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start snapdiff: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	url := waitForURL(t, &stderr, 10*time.Second)
	t.Logf("review url: %s", url)

	// gitscan returns diffs path-sorted, so:
	//   id=0  ->  snaps/Added__iphone17__light.png
	//   id=1  ->  snaps/Modified__iphone17__dark.png

	// 1. Reject the "added" snapshot with a comment.
	postForm(t, url+"/diff/0/verdict", map[string]string{
		"status":  "rejected",
		"comment": "rgb wrong",
	})

	// 2. Approve the "modified" snapshot.
	postForm(t, url+"/diff/1/verdict", map[string]string{"status": "approved"})

	// 3. Finalize.
	postForm(t, url+"/session/finalize", nil)

	// Wait for `snapdiff await` to exit (linger=0 in fixture config, so fast).
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("snapdiff await exited with error: %v\nstderr=%s", err, stderr.String())
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("snapdiff await did not exit within 10s\nstderr=%s", stderr.String())
	}

	// Assert stdout JSON contract.
	var result struct {
		Verdicts []struct {
			Path    string `json:"path"`
			Status  string `json:"status"`
			Comment string `json:"comment,omitempty"`
		} `json:"verdicts"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode stdout JSON: %v\nstdout=%q", err, stdout.String())
	}
	if got, want := len(result.Verdicts), 2; got != want {
		t.Fatalf("verdicts len = %d, want %d (json=%s)", got, want, stdout.String())
	}
	if result.Verdicts[0].Path != paths.added || result.Verdicts[0].Status != "rejected" || result.Verdicts[0].Comment != "rgb wrong" {
		t.Errorf("verdict[0] = %+v", result.Verdicts[0])
	}
	if result.Verdicts[1].Path != paths.modified || result.Verdicts[1].Status != "approved" || result.Verdicts[1].Comment != "" {
		t.Errorf("verdict[1] = %+v", result.Verdicts[1])
	}

	// Assert git side effects:
	//  - approved modified: file still differs from HEAD (dirty).
	//  - rejected added: file is GONE from the working tree.
	if !isDirty(t, repo, paths.modified) {
		t.Errorf("approved-modified should still be dirty in working tree")
	}
	if _, err := os.Stat(filepath.Join(repo, paths.added)); !os.IsNotExist(err) {
		t.Errorf("rejected-added should be removed; stat err=%v", err)
	}
}

// ---------- helpers ----------

type fixturePaths struct {
	modified string
	added    string
}

// setupFixtureRepo creates a temp git repo with one committed snapshot
// that we'll then "modify", plus an untracked file we'll register as
// "added". Returns the repo dir and the relative paths.
func setupFixtureRepo(t *testing.T) (string, fixturePaths) {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	modifiedRel := "snaps/Modified__iphone17__dark.png"
	addedRel := "snaps/Added__iphone17__light.png"

	writeImage(t, dir, modifiedRel, color.RGBA{255, 255, 255, 255})
	commitAll(t, dir, "init")

	// Now overwrite the modified file with a different pixel.
	writeImage(t, dir, modifiedRel, color.RGBA{0, 0, 0, 255})
	// And drop a brand-new untracked PNG.
	writeImage(t, dir, addedRel, color.RGBA{255, 0, 0, 255})

	// Write a snapdiff.toml with linger=0 so the test exits quickly.
	cfg := `
[snapshots]
globs = ["snaps/*.png"]
axis_regex = '(?P<test>[^/_]+)__(?P<device>[^/_]+)__(?P<theme>[^/.]+)\.png'

[server]
bind = "127.0.0.1:0"
linger_seconds = 0
`
	if err := os.WriteFile(filepath.Join(dir, "snapdiff.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, fixturePaths{modified: modifiedRel, added: addedRel}
}

func writeImage(t *testing.T, dir, rel string, c color.RGBA) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(full)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-q", "-m", msg},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func isDirty(t *testing.T, dir, path string) bool {
	t.Helper()
	cmd := exec.Command("git", "diff", "--quiet", "HEAD", "--", path)
	cmd.Dir = dir
	err := cmd.Run()
	// `git diff --quiet` exits 1 when there are differences.
	return err != nil
}

// buildSnapdiff compiles the binary into the temp dir and returns its path.
func buildSnapdiff(t *testing.T) string {
	t.Helper()
	repoRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	root := strings.TrimSpace(string(repoRoot))
	bin := filepath.Join(t.TempDir(), "snapdiff-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/snapdiff")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// waitForURL polls stderr for the "review at <url>" line, then probes /healthz.
func waitForURL(t *testing.T, stderr *bytes.Buffer, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if u := parseURLFromStderr(stderr.String()); u != "" {
			if probeHealth(u) {
				return u
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("did not see review URL in stderr within %v\nstderr=%q", timeout, stderr.String())
	return ""
}

func parseURLFromStderr(s string) string {
	const tag = "review at "
	idx := strings.Index(s, tag)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(tag):]
	end := strings.IndexAny(rest, "\n\r ")
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func probeHealth(u string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", u+"/healthz", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == 200
}

func postForm(t *testing.T, target string, vals map[string]string) {
	t.Helper()
	form := url.Values{}
	for k, v := range vals {
		form.Set(k, v)
	}
	resp, err := http.PostForm(target, form)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s: status=%d body=%s", target, resp.StatusCode, string(body))
	}
	if _, err := fmt.Fprint(io.Discard, body); err != nil {
		t.Fatal(err)
	}
}
