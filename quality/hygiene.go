package quality

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/textguard"
)

// The hygiene rules are about the repository rather than about the corpus, and
// they are the ones with a consequence outside this project. The volumes are
// copyright Springer and N. Bourbaki, the page renders are scans of them, and
// tamnd/bourbaki is public. .gitignore keeps out the ones nobody adds by hand;
// these are what fail the build when somebody does.
//
// Every one of them reads git ls-files rather than the directory. A developer's
// checkout has the PDFs in it and the page images beside them, and a rule that
// walked the directory would be red on every machine that can actually build
// the corpus and green in CI, which is exactly backwards.

func init() {
	register(
		Check{ID: "H01", Group: Hygiene, Hard: true,
			Title: "no PDF is tracked", Run: h01, Need: needGit},
		Check{ID: "H02", Group: Hygiene, Hard: true,
			Title: "no page image is tracked outside figures/", Run: h02, Need: needGit},
		Check{ID: "H03", Group: Hygiene, Hard: true,
			Title: "no tracked file over 512 KB", Run: h03, Need: needGit},
		Check{ID: "H04", Group: Hygiene, Hard: true,
			Title: "nothing secret-shaped is tracked", Run: h04, Need: needGit},
		Check{ID: "H05", Group: Hygiene, Hard: true,
			Title: "LF endings, one trailing newline, no trailing white space", Run: h05, Need: needGit},
		Check{ID: "H06", Group: Hygiene, Hard: true,
			Title: "the README coverage table is the one the corpus has", Run: h06},
		Check{ID: "H07", Group: Hygiene, Hard: true,
			Title: "no content file carries a provider's own markup", Run: h07},
	)
}

// H01. No PDF is tracked.
//
// The volumes are not ours to publish and never will be. This is the rule the
// whole layout is arranged around: pdf/ is gitignored, extraction reads it and
// nothing downstream does, and assembly, tagging, the reference graph and every
// rule in this package are pure functions of the Markdown for exactly this
// reason.
func h01(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, p := range c.Tracked {
		if strings.EqualFold(filepath.Ext(p), ".pdf") {
			out = append(out, Finding{File: p, Msg: "is a PDF and is tracked"})
		}
	}
	return out, nil
}

// imageExts are the page render formats. JBIG2 comes out of the scans and the
// renderer writes PNG, so those two matter most, but the rule is about any
// raster.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".tif": true, ".tiff": true,
	".ppm": true, ".pgm": true, ".pbm": true, ".jb2": true, ".webp": true,
}

// H02. No page image is tracked outside figures/.
//
// figures/ is the one place a raster is allowed, and what is allowed there is a
// cropped diagram. The F rules are what say the crop is a crop. This one is
// about everywhere else: 1253 page renders sit under images/ in a working
// checkout, and every one of them is a scan of a page of a copyrighted book.
func h02(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, p := range c.Tracked {
		if !imageExts[strings.ToLower(filepath.Ext(p))] {
			continue
		}
		if strings.HasPrefix(filepath.ToSlash(p), "figures/") {
			continue
		}
		out = append(out, Finding{File: p, Msg: "is a page image and is tracked outside figures/"})
	}
	return out, nil
}

// trackedMax is the size a tracked file is held to.
//
// It is the number spec 08 gives, and the corpus went over it in one place: the
// reference graph, at 704 KB. That was answered by changing the file rather
// than the rule, since nothing reads the manifest back except the byte
// comparison that decides whether it is stale, so its layout was free to move.
// One edge to a line instead of json.MarshalIndent brought it to 510 KB.
//
// That was room and not headroom. The size of the graph is a linear function of
// how much of the Éléments has been read in, chapter VIII is one of eight
// chapters in scope, and 510 KB is under the limit by a page and a half, so the
// next hundred references would have taken it over. The graph is now one file
// to a §, whose size does not grow with the corpus: the largest of the
// twenty-six is 42 KB. The limit is left where spec 08 put it, and it holds a
// generated manifest to the same size as anything else, which is the point.
//
// The table of contents went the same way and for the same reason, at 587 KB
// with twenty-eight of the forty volumes read. It is now one file to a volume
// under manifests/toc/, and the largest of those is under 60 KB. Two generated
// manifests have now hit this rule and both were answered by splitting the file
// along the grain of what generates it, which is the shape to reach for when a
// third one does.
const trackedMax = 512 << 10

// H03. No tracked file over 512 KB.
func h03(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, p := range c.Tracked {
		st, err := os.Stat(filepath.Join(c.Root, filepath.FromSlash(p)))
		if err != nil {
			continue // a tracked file that is not in the working tree is git's business
		}
		if st.Size() > trackedMax {
			out = append(out, Finding{File: p,
				Msg: fmt.Sprintf("%s, over the %s a tracked file is held to",
					bytesOf(st.Size()), bytesOf(trackedMax))})
		}
	}
	return out, nil
}

// secrets are the shapes a credential takes. They are matched on the committed
// bytes and not on a file list, because the way a key gets into a repository is
// somebody pasting it into a note and not somebody committing a key file.
var secrets = []struct {
	name string
	re   *regexp.Regexp
}{
	{"an OpenAI key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
	{"a private key", regexp.MustCompile(`-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----`)},
	{"a GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{30,}`)},
	{"an AWS access key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"a Slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"an Anthropic key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`)},
}

// H04. Nothing secret-shaped is tracked.
func h04(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, p := range c.Tracked {
		b, ok := c.textOf(p)
		if !ok {
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, s := range secrets {
				if s.re.MatchString(line) {
					out = append(out, Finding{File: p, Line: i + 1,
						Msg: "looks like " + s.name})
					break
				}
			}
		}
	}
	return out, nil
}

// H05. LF endings, one trailing newline, no trailing white space.
//
// Not a matter of taste. content_sha256 is taken over the normalised body so
// that an editor cannot make a translation look stale, and the normalisation
// only holds if what is committed is already normal. A file that arrives with
// CRLF hashes to the same thing and diffs against everything.
func h05(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, p := range c.Tracked {
		b, ok := c.textOf(p)
		if !ok || len(b) == 0 {
			continue
		}
		if i := bytes.IndexByte(b, '\r'); i >= 0 {
			out = append(out, Finding{File: p, Line: 1 + bytes.Count(b[:i], []byte("\n")),
				Msg: "has a carriage return in it"})
			continue
		}
		if b[len(b)-1] != '\n' {
			out = append(out, Finding{File: p, Line: 1 + bytes.Count(b, []byte("\n")),
				Msg: "does not end with a newline"})
		} else if len(b) > 1 && b[len(b)-2] == '\n' {
			out = append(out, Finding{File: p, Line: bytes.Count(b, []byte("\n")),
				Msg: "ends with a blank line"})
		}
		for i, line := range strings.Split(string(b), "\n") {
			if line != strings.TrimRight(line, " \t") {
				out = append(out, Finding{File: p, Line: i + 1, Msg: "has white space at the end of the line"})
				break
			}
		}
	}
	return out, nil
}

// textOf reads a tracked file and says whether it is text. A NUL byte in the
// first few kilobytes is the test git itself uses, and it is enough: the only
// binaries this repository is meant to hold are the figures.
func (c *Corpus) textOf(rel string) ([]byte, bool) {
	if imageExts[strings.ToLower(filepath.Ext(rel))] || strings.EqualFold(filepath.Ext(rel), ".pdf") {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(c.Root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, false
	}
	head := b
	if len(head) > 8000 {
		head = head[:8000]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return nil, false
	}
	return b, true
}

// H06. Every generated block of the README is the one the corpus has.
//
// The README is the only thing most people will read, and a number in it that
// says the corpus holds more than it does is the one wrong statement in this
// project that has an audience. The library table, the text layer table, the
// coverage table and the rule count are all generated between markers, so the
// check is that regenerating them changes nothing.
//
// It names the block rather than saying the README is stale, because four of
// them move for four different reasons: the library when a volume is
// registered, the text layer when one is measured, coverage when a section is
// extracted, and the rules when one is written.
func h06(c *Corpus) ([]Finding, error) {
	path := filepath.Join(c.Root, "README.md")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Finding{{File: "README.md", Msg: "there is no README"}}, nil
	}
	if err != nil {
		return nil, err
	}
	stale, missing := StaleREADME(c, string(b))
	var out []Finding
	for _, name := range missing {
		out = append(out, Finding{File: "README.md",
			Msg: fmt.Sprintf("has no %s block, so those numbers are not generated and cannot be checked",
				BeginMarker(name))})
	}
	if len(stale) > 0 {
		out = append(out, Finding{File: "README.md",
			Msg: fmt.Sprintf("the %s block is not what the corpus says, run bourbaki report readme -write",
				strings.Join(stale, ", "))})
	}
	return out, nil
}

// H07. No content file carries a provider's own markup.
//
// This one was written after the corpus had already shipped an example of it. A
// retranslation of the appendix on the Nullstellensatz came back wrapped in a
// :::writing fence, was accepted by every check on the translate path, and was
// written to content/vi. All seven translation rules passed it: the mathematics
// matched span for span, the tags matched, the heading tree matched, and the
// block count matched because the fence lines carried no blank line around them
// and so joined the paragraphs either side. It was found by reading the diff,
// which is not a control.
//
// The test is textguard's, which is the same one the OCR path and the translate
// path use on an answer before it is written. Having it here as well is not
// redundant: the guards run on what a model said, this runs on what the corpus
// holds, and the corpus is what a reader gets. A file can also be written by
// hand, or by a version of the tool that predates the guard, and this is what
// catches those.
//
// Hard. There is no version of a directive fence or a citation anchor that
// belongs in a book of algebra.
func h07(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		for _, leak := range textguard.Check(d.Body) {
			if leak.Kind != "markup" {
				continue
			}
			out = append(out, Finding{File: d.Path, Line: d.BodyLine(leak.Line), Msg: leak.Detail})
		}
	}
	return out, nil
}
