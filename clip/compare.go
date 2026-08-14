package clip

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/tamnd/bourbaki-solver/ocr"
)

// Verdict is what a comparison of the two readings came to.
type Verdict string

const (
	// Agree means the two readings say the same thing once the ways of writing
	// the same mathematics are folded together.
	Agree Verdict = "agree"
	// Differ means they do not, and somebody has to read the clip.
	Differ Verdict = "differ"
	// Silent means the clip went out and nothing came back. It is not a
	// disagreement and it is not an agreement, and counting it as either would
	// make a fleet that dropped half a batch look like a result.
	Silent Verdict = "silent"
)

// Row is one clip judged.
type Row struct {
	Page    int     `json:"page"`
	Line    int     `json:"line"`
	Name    string  `json:"name"`
	Verdict Verdict `json:"verdict"`
	Native  string  `json:"native"`
	Model   string  `json:"model,omitempty"`
	// At is where the two normalised readings first part company, as an index
	// into the normalised text, or -1. It is what makes a long line's
	// disagreement findable without reading the whole line twice.
	At int `json:"at,omitempty"`
	// Lost is the words the model read and we have not, and Extra the words we
	// have and it did not. They are filled in for a page and left empty for a
	// line, because a page is three hundred words and the character the two
	// readings first differ at says almost nothing about it. Which words are in
	// one reading and not the other is the whole of what a page audit is, and it
	// is the same question the extractor is already audited with.
	Lost  []string `json:"lost,omitempty"`
	Extra []string `json:"extra,omitempty"`
}

// Report is a whole comparison.
type Report struct {
	Book      string    `json:"book"`
	DPI       int       `json:"dpi"`
	Match     string    `json:"match,omitempty"`
	Generated time.Time `json:"generated"`
	Clips     int       `json:"clips"`
	Agreed    int       `json:"agreed"`
	Differed  int       `json:"differed"`
	SilentN   int       `json:"silent"`
	Rows      []Row     `json:"rows"`
}

// Compare reads the answers a fleet returned and judges each clip against the
// reading the extractor had pinned in the index.
func Compare(index Index, answers string) (Report, error) {
	report := Report{
		Book: index.Book, DPI: index.DPI, Match: index.Match,
		Generated: time.Now().UTC(), Clips: len(index.Targets),
	}
	for _, target := range index.Targets {
		row := Row{Page: target.Page, Line: target.Line, Name: target.Name, Native: target.Native, At: -1}
		text, err := ReadAnswer(filepath.Join(answers, answerName(target.Name)))
		switch {
		case err != nil:
			row.Verdict = Silent
			report.SilentN++
		default:
			row.Model = text
			row.At = Diff(target.Native, text)
			if target.Whole() {
				row.Lost, row.Extra = words(target.Native, text, target.Head)
			}
			switch {
			case target.Whole() && len(row.Lost) == 0 && len(row.Extra) == 0:
				// A page is judged on its words and not on its characters. Two
				// readings of three hundred words never agree character for
				// character, and holding them to that would report every page
				// and rank none of them.
				row.Verdict = Agree
				report.Agreed++
			case !target.Whole() && row.At < 0:
				row.Verdict = Agree
				report.Agreed++
			default:
				row.Verdict = Differ
				report.Differed++
			}
		}
		report.Rows = append(report.Rows, row)
	}
	sort.SliceStable(report.Rows, func(i, j int) bool {
		if report.Rows[i].Page != report.Rows[j].Page {
			return report.Rows[i].Page < report.Rows[j].Page
		}
		return report.Rows[i].Line < report.Rows[j].Line
	})
	return report, nil
}

// MinWord is how long a word has to be before its absence is worth printing.
//
// Four rather than crosscheck's five. That package is comparing against
// pdftotext, whose reading breaks around a formula in a different place from
// ours and leaves short fragments everywhere; this is comparing two readings
// that both write prose as prose, so the floor only has to clear the articles
// and the prepositions.
const MinWord = 4

// words is a page judged on which words one reading has and the other has not,
// in both directions.
//
// It is crosscheck's question and not crosscheck's answer, and the difference
// is the second reader. That package asks pdftotext, which reads the same text
// layer the extractor does and cannot see a glyph the layer misnames, and it is
// tuned for that: markup is stripped on our side only, because pdftotext emits
// none, and a capitalised word is thrown away, because there it is either a
// proper name both readings spell the same or a variable set in mathematics.
//
// Neither holds here. Both readings are Markdown with LaTeX in them, so the
// macro names have to come out of both or every \mathring in the model's
// reading is reported as a word we lost. And a capitalised word is where these
// defects live: Šmulian, Hölder, and every French name whose accent the text
// layer sets down beside the letter instead of over it. Throwing those away
// would throw away the finding.
//
// The two directions do different work. What the model has and we do not is the
// defect: a loose accent leaves us with Smulian where the page says Šmulian.
// What we have and the model does not is the other kind of finding, usually the
// model skipping a footnote rather than us inventing anything, and it is worth
// seeing before any of this is believed.
//
// head is the running head and the folio, and every word of it is dropped from
// both sides. The extractor puts them in the front matter and the model puts
// them in the body, so the first run of this reported ENDOMORPHISMES, ESPACES
// and BANACH against six of seven pages and reported nothing else about four of
// them. That is the audit finding a difference in where two readings file the
// furniture, which is a difference nobody is going to fix.
func words(ours, theirs, head string) (lost, extra []string) {
	mine, model, furniture := bag(ours), bag(theirs), bag(head)
	return missing(model, mine, furniture), missing(mine, model, furniture)
}

// missing is the words of the first bag the second has not, sorted, each
// reported once, and none of them the page's own furniture. Sorted rather than
// in reading order because a bag has no reading order and a report that changed
// the order of its findings between two runs of the same audit would be
// unreadable as a diff.
//
// What is reported is the word as it was written rather than the key it was
// filed under, because the key has had its case and its hyphens taken off it
// and the whole of a finding can be in those.
func missing(from, in, skip map[string]string) []string {
	var out []string
	for key, word := range from {
		if _, ok := in[key]; ok {
			continue
		}
		if _, ok := skip[key]; ok {
			continue
		}
		out = append(out, word)
	}
	sort.Strings(out)
	return out
}

// markup is what neither reading is making a claim about at the scale of a
// word: a control word, the dollars and braces of mathematics, and Markdown's
// own emphasis and headings.
var markup = regexp.MustCompile(`\\[a-zA-Z]+|[\\${}^_&#*` + "`" + `~\[\]()|]`)

// bag is the words of a reading, each filed under the form it is compared by
// and kept in the form it was written.
//
// The accents are kept, in NFC, and that is the whole point of the route: é and
// e are two different words here, and Šmulian against Smulian is the finding.
//
// The case and the hyphens are folded, and both because of what the first page
// run reported. Bourbaki sets Théorème and Corollaire in small capitals at the
// head of a statement; the extractor writes the word and the model writes
// THÉORÈME, and neither of them has read anything wrong. A word broken across a
// line break keeps its hyphen in the model's reading and loses it in ours, so
// the page that says ensemble came back as en-semble. Those two accounted for
// every finding on four of the seven pages of the first run.
func bag(text string) map[string]string {
	out := map[string]string{}
	stripped := markup.ReplaceAllString(norm.NFC.String(Unfence(text)), " ")
	for field := range strings.FieldsFuncSeq(stripped, func(r rune) bool {
		return !unicode.IsLetter(r) && !dash(r)
	}) {
		word := strings.TrimFunc(field, dash)
		if len([]rune(word)) < MinWord {
			continue
		}
		key := strings.ToLower(strings.Map(func(r rune) rune {
			if dash(r) {
				return -1
			}
			return r
		}, word))
		if _, ok := out[key]; !ok {
			out[key] = word
		}
	}
	return out
}

// dash is every mark the two readings join two halves of a word with.
//
// All of them, not the ASCII one, and the page that says so is the theorem of
// Krein and Rutman. The extractor writes the pair with an en dash, which is
// what the compositor set and what a name pair takes; the model types a hyphen.
// A comparison that knew only the hyphen split ours into two words, kept the
// model's as one, and reported Krein-Rutman as something the model read and we
// had not, on two pages, when both readings are right. The soft hyphen is here
// as well because it is what a text layer leaves where a word was broken.
//
// The em dash is deliberately not here. Bourbaki opens the proof of a statement
// with one and it is punctuation, not a joiner.
func dash(r rune) bool {
	switch r {
	case '-', '­', '‐', '‑', '–':
		return true
	}
	return false
}

// answerName is what the tool calls the Markdown for a clip. It mirrors the
// input tree and swaps the extension, which is the same rule the page batches
// rely on.
func answerName(image string) string {
	return strings.TrimSuffix(image, filepath.Ext(image)) + ".md"
}

// ReadAnswer is one clip's answer as the model wrote it.
//
// The header ocr-batch puts above every answer comes off first. It is four
// lines of machinery naming the source path on the rented box, the model slot
// and the seconds it took, and it is fenced in the same three dashes a front
// matter is, so a comparison that left it standing would report every clip in
// the run as a disagreement in its first character. The page pipeline strips
// the same block for the same reason and this borrows its rule rather than
// writing a second one that could drift from it.
func ReadAnswer(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(ocr.StripToolHeader(string(raw)))
	if text == "" {
		return "", fmt.Errorf("%s is empty", filepath.Base(path))
	}
	// The service answers its own failures in the file it answers questions in.
	// One page of a seven page run came back as a sentence apologising, from a
	// profile that had just been rotated onto, and an audit that took it for a
	// reading reported the page as a disagreement about every word on it. It is
	// a clip that has not been read, which is what silence means here.
	if why := ocr.ProviderFailure(text); why != "" {
		return "", fmt.Errorf("%s came back as %s", filepath.Base(path), why)
	}
	return Unfence(text), nil
}

// fences are the code block a model wraps an answer in when it decides the
// answer is code. The prompt asks for the line and nothing else, and most
// answers are the line and nothing else, but a line that is mostly LaTeX comes
// back fenced often enough that treating the fence as content would report a
// disagreement on every one of them.
var fences = regexp.MustCompile("(?s)^```[a-zA-Z]*\n(.*?)\n?```$")

// Unfence takes the code fence off an answer, if it has one.
func Unfence(text string) string {
	text = strings.TrimSpace(text)
	if match := fences.FindStringSubmatch(text); match != nil {
		return strings.TrimSpace(match[1])
	}
	return text
}

// alias is the macros that are two spellings of one thing.
//
// Every entry is something the two readings differ on for a reason that is not
// a defect. The extractor writes what the font drew, so it writes \rightarrow
// for an arrow and \dots for an ellipsis; a model writes what an author would
// type. Judging those as disagreements buries the ones that matter.
//
// What is deliberately not here is anything that changes what is on the page.
// \mathbf and \mathbb are not aliases, because Bourbaki sets its rings in bold
// and a model that writes blackboard bold has read the page wrong. \hat and
// \widehat are not aliases either, because which one it is is exactly what the
// wide accents are an argument about.
var alias = map[string]string{
	`\to`:             `\rightarrow`,
	`\longrightarrow`: `\rightarrow`,
	`\longmapsto`:     `\mapsto`,
	`\ldots`:          `\dots`,
	`\cdots`:          `\dots`,
	`\varnothing`:     `\emptyset`,
	`\ne`:             `\neq`,
	`\le`:             `\leq`,
	`\ge`:             `\geq`,
	`\colon`:          `:`,
	`\prime`:          `'`,
	`\ast`:            `*`,
}

// gone is the macros that say how a thing is set rather than what it is: the
// sizing of a delimiter, the spacing, the roman of an operator's name. A
// difference in any of those is a difference of typography between two people
// writing the same formula.
//
// \mathrm and its relatives are here and \mathbf is not, and the line between
// them is whether Bourbaki uses it to mean something. It sets its rings in bold
// and its Lie algebras in fraktur, and it sets an operator's name in roman
// because that is how every book sets an operator's name.
var gone = map[string]bool{
	`\left`: true, `\right`: true, `\middle`: true,
	`\bigl`: true, `\bigr`: true, `\bigm`: true, `\big`: true,
	`\Bigl`: true, `\Bigr`: true, `\Bigm`: true, `\Big`: true,
	`\biggl`: true, `\biggr`: true, `\bigg`: true,
	`\Biggl`: true, `\Biggr`: true, `\Bigg`: true,
	`\quad`: true, `\qquad`: true, `\thinspace`: true,
	`\mathrm`: true, `\text`: true, `\textrm`: true, `\textit`: true,
	`\mathit`: true, `\operatorname`: true, `\rm`: true, `\it`: true,
	`\mathop`: true, `\mathord`: true, `\mathrel`: true, `\mathbin`: true,
	`\limits`: true, `\nolimits`: true, `\displaystyle`: true, `\textstyle`: true,
}

// macro matches a control word, which is a backslash and the letters after it.
// Matching the whole name is the point: a table that replaced \le by \leq as a
// substring would turn \leq into \leqq and \top into an arrow with a p on it.
var macro = regexp.MustCompile(`\\[a-zA-Z]+`)

// noise is what neither reading is making a claim about: the dollars that say
// where mathematics starts, the braces that group it, the control symbols that
// space it, and the Markdown emphasis.
//
// Spaces go entirely rather than being collapsed. In mathematics they mean
// nothing at all, and in prose a difference of one is a difference about
// whether a model puts a space before a comma, which is not what any of this is
// for.
var noise = strings.NewReplacer(
	`\,`, ``, `\;`, ``, `\:`, ``, `\!`, ``, `\ `, ``,
	`$$`, ``, `$`, ``, `\(`, ``, `\)`, ``, `\[`, ``, `\]`, ``,
	// A brace goes whether it groups or is a brace of a set. The escaped pair
	// is listed first so that it goes whole and does not leave its backslash
	// standing where the brace was.
	`\{`, ``, `\}`, ``, `{`, ``, `}`, ``, `~`, ``,
	"**", ``, `*`, ``,
	" ", ``, "\t", ``, "\n", ``,
	" ", ``, " ", ``, " ", ``,
)

// Normalize reduces a reading to what it claims about the page.
//
// Two readings of one line of Bourbaki can be the same reading and not the same
// string, and almost all of the ways they can are mechanical: one writes $x$ and
// the other x, one wraps a subscript in braces and the other does not, one sizes
// a bracket. Folding those away is what makes the remaining differences worth a
// person's time, and it is the only thing standing between a useful audit and a
// list of six hundred lines that all say the same thing twice.
//
// The order is not free. The macros are folded first, while their names are
// still whole, and only then are the braces and the dollars taken out, because
// a name ends where the brace begins and a pass that has already removed the
// brace cannot tell \mathrm{a}b from \mathrmab.
func Normalize(text string) string {
	text = norm.NFC.String(Unfence(text))
	text = macro.ReplaceAllStringFunc(text, func(name string) string {
		if gone[name] {
			return ""
		}
		if to, ok := alias[name]; ok {
			return to
		}
		return name
	})
	return noise.Replace(text)
}

// Diff is where two readings first part company once normalised, or -1 when
// they do not.
func Diff(ours, theirs string) int {
	a, b := []rune(Normalize(ours)), []rune(Normalize(theirs))
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) == len(b) {
		return -1
	}
	return min(len(a), len(b))
}

// Window is the text around the point two readings parted, for printing beside
// each other. Twenty characters each way is enough to see a word.
func Window(text string, at int) string {
	if at < 0 {
		return ""
	}
	runes := []rune(Normalize(text))
	from, to := max(0, at-20), min(len(runes), at+20)
	out := string(runes[from:to])
	if from > 0 {
		out = "..." + out
	}
	if to < len(runes) {
		out += "..."
	}
	return out
}

// Summary is the few lines a run prints when it has judged everything.
func (r Report) Summary() string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s: %d clips at %d dpi, %d agree, %d differ, %d silent\n",
		r.Book, r.Clips, r.DPI, r.Agreed, r.Differed, r.SilentN)
	if judged := r.Agreed + r.Differed; judged > 0 {
		fmt.Fprintf(&out, "%.0f%% of the clips that came back agree with the extractor\n",
			100*float64(r.Agreed)/float64(judged))
	}
	return out.String()
}

// Markdown is the report a person reads, which is the disagreements and nothing
// else. A clip that agreed has nothing to say beyond the count above it.
func (r Report) Markdown() string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Clip audit: %s\n\n", r.Book)
	if r.Match != "" {
		fmt.Fprintf(&out, "Lines matching `%s`, cut at %d dpi and read as pictures.\n\n", r.Match, r.DPI)
	}
	fmt.Fprintf(&out, "%d clips, %d agree, %d differ, %d silent.\n\n", r.Clips, r.Agreed, r.Differed, r.SilentN)
	if r.Differed == 0 {
		fmt.Fprint(&out, "Nothing to look at: every clip that came back reads the same as the extractor.\n")
		return out.String()
	}
	fmt.Fprint(&out, "## Disagreements\n\n")
	for _, row := range r.Rows {
		if row.Verdict != Differ {
			continue
		}
		if row.Line < 0 {
			fmt.Fprintf(&out, "### page %d\n\n", row.Page)
			if len(row.Lost) > 0 {
				fmt.Fprintf(&out, "- the model read and we have not: %s\n", code(row.Lost))
			}
			if len(row.Extra) > 0 {
				fmt.Fprintf(&out, "- we have and the model did not: %s\n", code(row.Extra))
			}
			fmt.Fprintln(&out)
			continue
		}
		fmt.Fprintf(&out, "### page %d line %d\n\n", row.Page, row.Line)
		fmt.Fprintf(&out, "- ours: `%s`\n", row.Native)
		fmt.Fprintf(&out, "- model: `%s`\n", row.Model)
		fmt.Fprintf(&out, "- parts at: `%s` against `%s`\n\n",
			Window(row.Native, row.At), Window(row.Model, row.At))
	}
	// Every entry ends with the blank line that separates it from the next
	// one, including the last, which has nothing to be separated from. The
	// corpus audit reads a file that ends with a blank line as a finding, so
	// this is the difference between a report and a report the corpus will
	// not take.
	return strings.TrimRight(out.String(), "\n") + "\n"
}

// code is a list of words set as code and separated by commas, cut off at
// twenty. A page where the model and the extractor share almost nothing is a
// page to open rather than a page to read a list of three hundred words about.
func code(list []string) string {
	var out []string
	for _, word := range list {
		if len(out) == 20 {
			return strings.Join(out, ", ") + fmt.Sprintf(" and %d more", len(list)-20)
		}
		out = append(out, "`"+word+"`")
	}
	return strings.Join(out, ", ")
}
