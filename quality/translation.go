package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
)

// A translation of Bourbaki is a translation of the prose and a copy of the
// mathematics. The formulae, the tags, the headings and the statement counts
// all have to come through untouched, and a model asked to translate a page
// will quietly renumber a proposition or drop a display if nothing is watching.
// These rules are what watches.
//
// These rules reported not run for as long as the corpus had no language but
// English, which is the honest state for a rule with nothing to look at. The
// appendix on the Nullstellensatz is the first section that is translated, and
// all seven of them run against it now.
//
// The one section is also where the numbers in L06 and L07 come from. That is
// thin evidence and it is written down as what it is, one section, with the
// counts that produced each number, so that the next few sections can move a
// figure rather than argue with it.

func init() {
	register(
		Check{ID: "L01", Group: Translation, Hard: true,
			Title: "the math spans are the English ones, in order", Run: l01, Need: needTranslations},
		Check{ID: "L02", Group: Translation, Hard: true,
			Title: "the tag set is the English one", Run: l02, Need: needTranslations},
		Check{ID: "L03", Group: Translation, Hard: true,
			Title: "the heading tree is the English one", Run: l03, Need: needTranslations},
		Check{ID: "L04", Group: Translation, Hard: true,
			Title: "the statement counts are the English ones", Run: l04, Need: needTranslations},
		Check{ID: "L05", Group: Translation, Hard: true,
			Title: "source_content_sha256 is the English hash as it stands", Run: l05, Need: needTranslations},
		Check{ID: "L06", Group: Translation, Hard: false,
			Title: "the glossary is followed", Run: l06, Need: needGlossary},
		Check{ID: "L07", Group: Translation, Hard: true,
			Title: "no paragraph was left untranslated", Run: l07, Need: needTranslations},
		Check{ID: "L08", Group: Translation, Hard: false,
			Title: "no translation was written by a small model", Run: l08, Need: needTranslations},
	)
}

func needTranslations(c *Corpus) string {
	if len(c.Translations()) == 0 {
		return "the corpus has no language but English"
	}
	return ""
}

func needGlossary(c *Corpus) string {
	if why := needTranslations(c); why != "" {
		return why
	}
	if _, err := os.Stat(filepath.Join(c.Root, "manifests", "glossary.yaml")); err != nil {
		return "there is no manifests/glossary.yaml to hold a translation to"
	}
	return ""
}

// pair is a translated file and the English it came from.
type pair struct {
	tr, en Doc
}

// pairs are the translated files that name an English source, with the ones
// that name none or name a file that is not there reported as they are found.
//
// The link is translated_from and not the path, because the two trees are the
// same shape only as long as nothing has been renamed, and a section file is
// named for its title.
func (c *Corpus) pairs() ([]pair, []Finding) {
	byPath := map[string]Doc{}
	for _, d := range c.Docs {
		byPath[d.Path] = d
	}
	var out []pair
	var bad []Finding
	for _, d := range c.Docs {
		if d.Lang == "en" || d.Kind == KindSolution {
			continue
		}
		from := ""
		switch {
		case d.Section != nil:
			from = d.Section.TranslatedFrom
		case d.Exercise != nil:
			from = d.Exercise.TranslatedFrom
		default:
			continue
		}
		if from == "" {
			bad = append(bad, Finding{File: d.Path, Line: 1,
				Msg: "is a translation and names no translated_from, so nothing can be compared with it"})
			continue
		}
		en, ok := byPath[from]
		if !ok {
			bad = append(bad, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("translated_from is %s and there is no such file", from)})
			continue
		}
		out = append(out, pair{d, en})
	}
	return out, bad
}

// L01. The math spans are the English ones, in order.
//
// Compared as text and in sequence, not as a count. A translation that has the
// right number of formulae in the wrong order is what comes back when a model
// reflows a paragraph, and it is invisible to anything that counts.
func l01(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		tr, _ := Math(p.tr.Body)
		en, _ := Math(p.en.Body)
		if len(tr) != len(en) {
			out = append(out, Finding{File: p.tr.Path, Line: 1,
				Msg: fmt.Sprintf("has %d math spans and the English has %d", len(tr), len(en))})
			continue
		}
		for i := range tr {
			if tr[i].Text != en[i].Text || tr[i].Display != en[i].Display {
				out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(tr[i].Line),
					Msg: fmt.Sprintf("math span %d is %q and the English has %q",
						i+1, ellipsis(tr[i].Text, 40), ellipsis(en[i].Text, 40))})
				break
			}
		}
	}
	return out, nil
}

// tagRE and headRE read the two things a heading carries.
var (
	tagAttrRE = regexp.MustCompile(`\btag=([^} ]*)`)
	headingRE = regexp.MustCompile(`^(#{1,6}) (.*?)(?:\s*\{#([a-z0-9-]+)([^}]*)\})?$`)
)

// L02. The tag set is the English one.
//
// A tag is the one identifier that works across all four languages, which is
// the whole reason for having tags at all. A translation that mints its own is
// T07; one that loses or duplicates an English one is this.
func l02(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		tr, en := tagList(p.tr), tagList(p.en)
		if strings.Join(tr, " ") == strings.Join(en, " ") {
			continue
		}
		missing := notIn(en, tr)
		extra := notIn(tr, en)
		switch {
		case len(missing) > 0:
			out = append(out, Finding{File: p.tr.Path, Line: 1,
				Msg: fmt.Sprintf("the English has %d tags this file does not: %s",
					len(missing), strings.Join(missing, " "))})
		case len(extra) > 0:
			out = append(out, Finding{File: p.tr.Path, Line: 1,
				Msg: fmt.Sprintf("carries %d tags the English does not: %s",
					len(extra), strings.Join(extra, " "))})
		default:
			out = append(out, Finding{File: p.tr.Path, Line: 1,
				Msg: "carries the English tags in a different order"})
		}
	}
	return out, nil
}

func tagList(d Doc) []string {
	var out []string
	if d.Exercise != nil && d.Exercise.Tag != "" {
		out = append(out, d.Exercise.Tag)
	}
	for _, line := range strings.Split(d.Body, "\n") {
		if m := tagAttrRE.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

func notIn(a, b []string) []string {
	have := map[string]bool{}
	for _, x := range b {
		have[x] = true
	}
	var out []string
	for _, x := range a {
		if !have[x] {
			out = append(out, x)
		}
	}
	return out
}

// L03. The heading tree is the English one.
//
// The levels and the anchors, not the text: the text is the thing being
// translated. An anchor is a permanent label and a level is the structure of
// the §, and neither is a translator's to change.
func l03(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		tr, en := headingTree(p.tr), headingTree(p.en)
		if strings.Join(tr, "\n") == strings.Join(en, "\n") {
			continue
		}
		out = append(out, Finding{File: p.tr.Path, Line: 1,
			Msg: fmt.Sprintf("the heading tree is %d deep against the English %d, and %s",
				len(tr), len(en), firstDiff(tr, en))})
	}
	return out, nil
}

// headingTree is the level and the anchor of every heading, which is the shape
// of the file with the prose taken out.
func headingTree(d Doc) []string {
	var out []string
	for _, line := range strings.Split(d.Body, "\n") {
		m := headingRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, fmt.Sprintf("%s %s", m[1], m[3]))
	}
	return out
}

func firstDiff(a, b []string) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return fmt.Sprintf("heading %d is %q against %q", i+1, a[i], b[i])
		}
	}
	if len(a) < len(b) {
		return fmt.Sprintf("the English has %q and this file ends", b[len(a)])
	}
	return fmt.Sprintf("this file has %q and the English ends", a[len(b)])
}

// L04. The statement counts are the English ones.
//
// Counted by kind, so a Proposition turned into a Theorem is caught as well as
// one that went missing. The kind comes off the permanent label rather than off
// the heading, since the heading is in the language being checked.
func l04(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		tr, en := kindCounts(p.tr), kindCounts(p.en)
		for _, k := range sortedStrings(en) {
			if tr[k] != en[k] {
				out = append(out, Finding{File: p.tr.Path, Line: 1,
					Msg: fmt.Sprintf("has %d of kind %s and the English has %d", tr[k], k, en[k])})
			}
		}
		for _, k := range sortedStrings(tr) {
			if _, ok := en[k]; !ok {
				out = append(out, Finding{File: p.tr.Path, Line: 1,
					Msg: fmt.Sprintf("has %d of kind %s and the English has none", tr[k], k)})
			}
		}
	}
	return out, nil
}

func kindCounts(d Doc) map[string]int {
	out := map[string]int{}
	for _, line := range strings.Split(d.Body, "\n") {
		m := headingRE.FindStringSubmatch(line)
		if m == nil || m[3] == "" {
			continue
		}
		ref, err := corpus.ParseLabel(m[3])
		if err != nil {
			continue
		}
		out[string(ref.Kind)]++
	}
	return out
}

// L05. source_content_sha256 is the English hash as it stands.
//
// This is what makes translating four languages affordable. Re-translating
// everything on every English edit costs more than anybody will pay and
// re-translating nothing leaves the corpus lying about what it holds, so the
// English hash is recorded at translation time and the stale files are exactly
// the ones where it no longer matches.
func l05(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		var recorded string
		switch {
		case p.tr.Section != nil:
			recorded = p.tr.Section.SourceSHA256
		case p.tr.Exercise != nil:
			recorded = p.tr.Exercise.SourceSHA256
		}
		now := corpus.ContentSHA256(p.en.Body)
		switch {
		case recorded == "":
			out = append(out, Finding{File: p.tr.Path, Line: 1,
				Msg: "records no source_content_sha256, so nothing can say whether it is stale"})
		case recorded != now:
			out = append(out, Finding{File: p.tr.Path, Line: 1,
				Msg: fmt.Sprintf("was translated from %s and %s is now %s, so it is stale",
					short(recorded), p.en.Path, short(now))})
		}
	}
	return out, nil
}

// L06. The glossary is followed.
//
// Bourbaki's vocabulary is fixed and a translation that renders "artinian ring"
// three ways across one § is a translation nobody can search. The glossary is
// the agreed rendering per term per language, and the prompt puts the rows that
// match a chunk in front of the model. This reads the finished file and asks
// the same question of the answer that the chunker asked of the source: the
// terms the section was shown, and the ones it used.
//
// Soft, and it has to be. The rendering is looked for as it stands, and a
// language does things to a word that a literal search does not follow. So a
// finding here is a term to look at rather than a section to throw away, and
// the hard rules are the ones about mathematics, tags and structure, which are
// mechanical and admit no judgement.
//
// Measured on the first translated section, the appendix on the
// Nullstellensatz: the English mentions 57 glossary terms and the Vietnamese
// carries 56 of them as the glossary writes them. The one it does not is
// "respect", and the rule is right and the glossary is wrong. The row renders
// it as bảo toàn, which is the verb, a map respecting a structure. The corpus
// uses the word 33 times and all 33 of them are "with respect to", which is a
// preposition and not a term at all. The row went out in the prompt for every
// chunk that contained the phrase, inviting exactly the mistake the translator
// did not make. So the first thing this rule found was a bad row, which is one
// of the two things it is for.
func l06(c *Corpus) ([]Finding, error) {
	g, err := glossary.Load(c.Root)
	if err != nil {
		return nil, err
	}
	ps, out := c.pairs()
	for _, p := range ps {
		en := strings.ToLower(prose(p.en.Body))
		tr := strings.ToLower(prose(p.tr.Body))
		var missed []string
		for _, t := range g.Mentioned(p.tr.Lang, en) {
			if glossary.Follows(p.tr.Lang, tr, t.In(p.tr.Lang)) {
				continue
			}
			missed = append(missed, fmt.Sprintf("%s (%s)", t.EN, t.In(p.tr.Lang)))
		}
		if len(missed) == 0 {
			continue
		}
		out = append(out, Finding{File: p.tr.Path, Line: 1,
			Msg: fmt.Sprintf("the English mentions %d glossary terms and %d are not in this file as the glossary writes them: %s",
				len(g.Mentioned(p.tr.Lang, en)), len(missed), strings.Join(missed, ", "))})
	}
	return out, nil
}

// prose is a body with the things a translator was told to copy taken out: the
// mathematics, and the attribute block of a heading.
//
// Both would otherwise be read as vocabulary. A heading carries
// {#alg-viii-a3-thm-1 .statement tag=00QM}, and .statement is a word; a formula
// carries \operatorname{Spec} and the glossary has a row for spectrum. A term
// that appears only inside one of those is a term the section was never asked
// to render.
// A display block is dropped whole, the same way paragraphs drops it.
func prose(body string) string {
	var b strings.Builder
	inDisplay := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "$$" {
			inDisplay = !inDisplay
			continue
		}
		if inDisplay {
			continue
		}
		if m := headingRE.FindStringSubmatch(line); m != nil {
			b.WriteString(stripMath(m[2]))
			b.WriteString("\n")
			continue
		}
		b.WriteString(stripMath(line))
		b.WriteString("\n")
	}
	return b.String()
}

// L07. No paragraph was left untranslated.
//
// The failure this catches is a model that returns the English for a paragraph
// it could not handle, which reads as a complete translation to anything that
// counts paragraphs or hashes files.
//
// The test is per language and is about script rather than about vocabulary.
// Chinese and Japanese prose contains Han characters or kana; a paragraph of
// running prose with none is English that came back unchanged. Vietnamese is
// written in the Latin alphabet, so the test there is the diacritics, which
// Vietnamese uses in almost every sentence and English uses in none.
//
// The floor was twelve words and it was a guess, written down as a guess because
// there was no translation to measure it against. There is one now, and the
// first thing the measurement says is that the guess was expensive: the corpus
// has 3,891 English paragraphs and 921 of them, 23.7 per cent, are under twelve
// words. A floor that blinds a hard rule to a quarter of the book is not a
// floor, it is a hole.
//
// The second thing it says is that counting words is the wrong idea altogether.
// paragraphs runs after the mathematics has been taken out, so what is left of a
// display line is punctuation and operator names: "(1) Card(J) Card(I ." is four
// words and none of them are words, and a translation leaves it exactly as it
// stands. Counting the ones that start lower case does better and still leaks,
// because Bourbaki has lower case operators too and "long dim ." reads as two
// words of prose.
//
// So the question is not how long the paragraph is, it is whether the paragraph
// is English. That is what the failure is. englishWords counts the words that
// hold an English sentence together, and the separation is total: of the 3,891
// English paragraphs, 288 carry none of them and those 288 are the residue,
// while every one of the fourteen Vietnamese paragraphs in the corpus carries
// none either. Two, then, with a floor of two skipping 400 paragraphs of 3,891,
// 10.3 per cent against 23.7, and leaving two clear of everything real that has
// been measured. That margin is what a hard rule needs, since a false positive
// here fails the build.
func l07(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		for i, para := range paragraphs(p.tr.Body) {
			words := englishWords(para.text)
			if words < 2 {
				continue
			}
			if translatedInto(p.tr.Lang, para.text) {
				continue
			}
			out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(para.line),
				Msg: fmt.Sprintf("paragraph %d carries %d English words and nothing of %s: %s",
					i+1, words, p.tr.Lang, ellipsis(para.text, 50))})
		}
	}
	return out, nil
}

// english is the words that hold an English sentence together and carry no
// mathematics of their own. A paragraph with two of them is an English
// paragraph; what is left of a display when the dollars come off has none.
//
// Four words are missing from it that belong there on the face of it, and they
// are missing because Vietnamese has them too. "in" is to print, "to" is big,
// "an" is peace, "do" is the first half of "do đó", which is how a Vietnamese
// proof says therefore and which turned up in two of the fourteen paragraphs
// measured. "so", "may" and "can" are out for the same reason. A word that both
// languages spell the same way says nothing about which language a paragraph is
// in, and this list is only worth having if every word on it does.
var english = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`the of is and for be we that it as if then every there
this which are has have let was were from but not all such where when its they one on or
thus hence therefore follows also only same each other into over under between because since
while what who whose whom does did shall will would could might must been being had here now
first second third these those`) {
		english[w] = true
	}
}

func englishWords(s string) int {
	n := 0
	for _, w := range strings.Fields(strings.ToLower(s)) {
		if english[strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) })] {
			n++
		}
	}
	return n
}

// translatedInto asks whether this text carries the script of the language.
//
// The test itself lives in the glossary package, which is where the same
// question gets asked one term at a time as renderings come back from a model.
// One copy, because two would drift and the drift would show up as a paragraph
// this rule passes and the glossary refuses.
func translatedInto(lang, text string) bool { return glossary.WrittenIn(lang, text) }

type para struct {
	text string
	line int
}

// paragraphs splits a body into its prose, with the headings and the display
// mathematics taken out. Inline mathematics is replaced by a space rather than
// removed, so that a paragraph that is one long formula does not read as twelve
// words of untranslated English.
func paragraphs(body string) []para {
	var out []para
	var cur []string
	start := 0
	flush := func() {
		if len(cur) > 0 {
			if t := strings.TrimSpace(stripMath(strings.Join(cur, " "))); t != "" {
				out = append(out, para{t, start})
			}
			cur = nil
		}
	}
	inDisplay := false
	for i, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "$$":
			inDisplay = !inDisplay
			flush()
			continue
		case inDisplay:
			continue
		case trimmed == "":
			flush()
			continue
		case strings.HasPrefix(trimmed, "#"):
			flush()
			continue
		}
		if len(cur) == 0 {
			start = i + 1
		}
		cur = append(cur, trimmed)
	}
	flush()
	return out
}

// stripMath takes the inline mathematics out of a line of prose.
func stripMath(s string) string {
	var b strings.Builder
	in := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch {
		case rs[i] == '\\' && i+1 < len(rs):
			if !in {
				b.WriteRune(rs[i])
				b.WriteRune(rs[i+1])
			}
			i++
		case rs[i] == '$':
			in = !in
			b.WriteRune(' ')
		case !in:
			b.WriteRune(rs[i])
		}
	}
	return b.String()
}

// smallModel is the name of a model that is a cut down version of another one.
//
// Matched on the name and not on a list of models, because the list changes
// under us and the naming does not: a provider that serves a smaller variant
// says so in the suffix.
var smallModel = regexp.MustCompile(`(?i)[-_](mini|nano|lite|small|flash|turbo)\b`)

// L08. No translation was written by a small model.
//
// Nobody chooses the model here. The ask goes to a browser profile signed in to
// an account and whatever that account is being served comes back, and the name
// of it is what the answer reports. Two runs of the same section on the same
// host a half hour apart came back as gpt-5-6 and then gpt-5-6-mini, which is
// the account having been moved down between them.
//
// The second translation was measurably worse in one place and better in
// another. "no common zero" went from "không có không điểm chung", which is
// right, to "không có một không chung", which is not Vietnamese for anything;
// and "integral over K" went from "nguyên trên K" to "đại số trên K", algebraic,
// which is what the English of that line actually says. One better and one
// worse is not evidence that the small model is fine, it is evidence that
// nobody was looking, and this rule is what looks.
//
// Soft, and it has to be. The text may well be good and the corpus should not
// go red because an account was throttled at the wrong minute. What it should
// do is say which files were written that way, so that a later pass can decide
// to do them again rather than discover it by reading.
func l08(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Lang == "" || d.Lang == "en" {
			continue
		}
		var model string
		switch {
		case d.Section != nil:
			model = d.Section.TranslationModel
		case d.Exercise != nil:
			model = d.Exercise.TranslationModel
		}
		if !smallModel.MatchString(model) {
			continue
		}
		out = append(out, Finding{File: d.Path, Line: 1,
			Msg: fmt.Sprintf("was translated by %s, which is a cut down model, so the section is worth doing again", model)})
	}
	return out, nil
}
