package corpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	// Errata are the printing's errors in the body of this §, under the § label
	// rather than under the statement they fall in. A § is one file and the
	// correction has to be found from the file, and a misprint is as often in the
	// prose between two statements as in a statement: § 5 of chapter VIII says
	// "Chap. VII, §13, no. 1" in a paragraph belonging to no statement at all.
	Errata        []Erratum `yaml:"errata,omitempty"`
	ContentSHA256 string    `yaml:"content_sha256"`

	// Set only on a translation. TranslatedFrom is the path of the English
	// file and SourceSHA256 is its content_sha256 at the time of translation.
	TranslatedFrom   string `yaml:"translated_from,omitempty"`
	SourceSHA256     string `yaml:"source_content_sha256,omitempty"`
	TranslationModel string `yaml:"translation_model,omitempty"`
	TranslationRun   string `yaml:"translation_run,omitempty"`
	GlossaryVersion  int    `yaml:"glossary_version,omitempty"`
	// GlossaryTerms is the digest of the terminology this section was shown,
	// which is the glossary rows its English mentions and not the whole file.
	// It is what staleness is decided on, because glossary_version moves for
	// every edit anywhere and would restale a section over a phrase that does
	// not occur in it. The version stays for the record and for files written
	// before this field existed.
	GlossaryTerms string `yaml:"glossary_terms_sha256,omitempty"`
	// PromptSHA256 is the hash of the instructions this file was made with,
	// the common half and the language half together. It is the third kind of
	// staleness, beside the English changing and the glossary bumping: edit
	// the prompt and every file that carries the old hash was translated
	// under rules that no longer hold.
	PromptSHA256 string `yaml:"prompt_sha256,omitempty"`
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
	Starred       bool      `yaml:"starred"`
	Supplementary bool      `yaml:"supplementary,omitempty"`
	Refs          []string  `yaml:"refs,omitempty"`
	Errata        []Erratum `yaml:"errata,omitempty"`

	// Set only on a translation, and the same five facts a translated section
	// records. An exercise is a shorter file than a §, and it is held to the
	// same terminology and the same instructions, so staleness has to be
	// decided the same way: the English it was made from, the glossary rows it
	// was shown, and the prompt it was asked under.
	//
	// SourceSHA256 is taken over the English body here rather than copied out
	// of the English file's front matter, which is what a section does. An
	// exercise records no content_sha256 of its own, and adding one would mean
	// rewriting all 239 English files of this book to record a number that can
	// be computed from the body sitting under it.
	TranslatedFrom   string `yaml:"translated_from,omitempty"`
	SourceSHA256     string `yaml:"source_content_sha256,omitempty"`
	TranslationModel string `yaml:"translation_model,omitempty"`
	TranslationRun   string `yaml:"translation_run,omitempty"`
	GlossaryVersion  int    `yaml:"glossary_version,omitempty"`
	GlossaryTerms    string `yaml:"glossary_terms_sha256,omitempty"`
	PromptSHA256     string `yaml:"prompt_sha256,omitempty"`
}

// Erratum is a place where the printed page is wrong and the corpus keeps the
// printed words anyway.
//
// The corpus transcribes a printing. When the printing has an error in it the
// transcription has the same error, and correcting the text in place would be a
// silent edit of a source somebody may hold in their hands, which is the one
// thing this corpus must not do. So the words stay and the correction goes here,
// beside them, with what the page says, what it has to say for the statement to
// hold, and the evidence.
//
// The first of these is exercise 3 a) of chapter VIII, § 1. The 2023 English
// translation asks for a field K finite-dimensional over a subfield K'
// isomorphic to it, and then asks the reader to deduce that a ring built out of
// K is neither right Artinian nor right Noetherian. Its right ideals are the
// K'-subspaces of K, so the deduction holds exactly when K is infinite over K',
// and the French of 1981 says "de dimension infinie". The English is a slip in
// translation, and it is not harmless: the solver read it, wrote a solution to
// exercise 4 b) on top of it, and both judges passed it.
type Erratum struct {
	// Says is what the printed page has, quoted.
	Says string `yaml:"says"`
	// Read is what it has to be for the statement to hold.
	Read string `yaml:"read"`
	// Why is the evidence, in one sentence. A correction with no reason on it
	// is an opinion about a book, and this corpus does not hold those.
	Why string `yaml:"why"`
}

// SolutionFrontMatter is the head of content/solutions/<lang>/....
type SolutionFrontMatter struct {
	Label  string `yaml:"label"`
	Tag    string `yaml:"tag,omitempty"`
	Lang   string `yaml:"lang"`
	Status string `yaml:"status"`
	// Parts is the per-part verdict of a multi-part exercise, and it is what
	// makes partial mean something. A Bourbaki exercise runs to a) through h)
	// often enough that one verdict over the whole of it would be either a lie
	// about the parts that failed or a waste of the parts that did not.
	Parts []Part `yaml:"parts,omitempty"`
	// Uses is the tags of the results the solution leans on, which is how the
	// corpus learns which of its statements are load-bearing. It is checked:
	// a tag that is in no line of tags is a result the solution invented.
	Uses        []string `yaml:"uses,omitempty"`
	Model       string   `yaml:"model,omitempty"`
	Route       string   `yaml:"route,omitempty"`
	Generated   string   `yaml:"generated,omitempty"`
	Candidates  int      `yaml:"candidates,omitempty"`
	TruthJudge  string   `yaml:"truth_judge,omitempty"`
	AuditJudge  string   `yaml:"audit_judge,omitempty"`
	Corrections int      `yaml:"corrections"`
	// Reviewed is when the judges last read this solution again without
	// rewriting it. It is not Generated: a solution written in March and
	// re-judged in August under a better prompt is a different thing from one
	// written in August, and a file carrying one date for both cannot say which
	// it is.
	Reviewed string  `yaml:"reviewed,omitempty"`
	Tokens   *Tokens `yaml:"tokens,omitempty"`
	// HandRead is when a person read this solution against the exercise and the
	// § it belongs to, and Found is what they found, one line to a finding.
	//
	// Neither of them moves Status. Status is what the pipeline decided and
	// the corpus says so plainly; a reader who disagrees with it writes down
	// why here and the disagreement stays visible instead of being swallowed
	// by an edit. Part d) of exercise 6 of § 1 is the case that wanted this:
	// the judge failed it for citing definitions it had not been shown, which
	// was true and was the trimming's fault and not the solution's, and the
	// proof is sound. That is a fact about the run worth keeping, and it is
	// not the same fact as the solution having passed.
	HandRead string   `yaml:"hand_read,omitempty"`
	Found    []string `yaml:"found,omitempty"`
	// PromptSHA256 identifies the instructions the solution was written under,
	// as it does for a translated section. A prompt that gets better is a
	// reason to solve an exercise again, and a solution that cannot say which
	// prompt made it cannot be told apart from one made under the new rules.
	PromptSHA256 string `yaml:"prompt_sha256,omitempty"`
}

// Part is one lettered part of a multi-part exercise, a) or b), with the
// verdict that part alone got. Reason is why it did not pass, in a sentence,
// and it is empty on a part that did.
type Part struct {
	ID     string `yaml:"id"`
	Status string `yaml:"status"`
	Reason string `yaml:"reason,omitempty"`
}

// Tokens is what one solution cost.
type Tokens struct {
	Prompt     int `yaml:"prompt"`
	Completion int `yaml:"completion"`
	Total      int `yaml:"total"`
}

// Solution status values, spec 07 §3.2.
//
// There is no "failed". A solution can fail to be written, fail to be believed,
// or fail to be possible, and those are three different facts about the corpus.
// An exercise that cites a Book this corpus does not hold is blocked and that is
// the fault of the corpus; an exercise that asks the reader to explore is open
// and that is the fault of nobody; a solution a judge threw out is unverified
// and that one is the model's. Folding the three into one word would let a
// scorecard round them all up as tried and failed, and a scorecard that claims
// every Bourbaki exercise was either solved or flunked would be a lie whichever
// way it leaned.
const (
	StatusVerified    = "verified"    // both judges passed
	StatusPartial     = "partial"     // some parts passed and others did not
	StatusUnverified  = "unverified"  // a judge rejected it after the correction budget
	StatusBlocked     = "blocked"     // it turns on a Book the corpus does not hold
	StatusOpen        = "open"        // it asks for exploration rather than a proof
	StatusUnattempted = "unattempted" // not yet run
)

// Statuses is every status a solution may carry, in the order a scorecard
// reads best: what worked, then what worked in part, then the three ways it did
// not, then what was never tried.
var Statuses = []string{
	StatusVerified, StatusPartial, StatusUnverified,
	StatusBlocked, StatusOpen, StatusUnattempted,
}

// ValidStatus reports whether s is one of them.
func ValidStatus(s string) bool { return slices.Contains(Statuses, s) }

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

// BodyStart is the file line that line one of the parsed body sits on, counting
// both from one.
//
// A rule, or a renderer, works on the body and knows a body line. The reader it
// reports to has the file open in an editor and knows a file line, and the two
// differ by the front matter and by the blank line under it that NormalizeBody
// trims. This is the only place that difference is worked out, because two
// answers to "which line is this" is one answer too many.
func BodyStart(raw []byte) int {
	lines := bytes.Split(raw, []byte("\n"))
	line, fences := 0, 0
	for i, l := range lines {
		if string(bytes.TrimRight(l, " \t\r")) == "---" {
			fences++
			if fences == 2 {
				line = i + 2 // the line after the closing fence, counting from one
				break
			}
		}
	}
	if fences < 2 {
		return 1
	}
	for line <= len(lines) && strings.TrimSpace(string(lines[line-1])) == "" {
		line++
	}
	return line
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
	buf.WriteString("---\n")
	// A blank page has front matter and nothing else, and the eleven blank
	// versos of chapter VIII are eleven real files. The blank line that
	// separates the fence from the text has no text to separate it from there,
	// so writing it would leave every one of them ending in a blank line, which
	// is what H05 is for.
	if f.Body != "" {
		buf.WriteString("\n")
		buf.WriteString(f.Body)
	}
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

// fold is the letter an accented letter is filed under. Dropping the accent
// instead of folding it loses the letter with it, and the French volumes are
// full of them: § 13 of Algebra VIII is "Algèbres absolument semi-simples" and
// came out as 13_s13_alg_bres_absolument_semi_simples. The table is the Latin-1
// letters the Éléments are set in and the two ligatures, which is every letter
// the French, and the German and Scandinavian names in the historical notes,
// actually use.
var fold = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "á", "a", "ã", "a", "å", "a", "æ", "ae",
	"ç", "c",
	"è", "e", "é", "e", "ê", "e", "ë", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i",
	"ñ", "n",
	"ò", "o", "ó", "o", "ô", "o", "ö", "o", "õ", "o", "ø", "o", "œ", "oe",
	"ù", "u", "ú", "u", "û", "u", "ü", "u",
	"ý", "y", "ÿ", "y",
	"ß", "ss",
)

// Slug turns a printed title into the file-name part. It cuts on a word
// boundary rather than mid-word, so the name stays readable, and it is
// deterministic, so the same title always produces the same path.
func Slug(title string, max int) string {
	s := slugStrip.ReplaceAllString(fold.Replace(strings.ToLower(title)), "_")
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
//
// The directory is named the way the label is, s1 for § 1 and a1 for Appendix
// 1, as the exercises themselves are. Naming both s1 would file the solution to
// Exercise 1 of Appendix 1 of chapter VIII over the solution to Exercise 1 of
// its § 1, and the corpus would hold one solution to two exercises with nothing
// to say which one it answered.
func SolutionPath(root, lang, label string) (string, error) {
	r, err := ParseLabel(label)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "content", "solutions", lang, r.Book,
		strings.ToUpper(r.Chapter), ExerciseDir(r.Section, r.Appendix),
		fmt.Sprintf("%02d.md", r.Number)), nil
}
