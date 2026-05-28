package gallery

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// tinyPNG returns 8-byte PNG signature + a couple of trailing bytes so the
// file is "valid enough" for the signature check.
var tinyPNG = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}

func writeFile(t *testing.T, dir, rel string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestScan_ReturnsItemsForMatchingPNGs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shots/home-en-dark.png", tinyPNG)
	writeFile(t, dir, "shots/home-pt-light.png", tinyPNG)
	writeFile(t, dir, "shots/README.md", []byte("ignore me"))
	writeFile(t, dir, "elsewhere/skipped.png", tinyPNG)

	s := &Scanner{
		RepoDir:   dir,
		Globs:     []string{"shots/*.png"},
		AxisRegex: regexp.MustCompile(`(?P<scene>[^/-]+)-(?P<locale>[^/-]+)-(?P<theme>[^/.]+)\.png`),
	}
	items, warnings, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d (%v)", len(items), items)
	}
	wantPaths := []string{"shots/home-en-dark.png", "shots/home-pt-light.png"}
	gotPaths := []string{items[0].Path, items[1].Path}
	sort.Strings(gotPaths)
	for i := range wantPaths {
		if gotPaths[i] != wantPaths[i] {
			t.Errorf("path[%d] = %q, want %q", i, gotPaths[i], wantPaths[i])
		}
	}
	if items[0].Axes["scene"] != "home" || items[0].Axes["locale"] != "en" || items[0].Axes["theme"] != "dark" {
		t.Errorf("axes for %s = %v", items[0].Path, items[0].Axes)
	}
}

func TestScan_WarnsOnNonPNGInGlob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shots/legit.png", tinyPNG)
	writeFile(t, dir, "shots/imposter.png", []byte("not a png"))

	s := &Scanner{
		RepoDir: dir,
		Globs:   []string{"shots/*.png"},
	}
	items, warnings, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 1 || items[0].Path != "shots/legit.png" {
		t.Errorf("want only legit.png, got %v", items)
	}
	if len(warnings) != 1 {
		t.Errorf("want 1 warning, got %v", warnings)
	}
}

func TestScan_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".git/objects/cafe/bad.png", tinyPNG)
	writeFile(t, dir, "shots/keep.png", tinyPNG)

	s := &Scanner{
		RepoDir: dir,
		Globs:   []string{"**/*.png"},
	}
	items, _, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(items) != 1 || items[0].Path != "shots/keep.png" {
		t.Errorf("want only shots/keep.png, got %v", items)
	}
}

func TestScan_EmptyAxesWhenRegexUnsetOrNoMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "shots/weird-name.png", tinyPNG)

	// No regex: Axes == nil.
	s := &Scanner{RepoDir: dir, Globs: []string{"shots/*.png"}}
	items, _, err := s.Scan(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("Scan: err=%v items=%d", err, len(items))
	}
	if items[0].Axes != nil {
		t.Errorf("want nil Axes when no regex, got %v", items[0].Axes)
	}

	// Regex that doesn't match: Axes is an empty (but non-nil) map.
	s.AxisRegex = regexp.MustCompile(`(?P<a>x)-(?P<b>y)-(?P<c>z)\.png`)
	items, _, err = s.Scan(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("Scan: err=%v items=%d", err, len(items))
	}
	if items[0].Axes == nil || len(items[0].Axes) != 0 {
		t.Errorf("want empty (non-nil) Axes when regex misses, got %v", items[0].Axes)
	}
}

func TestIDFor_StableAndPathDependent(t *testing.T) {
	a := IDFor("shots/home-en-dark.png")
	b := IDFor("shots/home-en-dark.png")
	c := IDFor("shots/home-en-light.png")
	if a != b {
		t.Errorf("IDFor is not stable: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("IDFor collides on distinct paths: %q == %q", a, c)
	}
	if len(a) != 16 {
		t.Errorf("IDFor length = %d, want 16 (8 bytes hex)", len(a))
	}
}
