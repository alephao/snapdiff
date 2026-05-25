// Package review holds the in-memory state for one review cycle: the list
// of diffed snapshots, per-file verdicts, and a "done" signal channel
// closed when the reviewer hits Finalize.
package review

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/alephao/snapdiff/internal/gitscan"
)

// VerdictStatus is the per-item review outcome.
type VerdictStatus int

const (
	StatusPending VerdictStatus = iota
	StatusApproved
	StatusRejected
)

func (v VerdictStatus) String() string {
	switch v {
	case StatusPending:
		return "pending"
	case StatusApproved:
		return "approved"
	case StatusRejected:
		return "rejected"
	default:
		return fmt.Sprintf("verdict(%d)", int(v))
	}
}

// Verdict is the reviewer's call on a single Item.
type Verdict struct {
	Status  VerdictStatus
	Comment string
}

// Item is one snapshot under review.
type Item struct {
	ID      string
	Diff    gitscan.Diff
	Verdict Verdict
}

// Session is the in-memory state for one review cycle.
type Session struct {
	mu       sync.RWMutex
	items    []*Item
	byID     map[string]*Item
	done     chan struct{}
	doneOnce sync.Once
}

// NewSession builds a session from a list of diffs. IDs are stable per
// session (we use the index as a string).
func NewSession(diffs []gitscan.Diff) *Session {
	items := make([]*Item, len(diffs))
	byID := make(map[string]*Item, len(diffs))
	for i, d := range diffs {
		id := strconv.Itoa(i)
		it := &Item{ID: id, Diff: d, Verdict: Verdict{Status: StatusPending}}
		items[i] = it
		byID[id] = it
	}
	return &Session{
		items: items,
		byID:  byID,
		done:  make(chan struct{}),
	}
}

// Items returns the items in their original (path-sorted) order.
func (s *Session) Items() []*Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Item, len(s.items))
	copy(out, s.items)
	return out
}

// Get returns the item with the given ID.
func (s *Session) Get(id string) (*Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.byID[id]
	return it, ok
}

// SetVerdict updates a single item's verdict.
func (s *Session) SetVerdict(id string, v Verdict) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.byID[id]
	if !ok {
		return fmt.Errorf("review: unknown item id %q", id)
	}
	it.Verdict = v
	return nil
}

// BulkSetVerdict applies the verdict to every item whose Axes match every
// key/value in filter (empty filter matches all). Returns the number of
// items updated.
func (s *Session) BulkSetVerdict(filter map[string]string, v Verdict) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, it := range s.items {
		if !axesMatch(it.Diff.Axes, filter) {
			continue
		}
		it.Verdict = v
		count++
	}
	return count, nil
}

func axesMatch(axes, filter map[string]string) bool {
	for k, want := range filter {
		if axes[k] != want {
			return false
		}
	}
	return true
}

// AllResolved reports whether every item has a non-pending verdict.
func (s *Session) AllResolved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, it := range s.items {
		if it.Verdict.Status == StatusPending {
			return false
		}
	}
	return true
}

// Finalize closes the Done channel after verifying every item is
// resolved. Idempotent: second and later calls are no-ops returning nil.
func (s *Session) Finalize() error {
	s.mu.RLock()
	for _, it := range s.items {
		if it.Verdict.Status == StatusPending {
			s.mu.RUnlock()
			return errors.New("review: cannot finalize; items still pending")
		}
	}
	s.mu.RUnlock()

	s.doneOnce.Do(func() {
		close(s.done)
	})
	return nil
}

// Done returns a channel that is closed when Finalize succeeds.
func (s *Session) Done() <-chan struct{} {
	return s.done
}
