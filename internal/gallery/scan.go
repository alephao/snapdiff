// Package gallery enumerates all snapshot PNGs in a repository's working
// tree (subject to the configured globs). Unlike internal/gitscan, which is
// diff-driven, this walks the filesystem and returns every match — including
// unchanged files — so the gallery UI can browse the full baseline library.
package gallery

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/alephao/snapdiff/internal/gitscan"
)

// Item is one PNG screenshot in the gallery.
type Item struct {
	ID   string            // short, URL-safe; stable across restarts
	Path string            // relative to repo root, slash-separated
	Axes map[string]string // from AxisRegex; empty if no match; nil if no regex
}

// Scanner enumerates Items.
type Scanner struct {
	RepoDir   string
	Globs     []string
	AxisRegex *regexp.Regexp
}

var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

// Scan walks RepoDir and returns one Item per PNG matching one of Globs.
// Files matching a glob whose bytes are not a PNG are skipped with a warning.
func (s *Scanner) Scan(ctx context.Context) ([]Item, []string, error) {
	if s.RepoDir == "" {
		return nil, nil, errors.New("gallery: RepoDir is required")
	}
	var items []Item
	var warnings []string

	err := filepath.WalkDir(s.RepoDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			// Skip .git and node_modules — they aren't snapshots, and walking
			// them on large repos wastes time.
			name := d.Name()
			if p != s.RepoDir && (name == ".git" || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(s.RepoDir, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !s.matchesAnyGlob(rel) {
			return nil
		}
		if ok, err := hasPNGSignature(p); err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		} else if !ok {
			warnings = append(warnings, fmt.Sprintf("ignoring non-PNG file in glob: %s", rel))
			return nil
		}
		items = append(items, Item{
			ID:   IDFor(rel),
			Path: rel,
			Axes: gitscan.ExtractAxes(s.AxisRegex, rel),
		})
		return nil
	})
	if err != nil {
		return nil, warnings, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items, warnings, nil
}

func (s *Scanner) matchesAnyGlob(path string) bool {
	for _, g := range s.Globs {
		if ok, _ := doublestar.PathMatch(g, path); ok {
			return true
		}
	}
	return false
}

// hasPNGSignature reads only the first 8 bytes of the file to confirm the
// PNG magic header — cheap even for large screenshots.
func hasPNGSignature(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var head [8]byte
	n, err := io.ReadFull(f, head[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return false, err
	}
	if n < len(pngSignature) {
		return false, nil
	}
	for i, b := range pngSignature {
		if head[i] != b {
			return false, nil
		}
	}
	return true, nil
}

// IDFor returns a stable, short, URL-safe id for a relative path. The same
// path always yields the same id, so bookmarks survive process restarts.
func IDFor(path string) string {
	sum := sha1.Sum([]byte(path))
	return hex.EncodeToString(sum[:8])
}
