package web

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alephao/snapdiff/internal/gitscan"
	"github.com/alephao/snapdiff/internal/imdiff"
	"github.com/alephao/snapdiff/internal/review"
)

func tinyPNG(seed byte) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	c := color.RGBA{R: seed, G: seed, B: seed, A: 255}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

type fakeFinalize struct {
	calls int32
	err   error
	last  []*review.Item
}

func (f *fakeFinalize) fn(items []*review.Item) error {
	atomic.AddInt32(&f.calls, 1)
	f.last = items
	return f.err
}

func newTestServer(t *testing.T, diffs []gitscan.Diff) (*Server, *review.Session, *fakeFinalize) {
	t.Helper()
	sess := review.NewSession(diffs)
	differ := imdiff.New()
	ff := &fakeFinalize{}
	srv := NewServer(sess, differ, "http://127.0.0.1:0", ff.fn)
	return srv, sess, ff
}

func do(t *testing.T, h http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, body)
	if body != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func formBody(v url.Values) io.Reader { return strings.NewReader(v.Encode()) }

func TestHealthz(t *testing.T) {
	srv, _, _ := newTestServer(t, nil)
	w := do(t, srv.Handler(), "GET", "/healthz", nil)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestIndex_emptyShowsNoDiffsMessage(t *testing.T) {
	srv, _, _ := newTestServer(t, nil)
	w := do(t, srv.Handler(), "GET", "/", nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No diffs") {
		t.Errorf("expected 'No diffs' in body, got: %s", body)
	}
}

func TestIndex_listsDiffs(t *testing.T) {
	srv, _, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1), Axes: map[string]string{"theme": "dark"}},
		{Path: "b.png", Status: gitscan.StatusAdded, Current: tinyPNG(1), Axes: map[string]string{"theme": "light"}},
	})
	w := do(t, srv.Handler(), "GET", "/", nil)
	body := w.Body.String()
	if !strings.Contains(body, "a.png") || !strings.Contains(body, "b.png") {
		t.Errorf("expected both paths in body")
	}
	if !strings.Contains(body, "theme") {
		t.Errorf("expected axis header 'theme' in body")
	}
	if !strings.Contains(body, "Finalize") {
		t.Errorf("expected finalize button")
	}
}

func TestDiffPage_unknownID404(t *testing.T) {
	srv, _, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	w := do(t, srv.Handler(), "GET", "/diff/nope", nil)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDiffPage_rendersWithModes(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1), Axes: map[string]string{"theme": "dark"}},
	})
	id := sess.Items()[0].ID
	w := do(t, srv.Handler(), "GET", "/diff/"+id, nil)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	for _, mode := range []string{"side", "swipe", "toggle", "pixel", "onion"} {
		if !strings.Contains(body, `data-mode="`+mode+`"`) {
			t.Errorf("expected pane data-mode=%q in body", mode)
		}
	}
}

func TestImageEndpoints(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID

	for _, kind := range []string{"baseline.png", "current.png", "pixeldiff.png"} {
		w := do(t, srv.Handler(), "GET", "/diff/"+id+"/"+kind, nil)
		if w.Code != 200 {
			t.Errorf("%s: status = %d", kind, w.Code)
			continue
		}
		if got := w.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("%s: content-type = %q", kind, got)
		}
		if _, err := png.Decode(bytes.NewReader(w.Body.Bytes())); err != nil {
			t.Errorf("%s: not a valid PNG: %v", kind, err)
		}
	}
}

func TestImageEndpoints_addedHasNoBaseline(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusAdded, Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID
	w := do(t, srv.Handler(), "GET", "/diff/"+id+"/baseline.png", nil)
	if w.Code != 404 && w.Code != 204 {
		t.Errorf("baseline of added file: expected 404 or 204, got %d", w.Code)
	}
}

func TestVerdictApprove(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID
	w := do(t, srv.Handler(), "POST", "/diff/"+id+"/verdict",
		formBody(url.Values{"status": {"approved"}}))
	if w.Code != http.StatusSeeOther && w.Code != http.StatusFound && w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	it, _ := sess.Get(id)
	if it.Verdict.Status != review.StatusApproved {
		t.Errorf("verdict = %v, want approved", it.Verdict.Status)
	}
}

func TestVerdictApprove_redirectsToNextWhenProvided(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
		{Path: "b.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID
	nextID := sess.Items()[1].ID
	w := do(t, srv.Handler(), "POST", "/diff/"+id+"/verdict",
		formBody(url.Values{"status": {"approved"}, "next": {"/diff/" + nextID}}))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/diff/"+nextID {
		t.Errorf("Location = %q, want /diff/%s", got, nextID)
	}
}

func TestVerdictApprove_rejectsOffsiteNext(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID
	for _, bad := range []string{"https://evil.example/", "//evil.example/x", "evil"} {
		w := do(t, srv.Handler(), "POST", "/diff/"+id+"/verdict",
			formBody(url.Values{"status": {"approved"}, "next": {bad}}))
		if got := w.Header().Get("Location"); got != "/" {
			t.Errorf("next=%q: Location = %q, want /", bad, got)
		}
	}
}

func TestVerdictRejectWithComment(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID
	do(t, srv.Handler(), "POST", "/diff/"+id+"/verdict",
		formBody(url.Values{"status": {"rejected"}, "comment": {"logo cropped"}}))
	it, _ := sess.Get(id)
	if it.Verdict.Status != review.StatusRejected || it.Verdict.Comment != "logo cropped" {
		t.Errorf("verdict = %+v", it.Verdict)
	}
}

func TestVerdict_unknownID404(t *testing.T) {
	srv, _, _ := newTestServer(t, nil)
	w := do(t, srv.Handler(), "POST", "/diff/nope/verdict",
		formBody(url.Values{"status": {"approved"}}))
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestVerdict_invalidStatus400(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID
	w := do(t, srv.Handler(), "POST", "/diff/"+id+"/verdict",
		formBody(url.Values{"status": {"bogus"}}))
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkVerdict_filterByAxis(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1), Axes: map[string]string{"theme": "dark"}},
		{Path: "b.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1), Axes: map[string]string{"theme": "light"}},
		{Path: "c.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1), Axes: map[string]string{"theme": "dark"}},
	})
	do(t, srv.Handler(), "POST", "/diff/bulk-verdict",
		formBody(url.Values{"filter_theme": {"dark"}, "status": {"approved"}}))
	statuses := []review.VerdictStatus{
		sess.Items()[0].Verdict.Status,
		sess.Items()[1].Verdict.Status,
		sess.Items()[2].Verdict.Status,
	}
	if statuses[0] != review.StatusApproved || statuses[1] != review.StatusPending || statuses[2] != review.StatusApproved {
		t.Errorf("statuses = %v", statuses)
	}
}

func TestBulkVerdict_blankFilterMatchesAll(t *testing.T) {
	srv, sess, _ := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
		{Path: "b.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	do(t, srv.Handler(), "POST", "/diff/bulk-verdict",
		formBody(url.Values{"status": {"approved"}}))
	for _, it := range sess.Items() {
		if it.Verdict.Status != review.StatusApproved {
			t.Errorf("item %s: status = %v", it.ID, it.Verdict.Status)
		}
	}
}

func TestFinalize_blockedWhenPending(t *testing.T) {
	srv, _, ff := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	w := do(t, srv.Handler(), "POST", "/session/finalize", nil)
	if w.Code != 409 && w.Code != 400 {
		t.Errorf("status = %d, want 409 or 400", w.Code)
	}
	if atomic.LoadInt32(&ff.calls) != 0 {
		t.Errorf("OnFinalize called %d times, want 0", ff.calls)
	}
}

func TestFinalize_callsHookAndClosesDone(t *testing.T) {
	srv, sess, ff := newTestServer(t, []gitscan.Diff{
		{Path: "a.png", Status: gitscan.StatusModified, Baseline: tinyPNG(0), Current: tinyPNG(1)},
	})
	id := sess.Items()[0].ID
	_ = sess.SetVerdict(id, review.Verdict{Status: review.StatusApproved})

	w := do(t, srv.Handler(), "POST", "/session/finalize", nil)
	if w.Code < 200 || w.Code >= 400 {
		t.Errorf("status = %d", w.Code)
	}
	if got := atomic.LoadInt32(&ff.calls); got != 1 {
		t.Errorf("OnFinalize calls = %d, want 1", got)
	}
	select {
	case <-sess.Done():
	default:
		t.Error("Done channel should be closed after finalize")
	}
}

func TestStatic_servedFromHandler(t *testing.T) {
	srv, _, _ := newTestServer(t, nil)
	w := do(t, srv.Handler(), "GET", "/static/snapdiff.css", nil)
	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
}
