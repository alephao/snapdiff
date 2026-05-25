// Package apply executes the reviewer's verdicts against the git working
// tree: approved items are left alone, rejected items are reverted (via
// `git checkout <base_ref> -- <path>` for tracked files, or `os.Remove`
// for untracked-added files). Returns the verdict JSON the agent
// consumes from stdout.
package apply

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alephao/snapdiff/internal/gitscan"
	"github.com/alephao/snapdiff/internal/review"
)

// FileVerdict is the per-file outcome reported to the agent.
type FileVerdict struct {
	Path    string `json:"path"`
	Status  string `json:"status"` // "approved" | "rejected"
	Comment string `json:"comment,omitempty"`
}

// Result is the top-level JSON shape emitted on stdout by `snapdiff await`.
type Result struct {
	Verdicts []FileVerdict `json:"verdicts"`
}

// Apply runs the verdict actions against repoDir's working tree and returns
// the verdict list. baseRef is the git ref used to restore rejected
// modifications and deletions (typically "HEAD").
func Apply(ctx context.Context, repoDir, baseRef string, items []*review.Item) (Result, error) {
	for _, it := range items {
		if it.Verdict.Status == review.StatusPending {
			return Result{}, fmt.Errorf("apply: item %s is still pending", it.Diff.Path)
		}
	}

	verdicts := make([]FileVerdict, 0, len(items))
	for _, it := range items {
		if it.Verdict.Status == review.StatusRejected {
			if err := revert(ctx, repoDir, baseRef, it.Diff); err != nil {
				return Result{}, fmt.Errorf("revert %s: %w", it.Diff.Path, err)
			}
		}
		verdicts = append(verdicts, FileVerdict{
			Path:    it.Diff.Path,
			Status:  verdictWord(it.Verdict.Status),
			Comment: it.Verdict.Comment,
		})
	}
	return Result{Verdicts: verdicts}, nil
}

func verdictWord(s review.VerdictStatus) string {
	switch s {
	case review.StatusApproved:
		return "approved"
	case review.StatusRejected:
		return "rejected"
	default:
		return "pending"
	}
}

func revert(ctx context.Context, repoDir, baseRef string, d gitscan.Diff) error {
	switch d.Status {
	case gitscan.StatusAdded:
		// Untracked file the agent created. Remove it.
		full := filepath.Join(repoDir, d.Path)
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	case gitscan.StatusModified, gitscan.StatusDeleted:
		return runGit(ctx, repoDir, "checkout", baseRef, "--", d.Path)
	default:
		return fmt.Errorf("apply: unknown gitscan status %v", d.Status)
	}
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}
