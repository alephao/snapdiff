package apply

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alephao/snapdiff/internal/gitscan"
	"github.com/alephao/snapdiff/internal/review"
)

// initRepo creates a temp git repo with a baseline snapshot committed.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func writeFile(t *testing.T, dir, rel string, body []byte) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func itemFor(diff gitscan.Diff, v review.Verdict) *review.Item {
	return &review.Item{ID: "x", Diff: diff, Verdict: v}
}

func TestApply_approvedModified_leavesWorkingTreeAlone(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.png", []byte("baseline"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	writeFile(t, repo, "a.png", []byte("current"))

	res, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "a.png", Status: gitscan.StatusModified}, review.Verdict{Status: review.StatusApproved}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if string(readFile(t, repo, "a.png")) != "current" {
		t.Error("approved-modified should leave working tree at current")
	}
	if len(res.Verdicts) != 1 || res.Verdicts[0].Status != "approved" {
		t.Errorf("verdicts = %+v", res.Verdicts)
	}
}

func TestApply_rejectedModified_revertsToBaseline(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.png", []byte("baseline"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	writeFile(t, repo, "a.png", []byte("current"))

	_, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "a.png", Status: gitscan.StatusModified}, review.Verdict{Status: review.StatusRejected, Comment: "no"}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := string(readFile(t, repo, "a.png")); got != "baseline" {
		t.Errorf("rejected-modified should restore baseline, got %q", got)
	}
}

func TestApply_rejectedAdded_removesFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "README.md", []byte("hi"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	writeFile(t, repo, "new.png", []byte("untracked"))

	_, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "new.png", Status: gitscan.StatusAdded}, review.Verdict{Status: review.StatusRejected, Comment: "no"}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "new.png")); !os.IsNotExist(err) {
		t.Error("rejected-added should remove the file")
	}
}

func TestApply_approvedAdded_leavesFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "README.md", []byte("hi"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	writeFile(t, repo, "new.png", []byte("untracked"))

	_, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "new.png", Status: gitscan.StatusAdded}, review.Verdict{Status: review.StatusApproved}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "new.png")); err != nil {
		t.Errorf("approved-added should leave file in place, got err %v", err)
	}
}

func TestApply_rejectedDeleted_restoresFromBaseline(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "gone.png", []byte("baseline"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	if err := os.Remove(filepath.Join(repo, "gone.png")); err != nil {
		t.Fatal(err)
	}

	_, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "gone.png", Status: gitscan.StatusDeleted}, review.Verdict{Status: review.StatusRejected, Comment: "no"}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := string(readFile(t, repo, "gone.png")); got != "baseline" {
		t.Errorf("rejected-deleted should restore baseline, got %q", got)
	}
}

func TestApply_approvedDeleted_leavesDeletion(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "gone.png", []byte("baseline"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	if err := os.Remove(filepath.Join(repo, "gone.png")); err != nil {
		t.Fatal(err)
	}

	_, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "gone.png", Status: gitscan.StatusDeleted}, review.Verdict{Status: review.StatusApproved}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "gone.png")); !os.IsNotExist(err) {
		t.Error("approved-deleted should leave the file removed")
	}
}

func TestApply_verdictJSONStructure(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.png", []byte("baseline"))
	writeFile(t, repo, "b.png", []byte("baseline"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	writeFile(t, repo, "a.png", []byte("current"))
	writeFile(t, repo, "b.png", []byte("current"))

	res, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "a.png", Status: gitscan.StatusModified}, review.Verdict{Status: review.StatusApproved}),
		itemFor(gitscan.Diff{Path: "b.png", Status: gitscan.StatusModified}, review.Verdict{Status: review.StatusRejected, Comment: "logo cropped"}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Verdicts) != 2 {
		t.Fatalf("expected 2 verdicts, got %d", len(res.Verdicts))
	}
	if res.Verdicts[0].Path != "a.png" || res.Verdicts[0].Status != "approved" || res.Verdicts[0].Comment != "" {
		t.Errorf("verdict[0] = %+v", res.Verdicts[0])
	}
	if res.Verdicts[1].Path != "b.png" || res.Verdicts[1].Status != "rejected" || res.Verdicts[1].Comment != "logo cropped" {
		t.Errorf("verdict[1] = %+v", res.Verdicts[1])
	}
}

func TestApply_pendingItemErrors(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, repo, "a.png", []byte("baseline"))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "init")
	writeFile(t, repo, "a.png", []byte("current"))

	_, err := Apply(context.Background(), repo, "HEAD", []*review.Item{
		itemFor(gitscan.Diff{Path: "a.png", Status: gitscan.StatusModified}, review.Verdict{Status: review.StatusPending}),
	})
	if err == nil {
		t.Fatal("Apply should error on pending item")
	}
}
