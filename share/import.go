package share

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Dir is where imports live, under the corpus checkout.
//
// A tree of its own, and deliberately not under content/. What comes off a
// share page is raw material: it has no page map behind it, no tags, no
// reference graph, and nothing has checked it against the printed book. Every
// audit rule the corpus has runs over content/, and dropping unverified text in
// there would make all of them pass on it by accident. Promoting a file out of
// here is a separate job, with a person reading it.
const Dir = "imports"

// Target is where an import goes.
//
// The layout is <book>/chapter_<n>/<n>.<m>.md, one file to a numbered section,
// which is how the books themselves are laid out and how the corpus names
// sections everywhere else. What comes before a chapter, the introduction,
// has no number, so it sits at the top as <book>/intro.md.
type Target struct {
	Book    string
	Chapter int
	Section int
	// Intro is set for the front matter of a book, which has no numbers.
	Intro bool
}

// Path is where the file goes, relative to the corpus root.
func (t Target) Path() string {
	if t.Intro {
		return filepath.Join(Dir, t.Book, "intro.md")
	}
	return filepath.Join(Dir, t.Book,
		fmt.Sprintf("chapter_%d", t.Chapter),
		fmt.Sprintf("%d.%d.md", t.Chapter, t.Section))
}

// String is the label the target was written as.
func (t Target) String() string {
	if t.Intro {
		return "intro"
	}
	return fmt.Sprintf("%d.%d", t.Chapter, t.Section)
}

var where = regexp.MustCompile(`^(\d+)\.(\d+)$`)

// ParseTarget reads a label like "1.1" or "intro".
func ParseTarget(book, label string) (Target, error) {
	book = strings.TrimSpace(book)
	if book == "" {
		return Target{}, fmt.Errorf("no book was given, and an import has to go somewhere")
	}
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "intro" || label == "introduction" {
		return Target{Book: book, Intro: true}, nil
	}
	m := where.FindStringSubmatch(label)
	if m == nil {
		return Target{}, fmt.Errorf("%q is not a place in a book, which is either intro or a chapter and section like 1.1", label)
	}
	ch, _ := strconv.Atoi(m[1])
	sec, _ := strconv.Atoi(m[2])
	if ch == 0 || sec == 0 {
		return Target{}, fmt.Errorf("%q has a zero in it, and books are numbered from one", label)
	}
	return Target{Book: book, Chapter: ch, Section: sec}, nil
}

// title is how the conversations are named: a book, then the place in it, with
// a dash between. It is a hint and nothing more, because a person typed it, so
// it is only ever used when the command line did not say.
var title = regexp.MustCompile(`^(.*?)\s*[-–—]\s*([^-–—]+)$`)

// TargetFromTitle guesses where a conversation belongs from what it is called.
//
// Measured on the four Theory of Sets pages, which are titled "Bourbaki Sets -
// Intro" and "Bourbaki Sets - 1.1" through "1.3": all four parse. That is four
// conversations by one person on one afternoon and it is not evidence the
// habit holds, so a title that does not parse is an error asking for the label
// to be written out, not a guess.
func TargetFromTitle(book, name string) (Target, error) {
	m := title.FindStringSubmatch(strings.TrimSpace(name))
	if m == nil {
		return Target{}, fmt.Errorf("the conversation is called %q, which does not say where in the book it goes, so pass it as <label>=<url>", name)
	}
	return ParseTarget(book, m[2])
}

// Import is one rendered conversation, ready to write.
type Import struct {
	Target Target
	Page   *Page
	Conv   *Conversation
	// URL is the share link, kept whole. The id alone is not enough to go back
	// and look, and going back and looking is the point of recording any of it.
	URL string
}

// Sum is the digest of the answers as they arrived, before anything here
// touched them.
//
// Of the answers and not of the file, so that re-running an import after a
// change to the rendering shows the source is the same conversation. And not of
// the HTML, which carries a fresh nonce on every fetch and so would say the
// page had changed every time it was read.
func (im *Import) Sum() string {
	h := sha256.New()
	for _, t := range im.Conv.Turns {
		h.Write([]byte(t.Text))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Markdown is the file: front matter, then the body.
func (im *Import) Markdown() string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "book: %s\n", im.Target.Book)
	if im.Target.Intro {
		b.WriteString("intro: true\n")
	} else {
		fmt.Fprintf(&b, "chapter: %d\n", im.Target.Chapter)
		fmt.Fprintf(&b, "section: %d\n", im.Target.Section)
	}
	b.WriteString("lang: en\n")
	b.WriteString("extraction: share\n")
	fmt.Fprintf(&b, "share_url: %s\n", im.URL)
	fmt.Fprintf(&b, "share_title: %s\n", yamlString(im.Conv.Title))
	fmt.Fprintf(&b, "asks: %d\n", im.Conv.Asks)
	fmt.Fprintf(&b, "answers: %d\n", len(im.Conv.Turns))
	if len(im.Page.Models) > 0 {
		fmt.Fprintf(&b, "models: [%s]\n", strings.Join(im.Page.Models, ", "))
	}
	fmt.Fprintf(&b, "joined: %d of %d\n", joined(im.Page.Boundaries), len(im.Page.Boundaries))
	fmt.Fprintf(&b, "answers_sha256: %s\n", im.Sum())
	fmt.Fprintf(&b, "content_sha256: %s\n", sum(im.Page.Body))
	b.WriteString("---\n\n")
	b.WriteString(im.Page.Body)
	return b.String()
}

// Write puts the file where it goes and says what it wrote.
func (im *Import) Write(root string) (string, error) {
	rel := im.Target.Path()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(im.Markdown()), 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

func joined(bs []Boundary) int {
	n := 0
	for _, b := range bs {
		if b.Joined {
			n++
		}
	}
	return n
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// yamlString quotes a value only when it has to be quoted, which keeps the
// front matter reading like the rest of the corpus's front matter.
func yamlString(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, `:#'"{}[]&*!|>%@`+"\n\t") || strings.TrimSpace(s) != s {
		return strconv.Quote(s)
	}
	return s
}
