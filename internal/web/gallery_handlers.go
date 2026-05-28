package web

import (
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/alephao/snapdiff/internal/gallery"
	"github.com/alephao/snapdiff/internal/web/views"
)

// GalleryServer exposes a read-only baseline-browsing UI. It is independent
// of the diff-review Server: no session, no verdicts, no finalize.
type GalleryServer struct {
	items     []gallery.Item
	byID      map[string]*gallery.Item
	byAxes    map[string]*gallery.Item // canonical axesKey -> item
	axisNames []string                 // sorted union
	axisVals  map[string][]string      // axis -> sorted unique values
	repoDir   string
	publicURL string
	repoLabel string
	handler   http.Handler
}

// NewGalleryServer wires the gallery router. The caller owns the lifecycle.
func NewGalleryServer(items []gallery.Item, repoDir, publicURL string) *GalleryServer {
	g := &GalleryServer{
		items:     items,
		repoDir:   repoDir,
		publicURL: publicURL,
		byID:      make(map[string]*gallery.Item, len(items)),
		byAxes:    make(map[string]*gallery.Item, len(items)),
	}
	for i := range items {
		g.byID[items[i].ID] = &items[i]
		g.byAxes[axesKey(items[i].Axes)] = &items[i]
	}
	g.axisNames, g.axisVals = computeGalleryAxes(items)
	g.handler = g.routes()
	return g
}

// SetRepoLabel customizes the namespace shown in the brand area.
func (g *GalleryServer) SetRepoLabel(label string) { g.repoLabel = label }

// Handler returns the http.Handler suitable to mount on an http.Server.
func (g *GalleryServer) Handler() http.Handler { return g.handler }

// PublicURL is the URL that the cmd prints to stdout.
func (g *GalleryServer) PublicURL() string { return g.publicURL }

func (g *GalleryServer) routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	r.Get("/", g.handleIndex)
	r.Get("/screenshot/{id}", g.handleFocused)
	r.Get("/screenshot/{id}/matrix", g.handleMatrix)
	r.Get("/screenshot/{id}/image.png", g.handleImage)
	r.Get("/missing", g.handleMissing)

	return r
}

func (g *GalleryServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := map[string]string{}
	for _, n := range g.axisNames {
		if v := strings.TrimSpace(q.Get("filter_" + n)); v != "" {
			filter[n] = v
		}
	}
	search := strings.TrimSpace(q.Get("q"))
	lcSearch := strings.ToLower(search)

	var filtered []views.GalleryItemView
	for i := range g.items {
		it := &g.items[i]
		if !axisFilterMatches(it.Axes, filter) {
			continue
		}
		if lcSearch != "" && !strings.Contains(strings.ToLower(it.Path), lcSearch) {
			continue
		}
		filtered = append(filtered, views.GalleryItemView{
			ID:   it.ID,
			Path: it.Path,
			Name: baseNameOf(it.Path),
			Axes: it.Axes,
		})
	}
	axisOpts := g.buildFilterAxisOptions(filter, lcSearch)
	data := views.GalleryIndexData{
		RepoLabel: g.repoLabel,
		Items:     filtered,
		AxisNames: g.axisNames,
		AxisOpts:  axisOpts,
		Filter:    filter,
		Search:    search,
		Matched:   len(filtered),
		Total:     len(g.items),
		NoAnim:    noAnim(r),
	}
	render(w, r, views.GalleryIndex(data))
}

func (g *GalleryServer) handleFocused(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	it, ok := g.byID[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	g.renderFocused(w, r, it.Path, it.ID, it.Axes, true)
}

func (g *GalleryServer) handleMissing(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := map[string]string{}
	for _, n := range g.axisNames {
		v := q.Get("ax_" + n)
		if v == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		target[n] = v
	}
	// If the target combination actually exists, redirect to its clean URL.
	if it, ok := g.byAxes[axesKey(target)]; ok {
		http.Redirect(w, r, "/screenshot/"+it.ID, http.StatusSeeOther)
		return
	}
	g.renderFocused(w, r, "", "", target, false)
}

func (g *GalleryServer) renderFocused(w http.ResponseWriter, r *http.Request, path, id string, axes map[string]string, found bool) {
	options := map[string][]views.AxisOption{}
	for _, name := range g.axisNames {
		var opts []views.AxisOption
		for _, val := range g.axisVals[name] {
			target := cloneAxesWith(axes, name, val)
			sibling, exists := g.byAxes[axesKey(target)]
			var u string
			if exists {
				u = "/screenshot/" + sibling.ID
			} else {
				u = missingURL(target)
			}
			opts = append(opts, views.AxisOption{
				Value:   val,
				Active:  val == axes[name],
				Missing: !exists,
				URL:     u,
			})
		}
		// Present available options first; ✗ ones still selectable (show
		// placeholder) but visually deprioritized.
		sort.SliceStable(opts, func(i, j int) bool {
			if opts[i].Missing != opts[j].Missing {
				return !opts[i].Missing
			}
			return opts[i].Value < opts[j].Value
		})
		options[name] = opts
	}
	focusedURL := "/"
	matrixURL := ""
	if found {
		focusedURL = "/screenshot/" + id
		matrixURL = "/screenshot/" + id + "/matrix"
	} else {
		focusedURL = missingURL(axes)
	}
	data := views.GalleryDetailData{
		RepoLabel:   g.repoLabel,
		Path:        path,
		FoundID:     id,
		Found:       found,
		AxesCurrent: axes,
		AxisNames:   g.axisNames,
		Mode:        "focused",
		FocusedURL:  focusedURL,
		MatrixURL:   matrixURL,
		Options:     options,
		NoAnim:      noAnim(r),
	}
	render(w, r, views.GalleryDetail(data))
}

func (g *GalleryServer) handleMatrix(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	it, ok := g.byID[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if len(g.axisNames) < 1 {
		http.Redirect(w, r, "/screenshot/"+id, http.StatusSeeOther)
		return
	}

	q := r.URL.Query()
	rows := q.Get("rows")
	cols := q.Get("cols")
	if rows == "" {
		rows = g.axisNames[0]
	}
	if cols == "" {
		// Pick the next distinct axis if there's one available.
		for _, n := range g.axisNames {
			if n != rows {
				cols = n
				break
			}
		}
	}
	if rows == cols {
		for _, n := range g.axisNames {
			if n != rows {
				cols = n
				break
			}
		}
	}

	locked := map[string]string{}
	for _, n := range g.axisNames {
		if n == rows || n == cols {
			continue
		}
		if v := q.Get("lock_" + n); v != "" {
			locked[n] = v
		} else {
			locked[n] = it.Axes[n]
		}
	}

	rowVals := g.axisVals[rows]
	var colVals []string
	if cols != "" && cols != rows {
		colVals = g.axisVals[cols]
	} else {
		colVals = []string{""}
	}

	cells := make([][]views.MatrixCell, len(rowVals))
	for i, rv := range rowVals {
		row := make([]views.MatrixCell, len(colVals))
		for j, cv := range colVals {
			target := make(map[string]string, len(g.axisNames))
			for k, v := range locked {
				target[k] = v
			}
			target[rows] = rv
			if cols != "" && cols != rows {
				target[cols] = cv
			}
			if sib, exists := g.byAxes[axesKey(target)]; exists {
				row[j] = views.MatrixCell{
					RowVal:   rv,
					ColVal:   cv,
					ImageURL: "/screenshot/" + sib.ID + "/image.png",
					LinkURL:  "/screenshot/" + sib.ID,
					Path:     sib.Path,
				}
			} else {
				row[j] = views.MatrixCell{RowVal: rv, ColVal: cv, Missing: true}
			}
		}
		cells[i] = row
	}

	var lockedDisplay []views.LockedAxisDisplay
	for _, n := range g.axisNames {
		if n == rows || n == cols {
			continue
		}
		current := locked[n]
		var vals []views.LockedAxisValue
		for _, v := range g.axisVals[n] {
			u := buildMatrixURL(id, rows, cols, locked, n, v)
			vals = append(vals, views.LockedAxisValue{
				Value:  v,
				Active: v == current,
				URL:    u,
			})
		}
		lockedDisplay = append(lockedDisplay, views.LockedAxisDisplay{Name: n, Values: vals})
	}

	data := views.GalleryDetailData{
		RepoLabel:   g.repoLabel,
		Path:        it.Path,
		FoundID:     it.ID,
		Found:       true,
		AxesCurrent: it.Axes,
		AxisNames:   g.axisNames,
		Mode:        "matrix",
		FocusedURL:  "/screenshot/" + id,
		MatrixURL:   "/screenshot/" + id + "/matrix",
		Rows:        rows,
		Cols:        cols,
		RowVals:     rowVals,
		ColVals:     colVals,
		Locked:      lockedDisplay,
		Cells:       cells,
		AxisChoices: g.axisNames,
		NoAnim:      noAnim(r),
	}
	render(w, r, views.GalleryDetail(data))
}

func (g *GalleryServer) handleImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	it, ok := g.byID[id]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(g.repoDir, filepath.FromSlash(it.Path)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "image/png")
	_, _ = io.Copy(w, f)
}

// buildFilterAxisOptions computes per-axis dropdown options reflecting how
// many items would match each value of the axis *given the other filters
// and search*. Values with non-zero counts come first (alphabetical),
// followed by values that would yield zero matches (also alphabetical),
// which the template marks as disabled.
func (g *GalleryServer) buildFilterAxisOptions(filter map[string]string, lcSearch string) map[string][]views.FilterAxisOption {
	out := make(map[string][]views.FilterAxisOption, len(g.axisNames))
	for _, name := range g.axisNames {
		// Build "other filters" — the active filter map sans this axis,
		// so e.g. counting scenarios under page=index ignores any prior
		// scenario filter.
		other := make(map[string]string, len(filter))
		for k, v := range filter {
			if k == name {
				continue
			}
			other[k] = v
		}
		counts := make(map[string]int, len(g.axisVals[name]))
		for i := range g.items {
			it := &g.items[i]
			if !axisFilterMatches(it.Axes, other) {
				continue
			}
			if lcSearch != "" && !strings.Contains(strings.ToLower(it.Path), lcSearch) {
				continue
			}
			counts[it.Axes[name]]++
		}
		var available, unavailable []views.FilterAxisOption
		for _, v := range g.axisVals[name] {
			n := counts[v]
			opt := views.FilterAxisOption{Value: v, Matches: n, Disabled: n == 0}
			if n > 0 {
				available = append(available, opt)
			} else {
				unavailable = append(unavailable, opt)
			}
		}
		out[name] = append(available, unavailable...)
	}
	return out
}

// ---- helpers ----

// axesKey serializes an axes map as a canonical "k=v;k=v;..." string for
// O(1) sibling lookup. Sorted keys make the result independent of map
// iteration order.
func axesKey(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m[k])
	}
	return b.String()
}

func axisFilterMatches(axes, filter map[string]string) bool {
	for k, want := range filter {
		if axes[k] != want {
			return false
		}
	}
	return true
}

func cloneAxesWith(src map[string]string, k, v string) map[string]string {
	out := make(map[string]string, len(src)+1)
	for kk, vv := range src {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func missingURL(axes map[string]string) string {
	q := url.Values{}
	for k, v := range axes {
		q.Set("ax_"+k, v)
	}
	return "/missing?" + q.Encode()
}

func buildMatrixURL(id, rows, cols string, locked map[string]string, swapKey, swapVal string) string {
	q := url.Values{}
	q.Set("rows", rows)
	q.Set("cols", cols)
	for k, v := range locked {
		val := v
		if k == swapKey {
			val = swapVal
		}
		q.Set("lock_"+k, val)
	}
	return "/screenshot/" + id + "/matrix?" + q.Encode()
}

func computeGalleryAxes(items []gallery.Item) (names []string, vals map[string][]string) {
	setNames := map[string]struct{}{}
	tmp := map[string]map[string]struct{}{}
	for _, it := range items {
		for k, v := range it.Axes {
			setNames[k] = struct{}{}
			if tmp[k] == nil {
				tmp[k] = map[string]struct{}{}
			}
			tmp[k][v] = struct{}{}
		}
	}
	for k := range setNames {
		names = append(names, k)
	}
	sort.Strings(names)
	vals = map[string][]string{}
	for k, set := range tmp {
		s := make([]string, 0, len(set))
		for v := range set {
			s = append(s, v)
		}
		sort.Strings(s)
		vals[k] = s
	}
	return
}

func baseNameOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
