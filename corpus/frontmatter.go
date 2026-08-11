package corpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Every content file in the corpus is Markdown with a YAML block at the head,
// fenced by three dashes, the same shape taocp uses. Four schemas live in that
// block: a section, a translated section, an exercise, and a solution. The
// first two are one struct, because a translation is a section file with the
// provenance of the translation added, and keeping them apart would mean two
// definitions of the same twenty fields drifting from each other.
//
// The hash is what makes the corpus maintainable. content_sha256 is taken over
// the normalised body, so it does not move when an editor changes line endings
// or leaves a space at the end of a line, and a translation records the English
// hash it was made from. When the English changes, the translations whose
// recorded hash no longer matches are exactly the stale ones.

// SectionFrontMatter is the head of content/<lang>/<book>/<chapter>/NN_sN_*.md.
type SectionFrontMatter struct {
	Book            string       `yaml:"book"`
	BookTitle       string       `yaml:"book_title"`
	Chapter         string       `yaml:"chapter"`
	ChapterTitle    string       `yaml:"chapter_title"`
	Section         int          `yaml:"section"`
	SectionTitle    string       `yaml:"section_title"`
	Appendix        bool         `yaml:"appendix,omitempty"`
	Kind            string       `yaml:"kind,omitempty"`
	Lang            string       `yaml:"lang"`
	Source          string       `yaml:"source"`
	SourceEdition   string       `yaml:"source_edition,omitempty"`
	BookPages       string       `yaml:"book_pages,omitempty"`
	PDFPages        string       `yaml:"pdf_pages,omitempty"`
	Extraction      string       `yaml:"extraction,omitempty"`
	ExtractionModel string       `yaml:"extraction_model,omitempty"`
	Subsections     []Subsection `yaml:"subsections,omitempty"`
	Statements      int          `yaml:"statements"`
	Exercises       int          `yaml:"exercises"`
	Tags            []string     `yaml:"tags,omitempty"`
	ContentSHA256   string       `yaml:"content_sha256"`

	// Set only on a translation. TranslatedFrom is the path of the English
	// file and SourceSHA256 is its content_sha256 at the time of translation.
	TranslatedFrom   string `yaml:"translated_from,omitempty"`
	SourceSHA256     string `yaml:"source_content_sha256,omitempty"`
	TranslationModel string `yaml:"translation_model,omitempty"`
	TranslationRun   string `yaml:"translation_run,omitempty"`
	GlossaryVersion  int    `yaml:"glossary_version,omitempty"`
}

// ExerciseFrontMatter is the head of content/<lang>/<book>/<chapter>/exercises/sN/NN.md.
type ExerciseFrontMatter struct {
	Book    string `yaml:"book"`
	Chapter string `yaml:"chapter"`
	Section int    `yaml:"section"`
	// Appendix says the exercises are an appendix's rather than a §'s. Appendix
	// 1 of chapter VIII and its § 1 both number their exercises from one, so
	// without this the two sets would share a label and a path.
	Appendix bool   `yaml:"appendix,omitempty"`
	Exercise int    `yaml:"exercise"`
	Label    string `yaml:"label"`
	Tag      string `yaml:"tag,omitempty"`
	Lang     string `yaml:"lang"`
	BookPage string `yaml:"book_page,omitempty"`
	PDFPage  int    `yaml:"pdf_page,omitempty"`
	HasHint  bool   `yaml:"has_hint"`
	// Bourbaki marks the harder exercises with a pilcrow and sets some in
	// small type as supplementary. There is no numeric difficulty rating to
	// record, unlike TAOCP.
	Starred       bool     `yaml:"starred"`
	Supplementary bool     `yaml:"supplementary,omitempty"`
	Refs          []string `yaml:"refs,omitempty"`

	TranslatedFrom   string `yaml:"translated_from,omitempty"`
	SourceSHA256     string `yaml:"source_content_sha256,omitempty"`
	TranslationModel string `yaml:"translation_model,omitempty"`
}

// SolutionFrontMatter is the head of content/solutions/<lang>/....
type SolutionFrontMatter struct {
	Label       string  `yaml:"label"`
	Tag         string  `yaml:"tag,omitempty"`
	Lang        string  `yaml:"lang"`
	Status      string  `yaml:"status"`
	Model       string  `yaml:"model,omitempty"`
	Route       string  `yaml:"route,omitempty"`
	Generated   string  `yaml:"generated,omitempty"`
	Candidates  int     `yaml:"candidates,omitempty"`
	TruthJudge  string  `yaml:"truth_judge,omitempty"`
	AuditJudge  string  `yaml:"audit_judge,omitempty"`
	Corrections int     `yaml:"corrections"`
	Tokens      *Tokens `yaml:"tokens,omitempty"`
}

// Tokens is what one solution cost.
type Tokens struct {
	Prompt     int `yaml:"prompt"`
	Completion int `yaml:"completion"`
	Total      int `yaml:"total"`
}

// Solution status values.
const (
	StatusVerified    = "verified"
	StatusFailed      = "failed"
	StatusUnattempted = "unattempted"
)

// File is a content file: a front matter block and the Markdown under it.
type File[T any] struct {
	Meta T
	Body string
}

// SectionFile, ExerciseFile and SolutionFile name the three on-disk shapes.
type (
	SectionFile  = File[SectionFrontMatter]
	ExerciseFile = File[ExerciseFrontMatter]
	SolutionFile = File[SolutionFrontMatter]
)

var fenceRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---[ \t]*\r?\n?`)

// SplitFrontMatter separates the YAML head from the body. A file with no fence
// is an error rather than a file with an empty head, because a content file
// without front matter is a file the audit cannot check at all.
func SplitFrontMatter(b []byte) (head, body []byte, err error) {
	m := fenceRe.FindSubmatchIndex(b)
	if m == nil {
		return nil, nil, fmt.Errorf("no front matter: the file must start with a line of three dashes")
	}
	return b[m[2]:m[3]], b[m[1]:], nil
}

// NormalizeBody applies the body conventions: LF endings, no trailing
// whitespace on any line, nothing blank at either end, exactly one newline at
// the end. The hash is taken over the result, so an editor that strips or adds
// whitespace cannot make a translation look stale.
//
// The blank line at the head matters as much as the one at the foot. A file is
// rendered with a blank line between the fence and the body, so a body that
// kept its own leading blank line would gain another one every time the file
// was read and written back, and the hash would move with it.
func NormalizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	s = strings.Join(lines, "\n")
	s = strings.Trim(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}

// ContentSHA256 hashes the normalised body.
func ContentSHA256(body string) string {
	sum := sha256.Sum256([]byte(NormalizeBody(body)))
	return hex.EncodeToString(sum[:])
}

// ParseFile reads a content file of any of the four schemas.
func ParseFile[T any](b []byte) (File[T], error) {
	var f File[T]
	head, body, err := SplitFrontMatter(b)
	if err != nil {
		return f, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(head))
	dec.KnownFields(true)
	if err := dec.Decode(&f.Meta); err != nil {
		return f, fmt.Errorf("front matter: %w", err)
	}
	f.Body = NormalizeBody(string(body))
	return f, nil
}

// ReadFile reads and parses a content file from disk.
func ReadFile[T any](path string) (File[T], error) {
	b, err := os.ReadFile(path)
	if err != nil {
		var zero File[T]
		return zero, err
	}
	f, err := ParseFile[T](b)
	if err != nil {
		return f, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// Bytes renders a content file. The body is normalised first and, for a section
// file, content_sha256 is recomputed, so the hash in the file always describes
// the bytes under it.
func (f File[T]) Bytes() ([]byte, error) {
	f.Body = NormalizeBody(f.Body)
	if s, ok := any(&f.Meta).(*SectionFrontMatter); ok {
		s.ContentSHA256 = ContentSHA256(f.Body)
	}
	head, err := yaml.Marshal(f.Meta)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(head)
	buf.WriteString("---\n\n")
	buf.WriteString(f.Body)
	return buf.Bytes(), nil
}

// Write renders the file and writes it, creating the directory.
func (f File[T]) Write(path string) error {
	b, err := f.Bytes()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Stale reports whether a translated file was made from a different English
// body than the one now committed. This is the whole point of recording
// source_content_sha256: re-translating everything on every English edit is
// unaffordable, and re-translating nothing leaves the corpus lying.
func (m SectionFrontMatter) Stale(englishSHA256 string) bool {
	return m.TranslatedFrom != "" && m.SourceSHA256 != englishSHA256
}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns a printed title into the file-name part. It cuts on a word
// boundary rather than mid-word, so the name stays readable, and it is
// deterministic, so the same title always produces the same path.
func Slug(title string, max int) string {
	s := slugStrip.ReplaceAllString(strings.ToLower(title), "_")
	s = strings.Trim(s, "_")
	if max <= 0 || len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, "_"); i > 0 {
		return cut[:i]
	}
	return cut
}

// SlugLen is how much of a section title goes into its file name. Bourbaki
// titles run long ("Laws of composition; associativity; commutativity"), and
// the whole of one makes an unusable path.
const SlugLen = 40

// SectionPath is where a section file belongs.
//
// A § is named for its number twice over, once so the directory sorts in
// reading order and once so the name says what it is: 02_s2_the_structure_of_
// modules_of_finite_length.md. An appendix takes an A in place of the ordinal,
// A1_a1_algebras_without_unit_element.md, which sorts after every § because a
// letter sorts after a digit, and that is where the book puts it. Numbering it
// on from the last § instead would make the name of a file depend on how many
// §§ the chapter happens to have, and chapters II and III close with an
// appendix carrying no number at all.
func SectionPath(root, lang string, m SectionFrontMatter) string {
	name := fmt.Sprintf("%02d_s%d_%s.md", m.Section, m.Section, Slug(m.SectionTitle, SlugLen))
	switch m.Kind {
	case KindFront:
		// 00 puts the chapter's opening pages first, ahead of § 1, which is
		// where the book puts them.
		name = "00_frontmatter.md"
	case KindHistorical:
		// The note comes after everything, including the appendices, so its
		// name has to sort after a leading A. It is the only file here named
		// for what it is rather than for its number.
		name = "historical_note.md"
	}
	if m.Appendix {
		n := ""
		if m.Section > 0 {
			n = strconv.Itoa(m.Section)
		}
		name = fmt.Sprintf("A%s_a%s_%s.md", n, n, Slug(m.SectionTitle, SlugLen))
	}
	return filepath.Join(root, "content", lang, m.Book, m.Chapter, name)
}

// ExercisePath is where an exercise file belongs.
//
// The directory is named the way the section label is, s1 for § 1 and a1 for
// Appendix 1, so that the path and the label say the same thing.
func ExercisePath(root, lang string, m ExerciseFrontMatter) string {
	return filepath.Join(root, "content", lang, m.Book, m.Chapter,
		"exercises", ExerciseDir(m.Section, m.Appendix), fmt.Sprintf("%02d.md", m.Exercise))
}

// ExerciseDir is the directory a section's exercises go in, "s1" or "a1".
func ExerciseDir(section int, appendix bool) string {
	if appendix {
		return fmt.Sprintf("a%d", section)
	}
	return fmt.Sprintf("s%d", section)
}

// SolutionPath is where a solution belongs. It is keyed by the exercise's
// permanent label, so a solution follows its exercise even if the exercise
// moves between editions.
func SolutionPath(root, lang, label string) (string, error) {
	r, err := ParseLabel(label)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "content", "solutions", lang, r.Book,
		strings.ToUpper(r.Chapter), fmt.Sprintf("s%d", r.Section),
		fmt.Sprintf("%02d.md", r.Number)), nil
}
