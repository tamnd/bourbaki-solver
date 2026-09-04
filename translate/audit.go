// Package translate turns an English section body into another language and
// refuses most of what comes back.
//
// The refusing is the package. Translating Bourbaki is not translating prose:
// the prose is the connective tissue and the mathematics is the book, and a
// model asked to translate a section will quietly renumber a proposition, drop
// a display, tidy a bracket inside a formula, or stop two paragraphs early with
// no sign in the file that it did. None of those is visible to a reader who
// does not have the English open beside it, and all of them are visible to a
// program that does.
//
// So Audit compares the answer with its source and reports every way they
// differ that they were told not to. Invariants 1 to 6 of the spec are all
// here, all deterministic, all run on every section: the mathematics byte for
// byte and in order, the attribute blocks and their tags, the heading tree and
// the statement numbering, the block count, the cross reference skeleton, no
// front matter, no commentary, and the answer actually written in the language
// that was asked for. Invariant 7, the round trip, is a model call and is not
// here.
//
// What this cannot do is tell a correct translation from a fluent wrong one.
// Every rule below is about the shape of the answer, and a sentence that says
// the opposite of the English in perfect Vietnamese, with the formulae intact,
// passes all of them. That is what the round trip sample and a reader are for,
// and it is why nothing here is called a check that the translation is right.
package translate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// A Problem is one way the answer differs from its source.
//
// Rule names the invariant rather than numbering it, because the number is the
// spec's and the name is what a person reading a failure wants to see first.
type Problem struct {
	Rule string
	Msg  string
	// Line is one based in the answer, and 0 when the problem is the answer as
	// a whole rather than a place in it.
	Line int
}

func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("line %d: %s: %s", p.Line, p.Rule, p.Msg)
	}
	return p.Rule + ": " + p.Msg
}

// Rule names. Every problem carries one of these.
const (
	RuleMath         = "math"
	RuleMathProse    = "math prose"
	RuleTag          = "tag"
	RuleStructure    = "structure"
	RuleReference    = "reference"
	RuleFrontMatter  = "front matter"
	RuleCommentary   = "commentary"
	RuleLanguage     = "language"
	RuleBibliography = "bibliography"
	RuleScript       = "script"
	RuleRefusal      = "refusal"
)

// Audit compares a translated body with the English it was made from.
//
// An empty result means the answer may be written to the corpus. It does not
// mean the answer is a good translation, only that it is the same section.
//
// Every rule runs, and everything each of them finds is reported. Stopping at
// the first would hide the difference between an answer that dropped one
// formula and an answer that paraphrased the section, and those two want
// different things done about them: the first is worth one retry, the second is
// worth changing the prompt.
func Audit(lang, en, tr string) []Problem {
	var out []Problem
	if strings.TrimSpace(tr) == "" {
		return []Problem{{Rule: RuleCommentary, Msg: "the answer is empty"}}
	}
	out = append(out, auditFrontMatter(tr)...)
	out = append(out, auditRefusal(tr)...)
	out = append(out, auditCommentary(tr)...)
	out = append(out, auditMath(en, tr)...)
	out = append(out, auditAttrs(en, tr)...)
	out = append(out, auditHeadings(en, tr)...)
	out = append(out, auditBlocks(en, tr)...)
	out = append(out, auditRefs(en, tr)...)
	out = append(out, AuditBiblio(en, tr)...)
	out = append(out, auditLanguage(lang, en, tr)...)
	out = append(out, AuditScript(lang, en, tr)...)
	return out
}

// Invariant 5. The model returns a body, and the tool writes the head.
//
// A model handed a section with front matter above it tends to hand one back,
// and one it wrote itself, with the statement count guessed and the hash made
// up. That head would then be parsed as the real one. Refusing the fence is
// cheaper than trying to tell a real head from an invented one.
func auditFrontMatter(tr string) []Problem {
	if !strings.HasPrefix(strings.TrimSpace(tr), "---") {
		return nil
	}
	return []Problem{{Rule: RuleFrontMatter, Line: 1,
		Msg: "opens with a front matter fence, and the front matter is written by the tool"}}
}

// providerKinds are the textguard leaks that are the provider talking rather
// than the model answering: a rate limit or a block from the gateway, and a
// model declining the work. The transport returns both as a successful answer
// because from its side they are one, a page came back with a body in it.
//
// These were already found, and by this same check. They were reported under
// RuleCommentary along with narration, and -raw waives commentary, so the
// message was written to the corpus under a full set of headers with a matching
// source hash and looked finished to every pass after it. Eight sections of the
// Vietnamese corpus were sitting in that state, each one holding "Unusual
// activity has been detected from your device. Try again later." and a request
// id. So the detection was never the gap; the waiver was. Giving these two
// kinds a rule of their own is what lets takeRaw refuse to waive them without
// also refusing to waive a model that narrated its translation.
var providerKinds = map[string]bool{"gateway": true, "refusal": true}

// Invariant 6a. A message from the provider is not a translation.
//
// Never waived by -raw. -raw exists to keep an answer the audit disliked, on the
// grounds that a flawed translation is still a translation and is worth having
// while the corpus is being filled. A refusal is not a translation at all and
// there is nothing in it to fix later, which puts it with the transport failures
// rather than with the audit.
func auditRefusal(tr string) []Problem {
	var out []Problem
	for _, leak := range textguard.Check(tr) {
		if providerKinds[leak.Kind] {
			out = append(out, Problem{Rule: RuleRefusal, Line: leak.Line,
				Msg: fmt.Sprintf("the answer is a message from the provider, not a translation, %s: %q", leak.Kind, leak.Detail)})
		}
	}
	return out
}

// narration is a model saying what it has just done, in the words it says it in
// when the thing it has just done is a translation.
//
// textguard catches the transcription version of this, because that is what it
// was built for and the phrases it holds are the ones a page of OCR came back
// with. A translation is narrated differently and none of its phrases are
// there. Adding them to textguard would put translation phrases in front of
// every OCR page for nothing, so they live here.
//
// English, all of them, and deliberately so. A finished Vietnamese or Japanese
// body has almost no English prose in it, so a first person English sentence
// about a translation is not something the section could contain. The list is
// kept to phrases with both the first person or "here is" and the word
// translation in them, because "the translation" on its own is a thing an
// algebra text says about a map.
var narration = []string{
	"here is the translation",
	"here's the translation",
	"here is the translated",
	"here's the translated",
	"below is the translation",
	"i have translated",
	"i've translated",
	"the translation of the section",
	"translation of the above",
	"as requested, here",
}

// Invariant 6. No commentary.
func auditCommentary(tr string) []Problem {
	var out []Problem
	for _, leak := range textguard.Check(tr) {
		// The provider kinds are reported by auditRefusal instead, under a rule
		// -raw does not waive. Reporting them here as well would say one bad
		// answer twice and would put the count of problems out.
		if providerKinds[leak.Kind] {
			continue
		}
		out = append(out, Problem{Rule: RuleCommentary, Line: leak.Line,
			Msg: fmt.Sprintf("%s: %q", leak.Kind, leak.Detail)})
	}
	for i, line := range strings.Split(tr, "\n") {
		lower := strings.ToLower(line)
		for _, phrase := range narration {
			if strings.Contains(lower, phrase) {
				out = append(out, Problem{Rule: RuleCommentary, Line: i + 1,
					Msg: fmt.Sprintf("narration: %q", phrase)})
				break
			}
		}
	}
	return out
}

// Invariant 1. The mathematics is byte-identical and in order, apart from the
// words a printing sets inside it, which are prose and are translated.
//
// Compared span by span in sequence and not as a set, because a model that
// reflows a paragraph moves a formula without losing it, and a set comparison
// says nothing happened. mathtex.Split is the same splitter the extraction and
// the audit use, so a span here is a span everywhere.
//
// The exception is glossary.SameMath's and the reasoning for it is written out
// there. Everything outside a \text is still compared character for character,
// and a name the printing sets upright inside one, Card or resp., is compared
// the same way; what may move is a run holding an English word.
func auditMath(en, tr string) []Problem {
	var out []Problem
	got, unclosed := mathtex.Split(tr)
	if unclosed != nil {
		out = append(out, Problem{Rule: RuleMath, Line: unclosed.Line,
			Msg: "a math span is opened and never closed"})
	}
	want, _ := mathtex.Split(en)
	named := false
	if len(got) != len(want) {
		msg := fmt.Sprintf("has %d math spans and the English has %d", len(got), len(want))
		lost, added := alignSpans(want, got)
		if len(lost) > 0 {
			msg += ", and these are not in it: " + spanList(lost)
		}
		if len(added) > 0 {
			msg += ", and these are in it and not in the English: " + spanList(added)
		}
		named = len(lost) > 0 || len(added) > 0
		out = append(out, Problem{Rule: RuleMath, Msg: msg})
	}
	// The span-by-span comparison is only worth making while the two lists are
	// still in step. Once one is short the alignment above has already said
	// which spans went and which arrived, and every position after the first
	// drop reports as changed when nothing at that position changed at all.
	for i := 0; !named && i < len(got) && i < len(want); i++ {
		if glossary.SameMath(want[i].Text, got[i].Text) && got[i].Display == want[i].Display {
			continue
		}
		mine, theirs := ShortDiff(got[i].Text, want[i].Text)
		out = append(out, Problem{Rule: RuleMath, Line: got[i].Line,
			Msg: fmt.Sprintf("math span %d is %s and the English has %s", i+1, mine, theirs)})
		// One is enough. After the first difference the spans are out of step
		// and every one after it reports as changed, which buries the one that
		// actually moved under a hundred that did not.
		break
	}
	return append(out, auditMathProse(en, tr)...)
}

// spanAlignLimit is the most span pairs the alignment will compare. The whole
// point of it is a chunk of a few dozen spans, and a quadratic run over a
// pathological one is not worth the risk of holding a lane up.
const spanAlignLimit = 40000

// alignSpans says which of the English spans the answer does not have and which
// of the answer's spans the English does not, by the longest common
// subsequence of the two lists.
//
// A count is not something an answer can be corrected from, and that is what
// the rule gave the model until now. Two exercises of the last Vietnamese run
// stood on it for eleven attempts across two hosts and three sittings: one is
// 54 inline spans in 2685 characters, a span every fifty characters and most of
// them one letter, and the answers came back holding 52 of them, then 53, then
// 52 nine times over. "has 52 math spans and the English has 54" tells the
// model that it lost two of fifty-four and nothing at all about which two, so
// the re-ask askChunk makes is another draw from the same urn. Naming them
// makes it a correction.
//
// A subsequence and not a set, for the reason auditMath compares in order: a
// model that reflows a paragraph moves a formula without losing it, and a set
// comparison of a reflow says nothing happened. A span the model altered rather
// than dropped falls out of this as one lost and one added, which is the same
// finding the positional comparison would have made and says more, since it
// carries no claim about where the span sits.
func alignSpans(want, got []mathtex.Span) (lost, added []string) {
	if len(want)*len(got) > spanAlignLimit {
		return nil, nil
	}
	same := func(i, j int) bool {
		return want[i].Display == got[j].Display && glossary.SameMath(want[i].Text, got[j].Text)
	}
	n, m := len(want), len(got)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case same(i, j):
				dp[i][j] = dp[i+1][j+1] + 1
			case dp[i+1][j] >= dp[i][j+1]:
				dp[i][j] = dp[i+1][j]
			default:
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case same(i, j):
			i, j = i+1, j+1
		case dp[i+1][j] >= dp[i][j+1]:
			lost = append(lost, want[i].Text)
			i++
		default:
			added = append(added, got[j].Text)
			j++
		}
	}
	for ; i < n; i++ {
		lost = append(lost, want[i].Text)
	}
	for ; j < m; j++ {
		added = append(added, got[j].Text)
	}
	return lost, added
}

// spanList quotes the spans, at most six of them. A list of forty is a count
// written the long way and is no more use to the model than the count was.
func spanList(spans []string) string {
	const most = 6
	parts := make([]string, 0, most)
	for _, s := range spans[:min(len(spans), most)] {
		parts = append(parts, window([]rune(strings.Join(strings.Fields(s), " ")), 0))
	}
	out := strings.Join(parts, ", ")
	if len(spans) > most {
		out += fmt.Sprintf(", and %d more", len(spans)-most)
	}
	return out
}

// Invariant 1 read the other way round. A word set inside the mathematics is
// prose, so a span that comes back with the English words still in it is an
// untranslated piece of the section rather than a formula copied correctly.
//
// It is reported apart from the mathematics because the two failures want
// opposite things done about them. A span that differs is a model that touched
// what it was told to copy; a span whose words stand is a model that did what
// the prompt used to say. Every run is named rather than the first, since a
// display of chapter I holds four of them and they are all the same mistake
// made once.
func auditMathProse(en, tr string) []Problem {
	got, _ := mathtex.Split(tr)
	want, _ := mathtex.Split(en)
	var out []Problem
	for i := 0; i < len(got) && i < len(want); i++ {
		for _, run := range glossary.UntranslatedMathProse(want[i].Text, got[i].Text) {
			out = append(out, Problem{Rule: RuleMathProse, Line: got[i].Line,
				Msg: fmt.Sprintf("math span %d holds %s, which is prose and is not translated",
					i+1, run)})
		}
	}
	return out
}

// attrRE is a heading's attribute block: the identifier, the classes and the
// tag, in the braces at the end of the line.
var attrRE = regexp.MustCompile(`\{#[^}\n]*\}`)

// Invariant 2. The attribute blocks come through untouched.
//
// Whole blocks and not just the tags, because the identifier is what a link
// points at and the class is what the assembler reads. Compared in order, for
// the same reason the mathematics is.
func auditAttrs(en, tr string) []Problem {
	got := attrRE.FindAllString(tr, -1)
	want := attrRE.FindAllString(en, -1)
	if strings.Join(got, "\n") == strings.Join(want, "\n") {
		return nil
	}
	var out []Problem
	if missing := notIn(want, got); len(missing) > 0 {
		out = append(out, Problem{Rule: RuleTag,
			Msg: fmt.Sprintf("the English has %d attribute blocks this answer does not: %s",
				len(missing), strings.Join(missing, " "))})
	}
	if extra := notIn(got, want); len(extra) > 0 {
		out = append(out, Problem{Rule: RuleTag,
			Msg: fmt.Sprintf("carries %d attribute blocks the English does not: %s",
				len(extra), strings.Join(extra, " "))})
	}
	if len(out) == 0 {
		out = append(out, Problem{Rule: RuleTag,
			Msg: "carries the English attribute blocks in a different order"})
	}
	return out
}

// headingRE reads a heading: the hashes, the text, and the attribute block if
// there is one.
var headingRE = regexp.MustCompile(`^(#{1,6})\s+(.*?)(\s*\{#[^}]*\})?$`)

// trailingNumberRE is the number a statement heading ends with, as in
// "Proposition 3" or "Định lý 1". Bourbaki numbers statements per §, so this is
// a small integer and there is at most one of them.
var trailingNumberRE = regexp.MustCompile(`(\d+)\s*$`)

type heading struct {
	level int
	num   string
	line  int
	text  string
}

// Invariant 3. The heading tree and the statement numbering are the English
// ones.
//
// The heading text is in the language being checked, so what is compared is
// what is not: how deep the heading is, in what order, and what number it ends
// with. A Proposition 3 that came back as Mệnh đề 4 is caught here, and it is
// the failure that a reader is least likely to notice and most likely to be
// hurt by, because everything that cites it still says 3.
//
// The kind of a statement is not compared here even though the label carries
// it, because auditAttrs has already required the label to be the English one
// character for character, and a label that is unchanged has an unchanged kind.
func auditHeadings(en, tr string) []Problem {
	got, want := headings(tr), headings(en)
	var out []Problem
	if len(got) != len(want) {
		out = append(out, Problem{Rule: RuleStructure,
			Msg: fmt.Sprintf("has %d headings and the English has %d", len(got), len(want))})
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		switch {
		case got[i].level != want[i].level:
			out = append(out, Problem{Rule: RuleStructure, Line: got[i].line,
				Msg: fmt.Sprintf("heading %d is at level %d and the English has it at level %d",
					i+1, got[i].level, want[i].level)})
		case got[i].num != want[i].num:
			out = append(out, Problem{Rule: RuleStructure, Line: got[i].line,
				Msg: fmt.Sprintf("heading %d is numbered %s and the English numbers it %s",
					i+1, quoteOrNone(got[i].num), quoteOrNone(want[i].num))})
		}
	}
	return out
}

func headings(body string) []heading {
	var out []heading
	for i, line := range strings.Split(body, "\n") {
		m := headingRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		h := heading{level: len(m[1]), line: i + 1, text: strings.TrimSpace(m[2])}
		if n := trailingNumberRE.FindStringSubmatch(h.text); n != nil {
			h.num = n[1]
		}
		out = append(out, h)
	}
	return out
}

// bibEntryRE opens a bibliography entry: the number the note cites the work by,
// as "12." or "2 (*bis*).". It is necessary and it is nowhere near sufficient.
// The comment that stood here said the shape appears 57 times over the English
// corpus and that every one of them is under a BIBLIOGRAPHY heading. Measured
// again over content/en and content/en-mt it appears 459 times: 21 under a
// BIBLIOGRAPHY heading, 170 in the notes to the reader, and 268 in exercises and
// historical notes. The premise was wrong by a factor of twenty and the numbered
// paragraph is the commonest shape in the book, since a note to the reader is
// twelve of them and an exercise names its parts the same way.
//
// What it cost: WithoutBiblio took all 459 out of the question and WithBiblio
// put them back from the English, so 438 blocks of running prose were never
// asked for and came back verbatim English in every language. That is every
// numbered paragraph of all thirteen notes to the reader, which is the first
// page a reader of a Vietnamese volume opens.
var bibEntryRE = regexp.MustCompile(`^\d+\s*(\(\*?bis\*?\)\s*)?\.\s`)

// refTailRE is the end of a printed reference: the volume, the year in
// parentheses and the pages. It reads a citation by its tail because the
// opener is what bibEntryRE cannot be trusted on, and the tail is the part no
// running sentence has.
//
// Exercise 15 of § 2 of Topological Vector Spaces I carries a footnote, and the
// footnote is a citation: "1 For the exercises 12 and 13, see O. Goldman and N.
// Iwahori, The space of p-adic norms, Acta math., 109 (1963), pp. 137-177." The
// marker is a bare 1 with no period after it, so bibEntryRE does not see a
// bibliography entry, the line stays in the prose, and the only occurrence of
// the word space in that file is inside the title of the cited paper. The
// terminology rule then asked for "không gian" in a paper's name, the chunk was
// refused on all three attempts and the file was one of fifteen the run could
// not land.
//
// Over content/en this shape is on 268 lines of 259,261. bibEntryRE already
// covers 88 of them and the other 180 are the bracket-numbered references of
// the historical notes, "[4] E. Heine, Über trigonometrische Reihen, Crelle's
// Journal, 71 (1870), pp. 353-365.", which have the same fault for the same
// reason: the German and French titles of the works cited stand as printed, and
// every English word in one of them was being read as English left in the
// answer.
var refTailRE = regexp.MustCompile(`\(\d{4}\),?\s*pp?\.\s*\d`)

// bibAuthorRE and bibTitleRE are what tells an entry from a numbered paragraph.
//
// A heading is no use here. Of the 149 real entries in the corpus only 20 stand
// under a BIBLIOGRAPHY heading; 38 follow a historical note with no heading of
// their own, and 27 are the Klein and Lie citations inside exercise 3 of Lie III
// § 10, under nothing at all. So the block has to answer for itself.
//
// An entry opens on the work's author or on its title and neither is a sentence.
// The author is initials and a surname, "O. Neugebauer" or "F. KLEIN", or a name
// carrying no initial before an italic title, "Galileo Galilei, *Opere*," and
// "ARISTOTLE, *Organon*,", which is why the second alternative ends on the comma
// and the italic rather than on a word count. The title comes
// first when the work has no one author, "*The Works of Aristotle*, translated
// under the editorship of", and it is italic and followed by a comma, which is
// what keeps an italic exercise statement and a bold lead-in like "*Completion &
// local rings*." out. A numbered paragraph opens on a verb or an article
// instead: "The Elements of Mathematics series takes up", "Let $X$ be a set",
// "Show that the simple group", "Le traité prend les mathématiques".
//
// Measured over content/en, content/en-mt, content/fr, content/vi and
// content/solutions: 149 entries in 13 files, every one of them a historical
// note or that one exercise, and not one block missed of those standing under a
// BIBLIOGRAPHY or THU MUC heading. The French tree comes to zero, which is right,
// since no French bibliography has been read yet.
var (
	bibAuthorRE = regexp.MustCompile(`^\*{0,2}(\p{Lu}\.|\p{Lu}[\p{L}'’-]*( \p{Lu}[\p{L}'’-]*)*, \*)`)
	bibTitleRE  = regexp.MustCompile(`^\*[^*$]{2,140}\*,`)
)

// AuditBiblio holds a translation to Invariant 1 for the apparatus: a
// bibliography entry stands as printed.
//
// The bibliography is apparatus and not prose. "Vorlesungen über die Geschichte
// der antiken Mathematik" is the name of a book, and a reader who wants to find
// it needs the name the library has it under; a Vietnamese rendering of the
// title is a work nobody can look up. The same goes for the journal, the
// volume, the year and the pages, which is all the rest of the entry.
//
// This also settles a loop. Chunk 30 of the historical note of chapters I to IV
// is 49 bibliography entries, and their titles hold the English words
// hypothesis, topology, algebra, choice, formal, method and remark. The
// terminology rule read those as seven terms left untranslated and refused the
// chunk over and over, correctly by its own lights and wrongly, because the
// words are inside the names of books. prose leaves the entries out for the
// same reason it leaves out a display.
func AuditBiblio(en, tr string) []Problem {
	want, got := bibEntries(en), bibEntries(tr)
	if len(got) != len(want) {
		return []Problem{{Rule: RuleBibliography, Msg: fmt.Sprintf(
			"has %d bibliography entries and the English has %d", len(got), len(want))}}
	}
	for i := range want {
		if got[i] == want[i] {
			continue
		}
		mine, theirs := ShortDiff(got[i], want[i])
		return []Problem{{Rule: RuleBibliography, Msg: fmt.Sprintf(
			"bibliography entry %d is %s and the English has %s, and an entry stands as printed",
			i+1, mine, theirs)}}
	}
	return nil
}

func bibEntries(body string) []string {
	var out []string
	for _, b := range blocks(body) {
		// BiblioEntry and not the opening shape alone. AuditBiblio requires that
		// what this collects stands as printed, so a numbered paragraph gathered
		// here would be a rule against ever translating it, said by the one rule
		// nothing else can overrule.
		if BiblioEntry(b) {
			out = append(out, strings.Join(strings.Fields(b), " "))
		}
	}
	return out
}

// withoutBiblio is the passage with the numbered entries taken out, which is
// what a rule about the language of the writing has to look at. The entries are
// not writing, they are addresses, and they are in whatever language the book
// they name was printed in.
func withoutBiblio(body string) string {
	var out []string
	for _, b := range blocks(body) {
		if !BiblioEntry(b) {
			out = append(out, b)
		}
	}
	return joinBlocks(out)
}

// WithoutBiblio is a passage as a question: the numbered entries taken out, so
// that what is asked for is what is actually going to be translated.
//
// Chunk 30 of the historical note of chapters I to IV is why. It is 5,940
// characters, of which 4,900 are twenty entries from Dedekind to Cohen that
// stand as printed, and the rest is three footnotes. Asked whole, the chunk
// makes a model reproduce nearly five thousand characters of German and French
// titles, page ranges and volume numbers letter for letter, and no route
// managed it: the best answer came back with eleven entries of the twenty and
// twenty two citations gone. Asked as three footnotes it is a small question.
//
// WithBiblio is the other half and the two are only correct together.
//
// A passage with no entries in it comes back exactly as it stands, rather than
// through the block join, which would put the layout of every question through
// a normalisation for no reason. Three chunks of the 5,228 the English corpus
// comes to hold a bibliography, and they are the historical notes.
func WithoutBiblio(body string) string {
	for _, b := range blocks(body) {
		if BiblioEntry(b) {
			return withoutBiblio(body)
		}
	}
	return body
}

// WithBiblio puts the entries back where they stood, around an answer that was
// asked for everything else.
//
// The blocks of the answer go into the places the blocks of the English left
// empty, in order, and the entries are copied from the English. It fails if the
// answer does not have one block for every block it was asked for, which is
// auditBlocks over again, said here because the splice cannot be done at all
// without it and a wrong splice would silently move a footnote under the wrong
// citation.
func WithBiblio(en, answer string) (string, bool) {
	got, i := blocks(answer), 0
	var out []string
	for _, b := range blocks(en) {
		// The blank line between two blocks belongs to the join and not to
		// either of them. A block that keeps one of its own comes out of a place
		// where the English left two blank lines, and putting it back through
		// the join would leave three.
		if BiblioEntry(b) {
			out = append(out, strings.Trim(b, "\n"))
			continue
		}
		if i >= len(got) {
			return "", false
		}
		out = append(out, strings.Trim(got[i], "\n"))
		i++
	}
	if i != len(got) {
		return "", false
	}
	return joinBlocks(out), true
}

// SelfTranslation says the English of a passage is its own translation.
//
// Chunk 30 of the historical note of chapters I to IV is twenty bibliography
// entries and nothing else, 5,940 characters of Dedekind, Peano, Hilbert and
// Zermelo. An entry stands as printed, so every line of a correct answer is a
// line of the question, and the only thing asking for it can do is go wrong. It
// did, on every route it was put to: the best answer came back with eleven of
// the twenty entries and twenty two citations missing.
//
// The test is in two halves. Nothing is left to translate once the mathematics
// and the entries are taken out, and the passage passes its own audit as its
// own answer. The second half is what keeps a heading or one stray sentence
// from riding along with a list of entries: a passage with any prose in it
// fails the language rule against itself and is asked for like anything else.
func SelfTranslation(lang, en string) bool {
	if hasProse(withoutBiblio(en)) {
		return false
	}
	return len(Audit(lang, en, en)) == 0
}

// BiblioEntry says whether a block is one numbered bibliography entry.
//
// It is exported because the same question is asked in two places. The run asks
// it of a chunk before it reads the language of the writing, and the audit asks
// it of a paragraph of a file already on disk. A rule that reads the language
// and does not ask it will call every entry an untranslated run, since standing
// as printed is precisely what AuditBiblio requires of one, and the two rules
// then hold a file between them with no answer that satisfies both.
//
// Three tests and all three have to hold. The block is numbered the way an entry
// is numbered, it carries no mathematics, since a work's name has none and an
// exercise nearly always does, and it opens on an author or on an italic title.
// See bibAuthorRE for what the last one is measured against.
func BiblioEntry(block string) bool {
	s := strings.TrimSpace(block)
	m := bibEntryRE.FindString(s)
	if m == "" || strings.Contains(s, "$") {
		return false
	}
	rest := s[len(m):]
	return bibAuthorRE.MatchString(rest) || bibTitleRE.MatchString(rest)
}

// Invariant 3 again, the part of it that catches an answer stopping early.
//
// Blocks and not paragraphs: a heading, a display, a list and a paragraph are
// each one block, separated by a blank line, and the prompt asks for one block
// out for every block in. A model that gives up two thirds of the way through a
// long § produces an answer that is perfect as far as it goes and is missing
// the end, and every other rule here reads that as a smaller section rather
// than a truncated one. This is the rule that says truncated.
func auditBlocks(en, tr string) []Problem {
	got, want := len(blocks(tr)), len(blocks(en))
	if got == want {
		return nil
	}
	verb := "and stops short of it"
	if got > want {
		verb = "and has more than it"
	}
	return []Problem{{Rule: RuleStructure,
		Msg: fmt.Sprintf("has %d blocks, the English has %d, %s", got, want, verb)}}
}

func blocks(body string) []string {
	var out []string
	for _, b := range strings.Split(corpus.NormalizeBody(body), "\n\n") {
		if strings.TrimSpace(b) != "" {
			out = append(out, footnoteDefs(b)...)
		}
	}
	return out
}

// footnoteDefRE opens a footnote definition, which is the only block of the
// corpus written one to a line with no blank line between.
var footnoteDefRE = regexp.MustCompile(`^\[\^[^\]]+\]:`)

// footnoteDefs splits a run of footnote definitions into one block each.
//
// The definitions of a body sit together at the foot of the file with no blank
// line between them, so a paragraph split reads the whole run as a single
// block, and a block is never split. The historical note of chapters I to IV
// has 83 of them and they came to 35,000 characters, six times ChunkChars, in
// one question. It was answered with nothing, twice, and the chunk sat in the
// queue.
//
// A definition that runs on to a second line keeps that line, which is what the
// leading bracket test is for. Both sides go through here, so a translation
// that writes its definitions with a blank line between them counts the same as
// one that writes them the way the English does.
func footnoteDefs(b string) []string {
	if !footnoteDefRE.MatchString(b) {
		return []string{b}
	}
	var out []string
	for _, line := range strings.Split(b, "\n") {
		if len(out) > 0 && !footnoteDefRE.MatchString(line) {
			out[len(out)-1] += "\n" + line
			continue
		}
		out = append(out, line)
	}
	return out
}

// lastBlockOf is the part of a chunk after the last blank line, which is what
// decides whether the chunk ends in a footnote definition. Join is handed whole
// chunks rather than blocks, and a chunk that opens with the prose of the note
// can still end in the first of its definitions.
func lastBlockOf(s string) string {
	if i := strings.LastIndex(s, "\n\n"); i >= 0 {
		return s[i+2:]
	}
	return s
}

// joinBlocks puts blocks back in the order they came, with the blank line
// between them that separated them, except between two footnote definitions,
// which the printing sets one to a line.
func joinBlocks(parts []string) string {
	var b strings.Builder
	for i, p := range parts {
		switch {
		case i == 0:
		case footnoteDefRE.MatchString(lastBlockOf(parts[i-1])) && footnoteDefRE.MatchString(p):
			b.WriteString("\n")
		default:
			b.WriteString("\n\n")
		}
		b.WriteString(p)
	}
	return b.String()
}

// refRE is the skeleton of a Bourbaki cross reference: the chapter in roman
// numerals where it is followed by a §, and the §, No. and p. parts wherever
// they appear.
//
// Only these. A reference reads "V, §3, No. 2, p. 18, Theorem 2", and the word
// Theorem is translated while everything else stands, so a rule that tried to
// match the whole reference would have to know the word in three languages and
// would be wrong about the fourth. These four parts are numbers and punctuation
// and are the same in every language, which makes them the part that can be
// compared as text.
//
// The number after the statement word is not matched here. It is a bare integer
// in running prose and matching those would compare every number in the
// section. It is covered for a heading by auditHeadings and it is not covered
// in prose, which is a gap and is written down as one.
// A chapter is a bare roman numeral and roman numerals are also variables in
// this book, so it counts as a citation only where a § follows it. That has to
// be one alternative rather than a lookahead, because Go's regexp has no
// lookahead, and the § it looks ahead at is then inside the match: refs puts
// both parts back.
//
// The number sign is matched with either letter, because the two halves of the
// corpus print it differently: Algebra writes "No. 1" and Theory of Sets writes
// "no. 8". Only the capital was matched, so every citation in Theory of Sets
// went past this rule, and the Vietnamese of chapter III came back with "số 8"
// in 22 places, which is the Vietnamese word for number and not the address the
// English gives.
var refRE = regexp.MustCompile(`\b([IVXLC]+),\s*(§\s*\d+)|§\s*\d+|\b[Nn]o\.\s*\d+|\bp\.\s*\d+`)

// Invariant 4. Cross references keep their numbers.
func auditRefs(en, tr string) []Problem {
	got := refs(tr)
	want := refs(en)
	if strings.Join(got, " ") == strings.Join(want, " ") {
		return nil
	}
	var out []Problem
	if missing := notIn(want, got); len(missing) > 0 {
		out = append(out, Problem{Rule: RuleReference,
			Msg: fmt.Sprintf("the English cites %d things this answer does not: %s",
				len(missing), strings.Join(missing, " "))})
	}
	if extra := notIn(got, want); len(extra) > 0 {
		out = append(out, Problem{Rule: RuleReference,
			Msg: fmt.Sprintf("cites %d things the English does not: %s",
				len(extra), strings.Join(extra, " "))})
	}
	if len(out) == 0 {
		out = append(out, Problem{Rule: RuleReference,
			Msg: "cites the same things as the English in a different order"})
	}
	return out
}

// refs is the citation skeleton of a body, in the order it is cited.
//
// The space is squeezed out of each part, so that "No. 2" and "No.2" compare
// equal. Which of the two is written is a typesetting choice and not a
// citation, and holding a translation to the English spacing inside a reference
// would fail sections for nothing.
func refs(body string) []string {
	var out []string
	for _, loc := range refRE.FindAllStringSubmatchIndex(body, -1) {
		if !opensAWord(body, loc[0]) {
			continue
		}
		if loc[2] >= 0 {
			out = append(out, tighten(body[loc[2]:loc[3]]), tighten(body[loc[4]:loc[5]]))
			continue
		}
		out = append(out, spelt(tighten(body[loc[0]:loc[1]])))
	}
	return out
}

// opensAWord says whether the character in front of position i is one that a
// word can start after.
//
// The regexp says \b and \b in Go is the ASCII one, so a letter outside ASCII
// counts as the space in front of a word. In English that never shows, because
// what stands in front of a citation is a bracket or a comma. In Vietnamese it
// shows constantly: tập is set and the corpus writes bài tập for exercise, so
// "bài tập. 2)" ends in an ASCII p, a full stop and a number, and \bp\.\s*\d+
// reads it as a citation to page 2. Two sections died of that, § 4 of Espaces
// vectoriels topologiques III and § 6 of Lie V, both refused with "cites 1
// things the English does not: p.2" on every pass, with every chunk answered
// and accepted and the whole file audit refusing the join of them. The English
// has no such citation to be missing, so nothing a model could have written
// would have passed.
//
// It is asked here rather than written into the regexp because Go has no
// lookbehind, and putting the preceding character inside the match would eat it
// and could hide a citation that follows one immediately.
func opensAWord(body string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(body[:i])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func tighten(s string) string { return strings.Join(strings.Fields(s), "") }

// spelt is a citation part with the one spelling difference taken out of it,
// for the reason tighten takes the spacing out: which letter the number sign
// starts with is the printing's choice and not the address. Algebra writes
// "No. 1" and Theory of Sets writes "no. 8", and a translation that copies the
// citation from a section of one volume into prose about the other is right
// about where it points.
func spelt(s string) string {
	if strings.HasPrefix(s, "No.") {
		return "no." + s[len("No."):]
	}
	return s
}

// wordRE is a run of letters long enough to be a word somebody translates. Two
// and not one, because a part's name is a single letter in brackets, `(a)`, and
// it is a name rather than a word: a solution cites it and it comes through as
// it stands.
var wordRE = regexp.MustCompile(`\p{L}{2,}`)

// hasProse says whether a passage has anything in it to translate.
//
// The mathematics comes out first, by the same reckoning the terminology rule
// uses: a display goes whole, an inline span goes through mathtex.Strip, and a
// heading's attribute block goes, since the identifier and the class are markup
// the translator is told to copy. What is left is the prose, and a passage with
// no word in it has none.
// A name written hard against a formula goes out with the formula. Chunk 23 of
// § 17 of Algebra VIII is eight numbered displays of the reduced norm and trace
// and nothing else, and every word in it is Pcrd, Trd or Nrd. Its translation
// is itself. Every model asked handed it back unchanged, which is right, and
// the rule below refused it as the English unchanged, six times over two days,
// and killed the file at 50 chunks of 51 answered.
func hasProse(body string) bool {
	return wordRE.MatchString(proseText(body, true))
}

// The answer has to be in the language that was asked for.
//
// Three ways it is not. A model can hand back the English unchanged, which is
// what happens when a call fails in a way that still returns text, and it can
// hand back a translation of the first paragraph followed by the rest in
// English. The first is caught by comparing with the source and the second by
// asking whether the answer carries the script at all. Neither is subtle and
// both have to be caught before a file is written, because an English file
// under content/vi is worse than no file: the audit counts it as translated.
//
// The third is subtle and it is the one that got through. The appendix on the
// trace came back with two English sentences in the middle of a Vietnamese
// paragraph, Vietnamese on both sides of them on the same line, and every test
// above passed it because each is a test of the whole chunk and the whole chunk
// is written in Vietnamese. Asked of a run of consecutive words instead, it is
// as plain as the other two. This is L11 of the audit and it is the same
// function saying it, for the reason L08 is: finding out in the first minute
// costs one more ask, and finding out afterwards costs a section.
func auditLanguage(lang, en, tr string) []Problem {
	// The bibliography goes first, because the rule beside this one says an
	// entry stands as printed and this one would then refuse the answer that
	// does it. Chunk 29 of the historical note of chapter IV is nine entries
	// and a line of prose, and the entries are German and French book titles,
	// so an answer that keeps them, which is the only answer AuditBiblio
	// accepts, carries a run of seventy eight words with nothing Vietnamese in
	// it. It was asked for eleven times over three models and refused every
	// time, half of them for translating the titles and half for not.
	//
	// A chunk that is nothing but entries has no prose left once they are out,
	// and hasProse then says so and this rule says nothing at all, which is the
	// same answer it already gives to a chunk that is nothing but displays.
	en, tr = withoutBiblio(en), withoutBiblio(tr)
	// A passage with no prose in it is a passage whose translation is itself,
	// and both rules below would refuse the only correct answer there is.
	//
	// Bourbaki writes whole blocks in symbols. One chunk of exercise 8 of the
	// appendix to chapter I is three displays of the tables for not and or and
	// nothing else, and one chunk of chapter II, § 2 is the single line
	// $A' \times B' \subset A \times B.$ Asked for those in Vietnamese, every
	// model hands back what it was given, which is right, and the run then
	// refuses it, asks twice more, kills the chunk and refuses the whole
	// section. Two files of Theory of Sets were held out of the corpus by
	// exactly this, and no number of further attempts would ever have got them
	// in.
	if !hasProse(en) {
		return nil
	}
	if strings.TrimSpace(tr) == strings.TrimSpace(en) {
		return []Problem{{Rule: RuleLanguage, Msg: "is the English, unchanged"}}
	}
	if !glossary.WrittenIn(lang, tr) {
		return []Problem{{Rule: RuleLanguage,
			Msg: "carries no " + lang + " writing at all"}}
	}
	// A work the sentence cites by name goes the way an entry goes, and for the
	// same reason. See WithoutCitations.
	if run, words := glossary.Untranslated(lang, WithoutCitations(en, tr)); words >= 2 {
		return []Problem{{Rule: RuleLanguage, Msg: fmt.Sprintf(
			"has a run of %d words with nothing of %s in it: %s",
			len(strings.Fields(run)), lang, short(run))}}
	}
	// A run is consecutive words, and a formula standing between two of them
	// breaks it. Numbered formula (11) of chapter III, § 7 is a display, the
	// word whenever, a display, the word and, and a display, and both words
	// were left in English. No run in that line is longer than one word, so
	// every test above passes it, the chunk is accepted, the file is written,
	// and L07 of the audit refuses the file over a paragraph no chunk is ever
	// going to be asked about again.
	//
	// So read the blocks one at a time as well: what a block says in words is
	// English, and none of it is in the language. Two words is the floor the
	// run uses and it is the floor here, for the same reason. The English side
	// is read too, because a block the English writes in symbols is a block
	// whose translation is itself, and the blocks line up: the rule beside this
	// one refuses an answer that has a different number of them.
	ens := blocks(en)
	for i, b := range blocks(tr) {
		if i >= len(ens) || !hasProse(ens[i]) {
			continue
		}
		text := strings.Join(strings.Fields(prose(WithoutCitations(en, b))), " ")
		if glossary.EnglishWords(text) >= 2 && !glossary.WrittenIn(lang, text) {
			return []Problem{{Rule: RuleLanguage, Msg: fmt.Sprintf(
				"has a block whose words are English and none of them %s: %s",
				lang, short(text))}}
		}
	}
	return nil
}

// scripts is what each language of the corpus is written in.
//
// A language not listed is not checked, which is the safe way round: a rule
// that does not know what a language looks like has nothing to say about it.
var scripts = map[string][]*unicode.RangeTable{
	"en": {unicode.Latin},
	"fr": {unicode.Latin},
	"vi": {unicode.Latin},
	"zh": {unicode.Han, unicode.Bopomofo, unicode.Latin},
	"ja": {unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Latin},
}

// AuditScript says the answer has to be written in one alphabet, and it is not
// always. It is exported because L13 of the audit asks the same question of the
// files already on disk, and a run that refused one thing while the audit
// reported another would be two rules pretending to be one.
//
// The introduction to Theory of Sets came back from gpt-5.4 in fluent
// Vietnamese with two words in it that are not Vietnamese and not words of the
// source either: либо, Russian for "or", standing where the Vietnamese "hoặc"
// belongs, and որևէ, Armenian for "any", standing where "bất kỳ" belongs. Both
// sit inside a correct sentence, both carry the meaning the English has, and
// every rule above passes them: the mathematics is intact, the headings are
// intact, the block count is right, the chunk is plainly written in Vietnamese
// and there is no run of two consecutive words that is not. A reader who does
// not read Russian sees a typo, and a reader who does sees that the model
// changed language for one word in the middle of a sentence.
//
// It is cheap to catch because a Vietnamese section has no business carrying a
// Cyrillic or Armenian letter at all. What it must not catch is the Greek the
// book itself sets, so a script the English source uses is a script the
// translation may use, and the mathematics is out of scope entirely: prose
// takes the displays and the inline spans out before anything here looks at a
// letter.
func AuditScript(lang, en, tr string) []Problem {
	allow, ok := scripts[lang]
	if !ok {
		return nil
	}
	quoted := straysIn(prose(en), allow)
	var out []Problem
	seen := map[string]bool{}
	for i, line := range strings.Split(prose(tr), "\n") {
		for _, word := range strings.Fields(line) {
			name := strayScript(word, allow)
			if name == "" || quoted[name] || seen[name+" "+word] {
				continue
			}
			seen[name+" "+word] = true
			out = append(out, Problem{Rule: RuleScript, Line: i + 1, Msg: fmt.Sprintf(
				"%s is written in %s, and this is %s", short(word), name, lang)})
		}
	}
	return out
}

// strayScript names the script of the first letter of a word that is not one
// the language is written in, and is empty when every letter is.
func strayScript(word string, allow []*unicode.RangeTable) string {
	for _, r := range word {
		if !unicode.IsLetter(r) || unicode.In(r, allow...) {
			continue
		}
		return scriptName(r)
	}
	return ""
}

// straysIn is every script a passage uses that the language is not written in.
func straysIn(text string, allow []*unicode.RangeTable) map[string]bool {
	out := map[string]bool{}
	for _, r := range text {
		if unicode.IsLetter(r) && !unicode.In(r, allow...) {
			out[scriptName(r)] = true
		}
	}
	return out
}

// scriptName is what Unicode calls the script a letter belongs to.
//
// The names are sorted before the search so that a letter in more than one
// table, which happens for a few of the historic ones, is reported by the same
// name every time rather than by whichever the map handed over first.
func scriptName(r rune) string {
	names := make([]string, 0, len(unicode.Scripts))
	for name := range unicode.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if unicode.Is(unicode.Scripts[name], r) {
			return name
		}
	}
	return "no script Unicode names"
}

func notIn(a, b []string) []string {
	have := map[string]int{}
	for _, x := range b {
		have[x]++
	}
	var out []string
	for _, x := range a {
		if have[x] > 0 {
			have[x]--
			continue
		}
		out = append(out, x)
	}
	return out
}

func short(s string) string {
	return window([]rune(strings.Join(strings.Fields(s), " ")), 0)
}

// ShortDiff renders two spans that are meant to be the same and are not, with
// the window placed on the first rune where they part rather than on the head
// of each.
//
// short cuts at forty characters from the front, and a span of the inverse
// limits of chapter III opens with forty characters of \alpha and \beta that
// both sides share. The message read "math span 1 is x... and the English has
// x...", with the same forty characters twice, and it is that message the model
// is handed when it is asked again. It sat in the queue for an hour being told
// nothing. The difference is what has to be on the screen.
func ShortDiff(got, want string) (string, string) {
	g := []rune(strings.Join(strings.Fields(got), " "))
	w := []rune(strings.Join(strings.Fields(want), " "))
	n := 0
	for n < len(g) && n < len(w) && g[n] == w[n] {
		n++
	}
	start := 0
	if n > 12 {
		start = n - 12 // a little of what they share, to place the rest
	}
	return window(g, start), window(w, start)
}

// window quotes at most forty characters of s from start, marking either end
// that it cuts.
func window(r []rune, start int) string {
	if start > len(r) {
		start = len(r)
	}
	lead, trail := "", ""
	if start > 0 {
		lead = "..."
	}
	seg := r[start:]
	if len(seg) > 40 {
		seg, trail = seg[:40], "..."
	}
	return fmt.Sprintf("%q", lead+string(seg)+trail)
}

func quoteOrNone(s string) string {
	if s == "" {
		return "not at all"
	}
	return s
}
