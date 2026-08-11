package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Figures are the one place this corpus keeps a binary, and the rules are about
// keeping that door only as wide open as it has to be. A diagram Bourbaki
// prints cannot be transcribed into TeX in any honest way, so a crop of it is
// committed; a page image dropped in because the OCR of the page was hard is a
// scan of a copyrighted book in a public repository, and the two look identical
// to git.
//
// Chapter VIII of Algebra has no figures. Every rule here runs and every one of
// them finds nothing, which is the correct answer and not a rule that does not
// work: figures/ holds a .gitkeep and no content file references an image. The
// rules earn their place when chapters I to VII arrive, since those volumes do
// have diagrams.

func init() {
	register(
		Check{ID: "F01", Group: Figures, Hard: true,
			Title: "every figure a file references exists", Run: f01},
		Check{ID: "F02", Group: Figures, Hard: true,
			Title: "no figure under 100 by 100 pixels", Run: f02},
		Check{ID: "F03", Group: Figures, Hard: true,
			Title: "no figure over 512 KB", Run: f03},
		Check{ID: "F04", Group: Figures, Hard: true,
			Title: "nothing under figures/ is untracked", Run: f04, Need: needGit},
		Check{ID: "F05", Group: Figures, Hard: true,
			Title: "no two figures with the same bytes", Run: f05},
		Check{ID: "F06", Group: Figures, Hard: true,
			Title: "no full page used as a figure", Run: f06},
	)
}

// imageRE is a Markdown image, ![alt](path).
var imageRE = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)`)

// F01. Every figure a file references exists.
//
// The path is resolved against the corpus root when it is absolute in the repo
// sense and against the file's own directory otherwise, which is what a
// Markdown renderer does and therefore the only resolution that can be checked
// against what a reader will see.
func f01(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		for i, line := range strings.Split(d.Body, "\n") {
			for _, m := range imageRE.FindAllStringSubmatch(line, -1) {
				ref := m[1]
				if strings.Contains(ref, "://") {
					out = append(out, Finding{File: d.Path, Line: d.BodyLine(i + 1),
						Msg: "the figure is a URL, so the corpus does not hold it: " + ref})
					continue
				}
				target := ref
				if !strings.HasPrefix(ref, "/") {
					target = path.Join(path.Dir(d.Path), ref)
				}
				target = strings.TrimPrefix(target, "/")
				if _, err := os.Stat(filepath.Join(c.Root, filepath.FromSlash(target))); err != nil {
					out = append(out, Finding{File: d.Path, Line: d.BodyLine(i + 1),
						Msg: fmt.Sprintf("references %s and there is no such file", target)})
				}
			}
		}
	}
	return out, nil
}

// figureInfo is what the other five rules need out of a figure, read once.
type figureInfo struct {
	path   string
	size   int64
	width  int
	height int
	sha    string
	err    error
}

func (c *Corpus) figureInfo() []figureInfo {
	out := make([]figureInfo, 0, len(c.Figures))
	for _, rel := range c.Figures {
		fi := figureInfo{path: rel}
		full := filepath.Join(c.Root, filepath.FromSlash(rel))
		st, err := os.Stat(full)
		if err != nil {
			fi.err = err
			out = append(out, fi)
			continue
		}
		fi.size = st.Size()
		b, err := os.ReadFile(full)
		if err != nil {
			fi.err = err
			out = append(out, fi)
			continue
		}
		sum := sha256.Sum256(b)
		fi.sha = hex.EncodeToString(sum[:])
		if cfg, _, err := image.DecodeConfig(strings.NewReader(string(b))); err == nil {
			fi.width, fi.height = cfg.Width, cfg.Height
		}
		out = append(out, fi)
	}
	return out
}

// F02. No figure under 100 by 100 pixels.
//
// A crop that small is a piece of a diagram rather than a diagram, and it is
// what a cropping pass produces when it locks on to the wrong bounding box.
func f02(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, fi := range c.figureInfo() {
		if fi.width == 0 && fi.height == 0 {
			continue // not an image this build can decode, which F06 reports
		}
		if fi.width < 100 || fi.height < 100 {
			out = append(out, Finding{File: fi.path,
				Msg: fmt.Sprintf("%d by %d pixels, which is a piece of a diagram and not one", fi.width, fi.height)})
		}
	}
	return out, nil
}

// F03. No figure over 512 KB.
//
// A cropped line diagram at a sane resolution is tens of kilobytes. Half a
// megabyte means either a photograph of a page or a crop nobody downsampled.
func f03(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, fi := range c.figureInfo() {
		if fi.size > figureMax {
			out = append(out, Finding{File: fi.path,
				Msg: fmt.Sprintf("%s, over the %s a cropped diagram is held to", bytesOf(fi.size), bytesOf(figureMax))})
		}
	}
	return out, nil
}

const figureMax = 512 << 10

// needGit says why a rule that reads the index did not run.
func needGit(c *Corpus) string {
	if c.TrackedErr != nil {
		return c.TrackedErr.Error()
	}
	return ""
}

// F04. Nothing under figures/ is untracked.
//
// figures/ is the one directory .gitignore does not cover, so a stray file in
// it is a file that will be committed by the next git add and a file nobody
// meant to publish. This is the check that the directory holds figures and not
// scratch.
func f04(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, rel := range c.Figures {
		if !c.isTracked(rel) {
			out = append(out, Finding{File: rel,
				Msg: "is in figures/ and is not tracked, so it is scratch or it is about to be committed by accident"})
		}
	}
	return out, nil
}

// F05. No two figures with the same bytes.
//
// A duplicate crop means the same diagram is committed twice under two names,
// and every reference to one of them is a reference nobody will update when the
// diagram is redone. taocp's audit found real ones, which is why it is here.
func f05(c *Corpus) ([]Finding, error) {
	by := map[string][]string{}
	for _, fi := range c.figureInfo() {
		if fi.sha != "" {
			by[fi.sha] = append(by[fi.sha], fi.path)
		}
	}
	var out []Finding
	for _, sha := range sortedStrings(by) {
		paths := by[sha]
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		for _, p := range paths[1:] {
			out = append(out, Finding{File: p,
				Msg: "byte for byte the same figure as " + paths[0]})
		}
	}
	return out, nil
}

// F06. No full page used as a figure.
//
// This is the rule that matters most and the only one here that is about
// copyright rather than about tidiness. When a page will not transcribe, the
// tempting repair is to crop the whole page and reference the image, and the
// result is a scan of a page of a copyrighted book committed to a public
// repository under the name of a figure.
//
// A page render is recognised by its shape and its size together. The volumes
// are rendered at 300 dpi and up, so a full page is at least 2000 pixels down
// the long side and its aspect ratio is somewhere near a printed page. A
// diagram Bourbaki sets in the text is a fraction of a column.
func f06(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, fi := range c.figureInfo() {
		if fi.err != nil {
			out = append(out, Finding{File: fi.path, Msg: "cannot be read: " + fi.err.Error()})
			continue
		}
		if fi.width == 0 || fi.height == 0 {
			out = append(out, Finding{File: fi.path,
				Msg: "is in figures/ and is not an image this build can decode"})
			continue
		}
		long, short := fi.height, fi.width
		if long < short {
			long, short = short, long
		}
		ratio := float64(long) / float64(short)
		if long >= 2000 && ratio > 1.2 && ratio < 1.7 {
			out = append(out, Finding{File: fi.path,
				Msg: fmt.Sprintf("%d by %d is the shape and the size of a rendered page, not of a cropped diagram",
					fi.width, fi.height)})
		}
	}
	return out, nil
}

func bytesOf(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
