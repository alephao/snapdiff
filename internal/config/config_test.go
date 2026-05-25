package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "snapdiff.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write toml: %v", err)
	}
	return path
}

func TestLoad_parsesFullConfig(t *testing.T) {
	path := writeTOML(t, `
[snapshots]
globs = ["**/__Snapshots__/*.png", "ios/snaps/*.png"]
axis_regex = '(?P<test>[^/]+)__(?P<device>[^_]+)__(?P<theme>[^.]+)\.png'
base_ref = "origin/main"

[server]
bind = "127.0.0.1:9090"
linger_seconds = 30
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(cfg.Snapshots.Globs), 2; got != want {
		t.Errorf("globs len = %d, want %d", got, want)
	}
	if got, want := cfg.Snapshots.Globs[0], "**/__Snapshots__/*.png"; got != want {
		t.Errorf("globs[0] = %q, want %q", got, want)
	}
	if cfg.Snapshots.AxisRegex == nil {
		t.Fatal("AxisRegex is nil")
	}
	if got, want := cfg.Snapshots.BaseRef, "origin/main"; got != want {
		t.Errorf("BaseRef = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Bind, "127.0.0.1:9090"; got != want {
		t.Errorf("Bind = %q, want %q", got, want)
	}
	if got, want := cfg.Server.LingerSeconds, 30; got != want {
		t.Errorf("LingerSeconds = %d, want %d", got, want)
	}
}

func TestLoad_appliesDefaults(t *testing.T) {
	path := writeTOML(t, `
[snapshots]
globs = ["*.png"]
axis_regex = '(?P<test>[^.]+)\.png'
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.Snapshots.BaseRef, "HEAD"; got != want {
		t.Errorf("default BaseRef = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Bind, "0.0.0.0:7777"; got != want {
		t.Errorf("default Bind = %q, want %q", got, want)
	}
	if got, want := cfg.Server.LingerSeconds, 60; got != want {
		t.Errorf("default LingerSeconds = %d, want %d", got, want)
	}
}

func TestLoad_compilesAxisRegex(t *testing.T) {
	path := writeTOML(t, `
[snapshots]
globs = ["*.png"]
axis_regex = '(?P<test>[^/]+)__(?P<device>[^.]+)\.png'
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	names := cfg.Snapshots.AxisRegex.SubexpNames()
	got := map[string]bool{}
	for _, n := range names {
		if n != "" {
			got[n] = true
		}
	}
	for _, want := range []string{"test", "device"} {
		if !got[want] {
			t.Errorf("regex missing named group %q (got %v)", want, names)
		}
	}
}

func TestLoad_errorsOnMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_errorsOnEmptyGlobs(t *testing.T) {
	path := writeTOML(t, `
[snapshots]
globs = []
axis_regex = '(?P<test>.*)\.png'
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty globs")
	}
	if !strings.Contains(err.Error(), "globs") {
		t.Errorf("error %q should mention globs", err)
	}
}

func TestLoad_errorsOnMissingAxisRegex(t *testing.T) {
	path := writeTOML(t, `
[snapshots]
globs = ["*.png"]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing axis_regex")
	}
	if !strings.Contains(err.Error(), "axis_regex") {
		t.Errorf("error %q should mention axis_regex", err)
	}
}

func TestLoad_errorsOnInvalidAxisRegex(t *testing.T) {
	path := writeTOML(t, `
[snapshots]
globs = ["*.png"]
axis_regex = '(?P<test>[unclosed'
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "axis_regex") {
		t.Errorf("error %q should mention axis_regex", err)
	}
}

func TestLoad_errorsOnInvalidTOML(t *testing.T) {
	path := writeTOML(t, `this is not = valid = toml`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoad_errorsOnNegativeLinger(t *testing.T) {
	path := writeTOML(t, `
[snapshots]
globs = ["*.png"]
axis_regex = '(?P<test>.*)\.png'

[server]
linger_seconds = -1
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for negative linger_seconds")
	}
}
