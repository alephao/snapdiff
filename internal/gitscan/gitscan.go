// Package gitscan enumerates snapshot diffs in a git repo: files matching
// configured globs whose content differs between a base ref and the working
// tree (or which are untracked / deleted).
package gitscan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Status describes the kind of change for a snapshot file.
type Status int

const (
	StatusModified Status = iota
	StatusAdded
	StatusDeleted
)

func (s Status) String() string {
	switch s {
	case StatusModified:
		return "modified"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	default:
		return fmt.Sprintf("status(%d)", int(s))
	}
}

// Diff is one snapshot file that differs between BaseRef and the working tree.
type Diff struct {
	Path     string            // relative to repo root, slash-separated
	Status   Status            // Added | Modified | Deleted
	Baseline []byte            // empty for Added
	Current  []byte            // empty for Deleted
	Axes     map[string]string // from AxisRegex named groups; empty if no match
}

// Scanner enumerates Diffs for a given configuration.
type Scanner struct {
	RepoDir   string
	BaseRef   string
	Globs     []string
	AxisRegex *regexp.Regexp
}

// pngSignature is the 8-byte prefix every PNG file starts with.
var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

func isPNG(b []byte) bool {
	return len(b) >= len(pngSignature) && bytes.Equal(b[:len(pngSignature)], pngSignature)
}

// Scan returns the diffs, plus a list of human-readable warnings about
// files that were ignored.
func (s *Scanner) Scan(ctx context.Context) ([]Diff, []string, error) {
	if _, err := s.runGit(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, nil, fmt.Errorf("%s is not inside a git work tree: %w", s.RepoDir, err)
	}

	tracked, err := s.diffNameStatus(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("git diff: %w", err)
	}
	untracked, err := s.untrackedFiles(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("git ls-files: %w", err)
	}

	type candidate struct {
		path   string
		status Status
	}
	var cands []candidate
	for _, c := range tracked {
		if !s.matchesAnyGlob(c.path) {
			continue
		}
		cands = append(cands, candidate{c.path, c.status})
	}
	for _, p := range untracked {
		if !s.matchesAnyGlob(p) {
			continue
		}
		cands = append(cands, candidate{p, StatusAdded})
	}

	var diffs []Diff
	var warnings []string
	for _, c := range cands {
		var baseline, current []byte
		var err error

		if c.status != StatusAdded {
			baseline, err = s.show(ctx, c.path)
			if err != nil {
				return nil, nil, fmt.Errorf("git show %s: %w", c.path, err)
			}
		}
		if c.status != StatusDeleted {
			current, err = os.ReadFile(filepath.Join(s.RepoDir, c.path))
			if err != nil {
				return nil, nil, fmt.Errorf("read %s: %w", c.path, err)
			}
		}

		// PNG check: the file is a snapshot iff at least one of its bytes
		// (baseline or current) is a PNG. If neither is, skip with a warning.
		ok := (len(baseline) > 0 && isPNG(baseline)) || (len(current) > 0 && isPNG(current))
		if !ok {
			warnings = append(warnings, fmt.Sprintf("ignoring non-PNG file in glob: %s", c.path))
			continue
		}

		diffs = append(diffs, Diff{
			Path:     c.path,
			Status:   c.status,
			Baseline: baseline,
			Current:  current,
			Axes:     s.extractAxes(c.path),
		})
	}

	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Path < diffs[j].Path })
	return diffs, warnings, nil
}

func (s *Scanner) matchesAnyGlob(path string) bool {
	for _, g := range s.Globs {
		if ok, _ := doublestar.PathMatch(g, path); ok {
			return true
		}
	}
	return false
}

func (s *Scanner) extractAxes(path string) map[string]string {
	if s.AxisRegex == nil {
		return nil
	}
	m := s.AxisRegex.FindStringSubmatch(path)
	if m == nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for i, name := range s.AxisRegex.SubexpNames() {
		if name == "" {
			continue
		}
		out[name] = m[i]
	}
	return out
}

type trackedChange struct {
	path   string
	status Status
}

// diffNameStatus runs `git diff --name-status -z <base_ref>`.
func (s *Scanner) diffNameStatus(ctx context.Context) ([]trackedChange, error) {
	out, err := s.runGit(ctx, "diff", "--name-status", "-z", s.BaseRef)
	if err != nil {
		return nil, err
	}
	// -z output is NUL-separated tokens. For A/M/D entries this is:
	//   "M\0path\0M\0path\0..."
	// For R/C entries it's three tokens (status, old, new).
	tokens := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	var changes []trackedChange
	for i := 0; i < len(tokens); {
		if tokens[i] == "" {
			i++
			continue
		}
		code := tokens[i]
		switch {
		case strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C"):
			// status + old + new; we'd need rename-aware handling. Skip for MVP.
			if i+2 >= len(tokens) {
				return nil, fmt.Errorf("malformed rename/copy entry near %q", code)
			}
			i += 3
		case code == "A":
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("malformed A entry")
			}
			changes = append(changes, trackedChange{tokens[i+1], StatusAdded})
			i += 2
		case code == "M":
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("malformed M entry")
			}
			changes = append(changes, trackedChange{tokens[i+1], StatusModified})
			i += 2
		case code == "D":
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("malformed D entry")
			}
			changes = append(changes, trackedChange{tokens[i+1], StatusDeleted})
			i += 2
		case code == "T":
			i += 2
		default:
			return nil, fmt.Errorf("unhandled git diff status code: %q", code)
		}
	}
	return changes, nil
}

// untrackedFiles runs `git ls-files --others --exclude-standard -z`.
func (s *Scanner) untrackedFiles(ctx context.Context) ([]string, error) {
	out, err := s.runGit(ctx, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimRight(string(out), "\x00")
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\x00"), nil
}

// show runs `git show <base_ref>:<path>` and returns the bytes.
func (s *Scanner) show(ctx context.Context, path string) ([]byte, error) {
	return s.runGit(ctx, "show", s.BaseRef+":"+path)
}

func (s *Scanner) runGit(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.RepoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}
	return out, nil
}
