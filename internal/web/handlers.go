// Package web wires the chi router, templ views, and static assets that
// make up the snapdiff review UI.
package web

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/alephao/snapdiff/internal/imdiff"
	"github.com/alephao/snapdiff/internal/review"
	"github.com/alephao/snapdiff/internal/web/views"
)

//go:embed static/*
var staticFS embed.FS

// FinalizeFunc is invoked when the reviewer hits Finalize. It receives the
// items in their session order; the implementation typically calls
// internal/apply and records the result for the cmd to emit on stdout.
type FinalizeFunc func(items []*review.Item) error

// Server bundles the session, the pixel-diff cache, and a finalize callback
// behind an http.Handler.
type Server struct {
	session    *review.Session
	differ     *imdiff.Differ
	publicURL  string // informational only; printed by cmd
	repoLabel  string // shown in the top bar
	onFinalize FinalizeFunc

	handler http.Handler
}

// NewServer assembles the router.
func NewServer(s *review.Session, d *imdiff.Differ, publicURL string, onFinalize FinalizeFunc) *Server {
	srv := &Server{
		session:    s,
		differ:     d,
		publicURL:  publicURL,
		onFinalize: onFinalize,
	}
	srv.handler = srv.routes()
	return srv
}

// SetRepoLabel customizes the namespace shown in the brand area (e.g. "ios-app/").
func (s *Server) SetRepoLabel(label string) { s.repoLabel = label }

// Handler returns the http.Handler suitable to mount on an http.Server.
func (s *Server) Handler() http.Handler { return s.handler }

// PublicURL is the URL that the cmd prints to stdout for the reviewer.
func (s *Server) PublicURL() string { return s.publicURL }

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	r.Get("/", s.handleIndex)
	r.Get("/diff/{id}", s.handleDiff)
	r.Get("/diff/{id}/baseline.png", s.handleImage("baseline"))
	r.Get("/diff/{id}/current.png", s.handleImage("current"))
	r.Get("/diff/{id}/pixeldiff.png", s.handlePixelDiff)
	r.Post("/diff/{id}/verdict", s.handleVerdict)
	r.Post("/diff/bulk-verdict", s.handleBulkVerdict)
	r.Post("/session/finalize", s.handleFinalize)

	return r
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	items := s.session.Items()
	axisN := axisNames(items)
	d := views.IndexData{
		Items:     items,
		AxisNames: axisN,
		AxisVals:  axisValues(items),
		Groups:    buildGroups(items),
		Counts:    countItems(items),
		RepoLabel: s.repoLabel,
	}
	render(w, r, views.Index(d))
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	items := s.session.Items()
	var idx = -1
	for i, it := range items {
		if it.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		http.NotFound(w, r)
		return
	}
	prev, next := "", ""
	if idx > 0 {
		prev = items[idx-1].ID
	}
	if idx < len(items)-1 {
		next = items[idx+1].ID
	}
	render(w, r, views.Diff(views.DiffData{
		Item:      items[idx],
		AxisNames: axisNames(items),
		Position:  idx + 1,
		Total:     len(items),
		PrevID:    prev,
		NextID:    next,
	}))
}

func (s *Server) handleImage(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		it, ok := s.session.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		var bytes []byte
		switch kind {
		case "baseline":
			bytes = it.Diff.Baseline
		case "current":
			bytes = it.Diff.Current
		}
		if len(bytes) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(bytes)
	}
}

func (s *Server) handlePixelDiff(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	it, ok := s.session.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	out, err := s.differ.Diff(it.Diff.Baseline, it.Diff.Current)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(out)
}

func (s *Server) handleVerdict(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := s.session.Get(id); !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	status, err := parseStatus(r.FormValue("status"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.session.SetVerdict(id, review.Verdict{
		Status:  status,
		Comment: strings.TrimSpace(r.FormValue("comment")),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleBulkVerdict(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	status, err := parseStatus(r.FormValue("status"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	filter := map[string]string{}
	for k, v := range r.Form {
		if !strings.HasPrefix(k, "filter_") {
			continue
		}
		val := strings.TrimSpace(v[0])
		if val == "" {
			continue
		}
		filter[strings.TrimPrefix(k, "filter_")] = val
	}
	if _, err := s.session.BulkSetVerdict(filter, review.Verdict{
		Status:  status,
		Comment: strings.TrimSpace(r.FormValue("comment")),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleFinalize(w http.ResponseWriter, r *http.Request) {
	if !s.session.AllResolved() {
		http.Error(w, "some items are still pending", http.StatusConflict)
		return
	}
	items := s.session.Items()
	if s.onFinalize != nil {
		if err := s.onFinalize(items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := s.session.Finalize(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>done</title>
<link rel="stylesheet" href="/static/snapdiff.css"/></head>
<body><section class="empty"><h1>Done</h1><p>Verdicts sent to agent.</p></section></body></html>`))
}

func parseStatus(s string) (review.VerdictStatus, error) {
	switch s {
	case "approved":
		return review.StatusApproved, nil
	case "rejected":
		return review.StatusRejected, nil
	default:
		return 0, errors.New("status must be 'approved' or 'rejected'")
	}
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
}

// ---- shared computations ----

// axisNames returns the sorted union of axis names across items.
func axisNames(items []*review.Item) []string {
	set := map[string]struct{}{}
	for _, it := range items {
		for k := range it.Diff.Axes {
			set[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// axisValues returns sorted unique values per axis.
func axisValues(items []*review.Item) map[string][]string {
	tmp := map[string]map[string]struct{}{}
	for _, it := range items {
		for k, v := range it.Diff.Axes {
			if tmp[k] == nil {
				tmp[k] = map[string]struct{}{}
			}
			tmp[k][v] = struct{}{}
		}
	}
	out := map[string][]string{}
	for k, vs := range tmp {
		var vals []string
		for v := range vs {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		out[k] = vals
	}
	return out
}

func countItems(items []*review.Item) views.Counts {
	c := views.Counts{Total: len(items)}
	for _, it := range items {
		switch it.Verdict.Status {
		case review.StatusApproved:
			c.Approved++
		case review.StatusRejected:
			c.Rejected++
		default:
			c.Pending++
		}
	}
	return c
}

// buildGroups groups items by the leaf directory of their path (typically
// the test-class directory). Items keep their original order within a group;
// group order follows first-appearance.
func buildGroups(items []*review.Item) []views.ItemGroup {
	byDir := map[string]*views.ItemGroup{}
	var order []string
	for _, it := range items {
		dir := filepath.Dir(it.Diff.Path)
		// Use slash-separated paths (gitscan emits forward-slash paths).
		dir = filepath.ToSlash(dir)
		g, ok := byDir[dir]
		if !ok {
			label, prefix := splitLeaf(dir)
			g = &views.ItemGroup{Label: label, Prefix: prefix}
			byDir[dir] = g
			order = append(order, dir)
		}
		g.Items = append(g.Items, it)
		switch it.Verdict.Status {
		case review.StatusApproved:
			g.Counts.Approved++
		case review.StatusRejected:
			g.Counts.Rejected++
		default:
			g.Counts.Pending++
		}
		g.Counts.Total++
	}
	out := make([]views.ItemGroup, 0, len(order))
	for _, k := range order {
		out = append(out, *byDir[k])
	}
	return out
}

func splitLeaf(dir string) (label, prefix string) {
	if dir == "" || dir == "." {
		return "(root)", ""
	}
	slash := strings.LastIndex(dir, "/")
	if slash < 0 {
		return dir, ""
	}
	return dir[slash+1:], dir[:slash+1]
}
