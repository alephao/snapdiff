package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alephao/snapdiff/internal/gallery"
)

// galleryFixture builds a tmp repo with 4 PNGs across two axes (page × theme)
// where one combination is intentionally missing so we can exercise the
// placeholder paths. Returns the repo dir, items, and a function that
// gives back the gallery server hooked to that dir.
func galleryFixture(t *testing.T) (string, []gallery.Item, *GalleryServer) {
	t.Helper()
	dir := t.TempDir()
	tinyPNG := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	mustWrite := func(rel string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, tinyPNG, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Three of four combos exist: home-dark missing.
	mustWrite("shots/home-light.png")
	mustWrite("shots/about-dark.png")
	mustWrite("shots/about-light.png")
	items := []gallery.Item{
		{ID: gallery.IDFor("shots/about-dark.png"), Path: "shots/about-dark.png", Axes: map[string]string{"page": "about", "theme": "dark"}},
		{ID: gallery.IDFor("shots/about-light.png"), Path: "shots/about-light.png", Axes: map[string]string{"page": "about", "theme": "light"}},
		{ID: gallery.IDFor("shots/home-light.png"), Path: "shots/home-light.png", Axes: map[string]string{"page": "home", "theme": "light"}},
	}
	srv := NewGalleryServer(items, dir, "http://test")
	srv.SetRepoLabel("test/")
	return dir, items, srv
}

func TestGallery_IndexLists200WithItems(t *testing.T) {
	_, items, srv := galleryFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, it := range items {
		name := it.Path[strings.LastIndex(it.Path, "/")+1:]
		if !strings.Contains(body, name) {
			t.Errorf("body missing card for %q", name)
		}
	}
	// Each axis name should appear as a filter dropdown.
	for _, n := range []string{"page", "theme"} {
		if !strings.Contains(body, `name="filter_`+n+`"`) {
			t.Errorf("body missing dropdown for axis %q", n)
		}
	}
}

func TestGallery_IndexDropdownPartitionsByOtherFilters(t *testing.T) {
	// With filter_page=home active, the only available theme is "light" —
	// "dark" would yield 0 (since home-dark doesn't exist). The template
	// must mark dark as disabled and emit the divider option.
	_, _, srv := galleryFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/?filter_page=home", nil)
	srv.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()

	if !strings.Contains(body, "── no matches ──") {
		t.Errorf("expected divider option in dropdown; body did not contain it")
	}
	if !strings.Contains(body, `<option value="dark" disabled>dark (0)</option>`) {
		t.Errorf("expected dark option to be disabled with (0); got body:\n%s", trim(body))
	}
	if !strings.Contains(body, `<option value="light">light (1)</option>`) {
		t.Errorf("expected light option enabled with (1); got body:\n%s", trim(body))
	}
}

func TestGallery_IndexFiltersByAxis(t *testing.T) {
	_, _, srv := galleryFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/?filter_page=about", nil)
	srv.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "about-dark.png") || !strings.Contains(body, "about-light.png") {
		t.Errorf("expected about-* items in filtered body")
	}
	if strings.Contains(body, "home-light.png") {
		t.Errorf("did not expect home-light when filter_page=about")
	}
}

func TestGallery_ImageEndpointServesPNGBytes(t *testing.T) {
	_, items, srv := galleryFixture(t)
	id := items[0].ID
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/screenshot/"+id+"/image.png", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) < 8 || string(body[:4]) != "\x89PNG" {
		t.Errorf("body does not start with PNG signature: %x", body[:8])
	}
}

func TestGallery_FocusedShowsAxisDropdownsAndImage(t *testing.T) {
	_, items, srv := galleryFixture(t)
	id := items[0].ID // about-dark
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/screenshot/"+id, nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "shot-frame") || !strings.Contains(body, "/image.png") {
		t.Errorf("focused body missing shot-frame/image: %s", body[:min(800, len(body))])
	}
	// Page dropdown should include the 'home' option — home-dark doesn't
	// exist so the label carries the missing marker.
	if !strings.Contains(body, "home ✗") {
		t.Errorf("body missing 'home ✗' option for the missing combo")
	}
}

func TestGallery_MissingRouteRedirectsForExistingCombo(t *testing.T) {
	_, items, srv := galleryFixture(t)
	// about + light exists; missing should redirect to that item.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/missing?ax_page=about&ax_theme=light", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	want := "/screenshot/" + items[1].ID
	if loc := rec.Header().Get("Location"); loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

func TestGallery_MissingRouteRendersPlaceholderForNonExistentCombo(t *testing.T) {
	_, _, srv := galleryFixture(t)
	q := url.Values{}
	q.Set("ax_page", "home")
	q.Set("ax_theme", "dark") // missing combo
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/missing?"+q.Encode(), nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "is-missing") || !strings.Contains(body, "No PNG on disk") {
		t.Errorf("placeholder not rendered; body=%s", body[:min(800, len(body))])
	}
}

func TestGallery_MatrixDefaultLayout(t *testing.T) {
	_, items, srv := galleryFixture(t)
	id := items[0].ID
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/screenshot/"+id+"/matrix", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	// Both row values should appear as <th class="row-head"> labels.
	if !strings.Contains(body, "row-head") {
		t.Errorf("matrix body missing row-head")
	}
	// 4 cells expected (2 rows × 2 cols); one should be missing (home, dark).
	if !strings.Contains(body, "is-missing") {
		t.Errorf("matrix body missing 'is-missing' placeholder cell")
	}
	// And the existing combo (about, dark) should be linked.
	if !strings.Contains(body, "/screenshot/"+items[0].ID+"/image.png") {
		t.Errorf("matrix body missing image link for about-dark")
	}
}

func TestGallery_MatrixRowsColsQueryParams(t *testing.T) {
	_, items, srv := galleryFixture(t)
	id := items[0].ID
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/screenshot/"+id+"/matrix?rows=theme&cols=page", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="theme" selected`) {
		t.Errorf("rows dropdown should have theme selected")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// trim is a tiny helper for clearer test failures.
func trim(s string) string {
	if len(s) > 1200 {
		return s[:1200] + "…"
	}
	return s
}
