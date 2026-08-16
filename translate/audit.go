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
	"strings"

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
	RuleMath        = "math"
	RuleMathProse   = "math prose"
	RuleTag         = "tag"
	RuleStructure   = "structure"
	RuleReference   = "reference"
	RuleFrontMatter = "front matter"
	RuleCommentary  = "commentary"
	RuleLanguage    = "language"
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
	out = append(out, auditCommentary(tr)...)
	out = append(out, auditMath(en, tr)...)
	out = append(out, auditAttrs(en, tr)...)
	out = append(out, auditHeadings(en, tr)...)
	out = append(out, auditBlocks(en, tr)...)
	out = append(out, auditRefs(en, tr)...)
	out = append(out, auditLanguage(lang, en, tr)...)
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
	if len(got) != len(want) {
		out = append(out, Problem{Rule: RuleMath,
			Msg: fmt.Sprintf("has %d math spans and the English has %d", len(got), len(want))})
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		if glossary.SameMath(want[i].Text, got[i].Text) && got[i].Display == want[i].Display {
			continue
		}
		out = append(out, Problem{Rule: RuleMath, Line: got[i].Line,
			Msg: fmt.Sprintf("math span %d is %s and the English has %s",
				i+1, short(got[i].Text), short(want[i].Text))})
		// One is enough. After the first difference the spans are out of step
		// and every one after it reports as changed, which buries the one that
		// actually moved under a hundred that did not.
		break
	}
	return append(out, auditMathProse(en, tr)...)
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
			out = append(out, b)
		}
	}
	return out
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
var refRE = regexp.MustCompile(`\b([IVXLC]+),\s*(§\s*\d+)|§\s*\d+|\bNo\.\s*\d+|\bp\.\s*\d+`)

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
	for _, m := range refRE.FindAllStringSubmatch(body, -1) {
		if m[1] != "" {
			out = append(out, tighten(m[1]), tighten(m[2]))
			continue
		}
		out = append(out, tighten(m[0]))
	}
	return out
}

func tighten(s string) string { return strings.Join(strings.Fields(s), "") }

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
	if strings.TrimSpace(tr) == strings.TrimSpace(en) {
		return []Problem{{Rule: RuleLanguage, Msg: "is the English, unchanged"}}
	}
	if !glossary.WrittenIn(lang, tr) {
		return []Problem{{Rule: RuleLanguage,
			Msg: "carries no " + lang + " writing at all"}}
	}
	if run, words := glossary.Untranslated(lang, tr); words >= 2 {
		return []Problem{{Rule: RuleLanguage, Msg: fmt.Sprintf(
			"has a run of %d words with nothing of %s in it: %s",
			len(strings.Fields(run)), lang, short(run))}}
	}
	return nil
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
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= 40 {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%q", s[:40]+"...")
}

func quoteOrNone(s string) string {
	if s == "" {
		return "not at all"
	}
	return s
}
