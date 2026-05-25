package review

import (
	"sync"
	"testing"

	"github.com/alephao/snapdiff/internal/gitscan"
)

func diff(path string, axes map[string]string) gitscan.Diff {
	return gitscan.Diff{
		Path:     path,
		Status:   gitscan.StatusModified,
		Baseline: []byte("baseline"),
		Current:  []byte("current"),
		Axes:     axes,
	}
}

func TestNewSession_emptyItemsIsAllResolved(t *testing.T) {
	s := NewSession(nil)
	if !s.AllResolved() {
		t.Error("empty session should be AllResolved")
	}
}

func TestNewSession_assignsStableIDsInOrder(t *testing.T) {
	in := []gitscan.Diff{
		diff("a.png", nil),
		diff("b.png", nil),
		diff("c.png", nil),
	}
	s := NewSession(in)
	items := s.Items()
	if len(items) != 3 {
		t.Fatalf("len items = %d", len(items))
	}
	for i, item := range items {
		if item.ID == "" {
			t.Errorf("item %d has empty ID", i)
		}
		if item.Diff.Path != in[i].Path {
			t.Errorf("order broken at %d: %q vs %q", i, item.Diff.Path, in[i].Path)
		}
		if item.Verdict.Status != StatusPending {
			t.Errorf("item %d should start Pending, got %v", i, item.Verdict.Status)
		}
	}
}

func TestSession_GetByID(t *testing.T) {
	s := NewSession([]gitscan.Diff{diff("a.png", nil)})
	id := s.Items()[0].ID
	got, ok := s.Get(id)
	if !ok {
		t.Fatal("Get returned !ok")
	}
	if got.Diff.Path != "a.png" {
		t.Errorf("Path = %q", got.Diff.Path)
	}

	if _, ok := s.Get("does-not-exist"); ok {
		t.Error("Get for unknown ID should return !ok")
	}
}

func TestSession_SetVerdictApprove(t *testing.T) {
	s := NewSession([]gitscan.Diff{diff("a.png", nil)})
	id := s.Items()[0].ID
	if err := s.SetVerdict(id, Verdict{Status: StatusApproved}); err != nil {
		t.Fatalf("SetVerdict: %v", err)
	}
	got, _ := s.Get(id)
	if got.Verdict.Status != StatusApproved {
		t.Errorf("Status = %v, want Approved", got.Verdict.Status)
	}
}

func TestSession_SetVerdictRejectWithComment(t *testing.T) {
	s := NewSession([]gitscan.Diff{diff("a.png", nil)})
	id := s.Items()[0].ID
	if err := s.SetVerdict(id, Verdict{Status: StatusRejected, Comment: "logo cropped"}); err != nil {
		t.Fatalf("SetVerdict: %v", err)
	}
	got, _ := s.Get(id)
	if got.Verdict.Status != StatusRejected || got.Verdict.Comment != "logo cropped" {
		t.Errorf("Verdict = %+v", got.Verdict)
	}
}

func TestSession_SetVerdictUnknownIDErrors(t *testing.T) {
	s := NewSession([]gitscan.Diff{diff("a.png", nil)})
	err := s.SetVerdict("nope", Verdict{Status: StatusApproved})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSession_AllResolved(t *testing.T) {
	s := NewSession([]gitscan.Diff{diff("a.png", nil), diff("b.png", nil)})
	if s.AllResolved() {
		t.Error("should not be resolved with 0 verdicts")
	}
	_ = s.SetVerdict(s.Items()[0].ID, Verdict{Status: StatusApproved})
	if s.AllResolved() {
		t.Error("should not be resolved with 1/2 verdicts")
	}
	_ = s.SetVerdict(s.Items()[1].ID, Verdict{Status: StatusRejected, Comment: "x"})
	if !s.AllResolved() {
		t.Error("should be resolved with 2/2 verdicts")
	}
}

func TestSession_BulkSetVerdict_FilterByAxis(t *testing.T) {
	s := NewSession([]gitscan.Diff{
		diff("a.png", map[string]string{"theme": "dark", "device": "iphone17"}),
		diff("b.png", map[string]string{"theme": "light", "device": "iphone17"}),
		diff("c.png", map[string]string{"theme": "dark", "device": "ipad"}),
	})
	count, err := s.BulkSetVerdict(map[string]string{"theme": "dark"}, Verdict{Status: StatusApproved})
	if err != nil {
		t.Fatalf("BulkSetVerdict: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	statuses := []VerdictStatus{
		s.Items()[0].Verdict.Status,
		s.Items()[1].Verdict.Status,
		s.Items()[2].Verdict.Status,
	}
	want := []VerdictStatus{StatusApproved, StatusPending, StatusApproved}
	for i := range statuses {
		if statuses[i] != want[i] {
			t.Errorf("item %d: got %v, want %v", i, statuses[i], want[i])
		}
	}
}

func TestSession_BulkSetVerdict_MultipleAxes(t *testing.T) {
	s := NewSession([]gitscan.Diff{
		diff("a.png", map[string]string{"theme": "dark", "device": "iphone17"}),
		diff("b.png", map[string]string{"theme": "dark", "device": "ipad"}),
	})
	count, _ := s.BulkSetVerdict(map[string]string{"theme": "dark", "device": "iphone17"}, Verdict{Status: StatusApproved})
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestSession_BulkSetVerdict_EmptyFilterMatchesAll(t *testing.T) {
	s := NewSession([]gitscan.Diff{
		diff("a.png", nil),
		diff("b.png", nil),
	})
	count, _ := s.BulkSetVerdict(map[string]string{}, Verdict{Status: StatusApproved})
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestSession_FinalizeClosesDone(t *testing.T) {
	s := NewSession(nil)
	if err := s.Finalize(); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	select {
	case <-s.Done():
	default:
		t.Error("Done channel should be closed after Finalize")
	}
}

func TestSession_FinalizeErrorsIfPending(t *testing.T) {
	s := NewSession([]gitscan.Diff{diff("a.png", nil)})
	if err := s.Finalize(); err == nil {
		t.Fatal("Finalize should error when items are pending")
	}
	select {
	case <-s.Done():
		t.Error("Done should NOT be closed when Finalize errored")
	default:
	}
}

func TestSession_FinalizeIdempotent(t *testing.T) {
	s := NewSession(nil)
	_ = s.Finalize()
	if err := s.Finalize(); err != nil {
		t.Errorf("second Finalize errored: %v", err)
	}
}

func TestSession_ConcurrentSetVerdict(t *testing.T) {
	in := make([]gitscan.Diff, 50)
	for i := range in {
		in[i] = diff(string(rune('a'+i%26))+".png", nil)
	}
	// Make all paths unique by appending index.
	for i := range in {
		in[i].Path = in[i].Path[:1] + "_" + intToStr(i) + ".png"
	}
	s := NewSession(in)

	var wg sync.WaitGroup
	for _, it := range s.Items() {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_ = s.SetVerdict(id, Verdict{Status: StatusApproved})
		}(it.ID)
	}
	wg.Wait()

	if !s.AllResolved() {
		t.Error("expected all resolved after concurrent writes")
	}
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	var s string
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
