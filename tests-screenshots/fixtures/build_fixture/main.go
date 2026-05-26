// Command build_fixture creates a deterministic temp git repo with a
// known set of snapshot diffs, used by the Playwright screenshot suite
// to boot a stable snapdiff session.
//
// Output layout (mirrors the user's real iOS repo):
//
//	<out>/snapdiff.toml
//	<out>/snaps/<Module>/__Snapshots__/<TestClass>/<test>.<theme>.<lang>.png
//
// PNGs are 320x568 with content keyed by path hash so files are
// visually distinct in the snapdiff UI. Modified items have a small
// per-row color shift between baseline and current.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
)

type spec struct {
	path string
	mode string // "modified" | "added" | "deleted"
}

var specs = []spec{
	// AddGroupFormViewTests — 4 modified
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedIn.dark.en.png", "modified"},
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedIn.dark.pt-BR.png", "modified"},
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedIn.light.en.png", "modified"},
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedIn.light.pt-BR.png", "modified"},
	// AddGroupFormViewTests — 4 more modified (different test)
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedInExcluded.dark.en.png", "modified"},
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedInExcluded.dark.pt-BR.png", "modified"},
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedInExcluded.light.en.png", "modified"},
	{"snaps/AddGroupFeatureTests/__Snapshots__/AddGroupFormViewTests/formViewSignedInExcluded.light.pt-BR.png", "modified"},
	// ProfileViewTests — 4 modified
	{"snaps/AuthFeatureTests/__Snapshots__/ProfileViewTests/profileSheet.dark.en.png", "modified"},
	{"snaps/AuthFeatureTests/__Snapshots__/ProfileViewTests/profileSheet.dark.pt-BR.png", "modified"},
	{"snaps/AuthFeatureTests/__Snapshots__/ProfileViewTests/profileSheet.light.en.png", "modified"},
	{"snaps/AuthFeatureTests/__Snapshots__/ProfileViewTests/profileSheet.light.pt-BR.png", "modified"},
	// ProfileViewTests — 2 added (new test variant)
	{"snaps/AuthFeatureTests/__Snapshots__/ProfileViewTests/profileSheetDevMode.dark.en.png", "added"},
	{"snaps/AuthFeatureTests/__Snapshots__/ProfileViewTests/profileSheetDevMode.light.en.png", "added"},
	// GroupListViewTests — 4 modified
	{"snaps/GroupListFeatureTests/__Snapshots__/GroupListViewTests/populatedStateOnline.dark.en.png", "modified"},
	{"snaps/GroupListFeatureTests/__Snapshots__/GroupListViewTests/populatedStateOnline.dark.pt-BR.png", "modified"},
	{"snaps/GroupListFeatureTests/__Snapshots__/GroupListViewTests/populatedStateOnline.light.en.png", "modified"},
	{"snaps/GroupListFeatureTests/__Snapshots__/GroupListViewTests/populatedStateOnline.light.pt-BR.png", "modified"},
	// SignInViewTests — 1 deleted
	{"snaps/AuthFeatureTests/__Snapshots__/SignInViewTests/signInSheet.dark.en.png", "deleted"},
}

const snapdiffTOML = `[snapshots]
globs = ["snaps/**/*.png"]
axis_regex = '__Snapshots__/(?P<testClass>[^/]+)/(?P<test>[^.]+)\.(?P<theme>[^.]+)\.(?P<lang>[^/]+)\.png'

[server]
bind = "127.0.0.1:0"
linger_seconds = 0
`

func main() {
	out := flag.String("out", "", "output directory (must not exist)")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "fixture: --out is required")
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		die(err)
	}

	// Initialize git repo.
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "fixture@example.com"},
		{"config", "user.name", "fixture"},
		{"config", "commit.gpgsign", "false"},
	} {
		if err := runGit(*out, args...); err != nil {
			die(err)
		}
	}

	// Phase 1: write baseline versions for everything that should have a
	// baseline (modified + deleted), commit them.
	for _, s := range specs {
		if s.mode == "added" {
			continue
		}
		full := filepath.Join(*out, s.path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			die(err)
		}
		if err := writePNG(full, colorFor(s.path, 0)); err != nil {
			die(err)
		}
	}
	if err := runGit(*out, "add", "-A"); err != nil {
		die(err)
	}
	if err := runGit(*out, "commit", "-q", "-m", "fixture baseline"); err != nil {
		die(err)
	}

	// Phase 2: apply mutations to produce the working-tree state.
	for _, s := range specs {
		full := filepath.Join(*out, s.path)
		switch s.mode {
		case "modified":
			if err := writePNG(full, colorFor(s.path, 1)); err != nil {
				die(err)
			}
		case "added":
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				die(err)
			}
			if err := writePNG(full, colorFor(s.path, 1)); err != nil {
				die(err)
			}
		case "deleted":
			if err := os.Remove(full); err != nil {
				die(err)
			}
		}
	}

	// Write the fixture's snapdiff.toml.
	if err := os.WriteFile(filepath.Join(*out, "snapdiff.toml"), []byte(snapdiffTOML), 0o644); err != nil {
		die(err)
	}

	fmt.Println(*out)
}

// colorFor produces a deterministic RGB seeded by path + variant.
// variant=0 = baseline, variant=1 = current.
func colorFor(path string, variant int) color.RGBA {
	h := sha256.Sum256([]byte(path))
	r := h[0]
	g := h[1]
	b := h[2]
	if variant == 1 {
		// shift channels modestly so the diff is visible but the seed
		// color is recognizable on both sides
		r += 60
		g -= 30
		b += 20
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// writePNG renders a 320x568 image with a deterministic banded layout
// keyed by the supplied base color: a darker header strip across the
// top, a body wash, and four content cells.
func writePNG(path string, base color.RGBA) error {
	const w, h = 320, 568
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	wash := dim(base, 0.92)
	header := dim(base, 0.55)
	cell := dim(base, 0.75)
	accent := dim(base, 0.40)

	fillRect(img, 0, 0, w, h, wash)
	fillRect(img, 0, 0, w, 80, header)
	for row := 0; row < 4; row++ {
		y := 120 + row*100
		fillRect(img, 20, y, w-20, y+60, cell)
		fillRect(img, 30, y+10, 36, y+50, accent)
	}
	fillRect(img, 20, h-70, w-20, h-20, accent)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func fillRect(dst *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > dst.Bounds().Dx() {
		x1 = dst.Bounds().Dx()
	}
	if y1 > dst.Bounds().Dy() {
		y1 = dst.Bounds().Dy()
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			dst.SetRGBA(x, y, c)
		}
	}
}

// dim scales a color toward black (factor < 1) or returns it (factor=1).
func dim(c color.RGBA, factor float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c.R) * factor),
		G: uint8(float64(c.G) * factor),
		B: uint8(float64(c.B) * factor),
		A: c.A,
	}
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "fixture:", err)
	os.Exit(1)
}
