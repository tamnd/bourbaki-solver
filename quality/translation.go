package quality

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/textguard"
	"github.com/tamnd/bourbaki-solver/translate"
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
		Check{ID: "L09", Group: Translation, Hard: true,
			Title: "the glossary version moves when the renderings do", Run: l09, Need: needGlossaryBase},
		Check{ID: "L10", Group: Translation, Hard: true,
			Title: "no English term was left standing", Run: l10, Need: needGlossary},
		Check{ID: "L11", Group: Translation, Hard: true,
			Title: "no sentence was left untranslated", Run: l11, Need: needTranslations},
		Check{ID: "L12", Group: Translation, Hard: true,
			Title: "a word set inside the mathematics is translated too", Run: l12, Need: needTranslations},
		Check{ID: "L13", Group: Translation, Hard: true,
			Title: "no word is written in another alphabet", Run: l13, Need: needTranslations},
		Check{ID: "L14", Group: Translation, Hard: true,
			Title: "a bibliography entry stands as printed", Run: l14, Need: needTranslations},
		Check{ID: "L15", Group: Translation, Hard: false,
			Title: "no translation was written on the free gateway", Run: l15, Need: needTranslations},
		Check{ID: "L16", Group: Translation, Hard: true,
			Title: "no paragraph of the machine English came back in French", Run: l16, Need: needTranslations},
		Check{ID: "L17", Group: Translation, Hard: true,
			Title: "no translation is a provider error", Run: l17, Need: needTranslations},
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

// A Pair is what pair is, for a caller outside the rules.
//
// The rules are not the only thing that has to walk the translations against
// their sources: report translation counts what is translated and how closely
// the glossary is followed, and it has to pair the files the same way the rules
// do or it will report on a set the rules never looked at.
type Pair struct {
	Translation Doc
	English     Doc
}

// Pairs is every translated file that names an English source, with the ones
// that name none or name a file that is not there left out. L01 is what reports
// those, so a report that also reported them would say the same thing twice.
func (c *Corpus) Pairs() []Pair {
	ps, _ := c.pairs()
	out := make([]Pair, 0, len(ps))
	for _, p := range ps {
		out = append(out, Pair{Translation: p.tr, English: p.en})
	}
	return out
}

// Prose is a body with the mathematics and the heading attributes taken out,
// which is the only text a glossary term can honestly be looked for in. It is
// exactly what L06 reads, exported so that the report and the rule cannot drift
// apart on what counts as a mention.
func Prose(body string) string { return prose(body) }

// pairs are the translated files that name an English source, with the ones
// that name none or name a file that is not there reported as they are found.
//
// The link is translated_from and not the path, because the two trees are the
// same shape only as long as nothing has been renamed, and a section file is
// named for its title.
func (c *Corpus) pairs() ([]pair, []Finding) {
	byPath := map[string]Doc{}
	// Sources first, so that a language which is both audited and a source is
	// taken from the list the rules walk and there is one Doc for one path.
	for _, d := range c.Sources {
		byPath[d.Path] = d
	}
	for _, d := range c.Docs {
		byPath[d.Path] = d
	}
	var out []pair
	var bad []Finding
	source := c.SourceLangs()
	for _, d := range c.Docs {
		if source[d.Lang] || d.Kind == KindSolution {
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
//
// The one part of a span a translation may change is the words inside a \text,
// which are prose set in a formula because TeX has no other way of writing a
// word in one. glossary.SameMath is where that is decided and why; L12 is the
// rule that those words were in fact translated.
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
			if !glossary.SameMath(en[i].Text, tr[i].Text) || tr[i].Display != en[i].Display {
				mine, theirs := translate.ShortDiff(tr[i].Text, en[i].Text)
				out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(tr[i].Line),
					Msg: fmt.Sprintf("math span %d is %s and the English has %s", i+1, mine, theirs)})
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
		// The rows this section was shown are its own volume's, since a row
		// can be scoped to a book. See glossary.Glossary.For.
		mentioned := g.For(BookOf(p.tr)).Mentioned(p.tr.Lang, en)
		for _, t := range mentioned {
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
				len(mentioned), len(missed), strings.Join(missed, ", "))})
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
	for _, line := range strings.Split(mathtex.BlankDisplays(body), "\n") {
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
			// A work the sentence cites by name stands as printed, the way an
			// entry does. See translate.WithoutCitations.
			text := translate.WithoutCitations(p.en.Body, para.text)
			words := englishWords(text)
			if words < 2 {
				continue
			}
			if translate.BiblioEntry(para.text) {
				continue // it stands as printed, and L14 is what watches that
			}
			if translatedInto(p.tr.Lang, text) {
				continue
			}
			out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(para.line),
				Msg: fmt.Sprintf("paragraph %d carries %d English words and nothing of %s: %s",
					i+1, words, p.tr.Lang, ellipsis(text, 50))})
		}
	}
	return out, nil
}

// L11. No sentence was left untranslated.
//
// L07 reads a paragraph and this reads a run of words inside one, which is the
// difference between the two failures rather than a stricter version of the
// same one.
//
// The appendix on the trace of an endomorphism is where it comes from. Fifteen
// chunks went over and fourteen came back Vietnamese; the eleventh came back
// with two English sentences in the middle of a paragraph, Vietnamese on both
// sides of them on the same line. L07 saw a paragraph written in Vietnamese,
// because it was. L01 to L05 saw the mathematics, the tags, the headings and
// the counts all correct, because they were. The only rule that said anything
// was L10, which reported the one sentence eight times over as eight glossary
// terms left standing, and eight findings that name eight words are not a rule
// saying "this sentence is in English".
//
// The unit is the run of consecutive words carrying none of the language, and
// the floor is two English words in that run, which is what L07 measured for a
// paragraph. Over the corpus as it stands exactly one run anywhere reaches it
// and it is this one, at fourteen English words in thirty two. The next run
// down carries none and is the residue of a display. Hard, on that margin, and
// because a sentence of the book missing from the translation is not something
// a reader can be left to notice.
func l11(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		for i, para := range paragraphs(p.tr.Body) {
			text := translate.WithoutCitations(p.en.Body, para.text)
			run, words := glossary.Untranslated(p.tr.Lang, text)
			if words < 2 {
				continue
			}
			if translate.BiblioEntry(para.text) {
				continue // it stands as printed, and L14 is what watches that
			}
			if !translatedInto(p.tr.Lang, text) {
				// The whole paragraph came back in English and L07 says so.
				// Two rules on one paragraph is one finding too many.
				continue
			}
			out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(para.line),
				Msg: fmt.Sprintf("paragraph %d has a run of %d words with nothing of %s in it: %s",
					i+1, len(strings.Fields(run)), p.tr.Lang, ellipsis(run, 60))})
		}
	}
	return out, nil
}

// L12. A word set inside the mathematics is translated too.
//
// Theory of Sets is the volume that needed this. The other five write formulae
// out of symbols, and the assumption underneath L07, that what is left of a
// display when the dollars come off carries no English, holds for every one of
// them. Chapter I of Theory of Sets states its criteria as
//
//	$((\text{not } A) \text{ or } B) \Rightarrow ((\text{not not } A) \text{ or } B)$
//
// and a translation that copies the mathematics through as it was told to hands
// back a Vietnamese chapter whose every formula is half English. L07 saw it,
// twenty six times, and had nothing to say about it that anybody could act on:
// the paragraph is a display and the words are inside the mathematics, which
// L01 forbade changing. Two hard rules, one file, no way through.
//
// So the mathematics is copied byte for byte apart from those runs, which are
// prose and are translated, and this is the rule that they were. What counts as
// prose and what counts as a name the printing sets upright is decided in
// glossary.UntranslatedMathProse, off the same word list L07 and L11 use.
//
// Hard, and for L07's reason rather than L10's: a formula that says "not" to a
// reader who does not read English is not a formula that reader can use, and
// nothing else in the audit will ever mention it.
func l12(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		tr, _ := Math(p.tr.Body)
		en, _ := Math(p.en.Body)
		if len(tr) != len(en) {
			continue // L01 says so, and the spans are not paired up any more
		}
		for i := range tr {
			for _, run := range glossary.UntranslatedMathProse(en[i].Text, tr[i].Text) {
				out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(tr[i].Line),
					Msg: fmt.Sprintf("math span %d holds %s, which is prose and is still in English",
						i+1, run)})
			}
		}
	}
	return out, nil
}

// L13. No word is written in another alphabet.
//
// The introduction to Theory of Sets is where this came from. It came back in
// Vietnamese with либо standing where hoặc belongs and որևէ standing where bất
// kỳ belongs, one Russian word and one Armenian word, both inside sentences
// that are otherwise right, and the twelve rules above all passed it: the
// mathematics is intact, the tags and the headings and the counts are the
// English ones, the file plainly reads as Vietnamese, and there is no run of
// two words that carries none of it. A model changing alphabet for one word is
// not a failure any of them was built to see.
//
// The rule is the run's, translate.AuditScript, called on the files as they
// stand. Anything written before the run had it is caught here rather than
// never, which is the point of running the audit over the corpus rather than
// only over what is being written today.
//
// Hard. A word a reader cannot read is not a translation of the word the book
// has, and unlike a clumsy rendering there is no reading of it that is right.
func l13(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		for _, q := range translate.AuditScript(p.tr.Lang, p.en.Body, p.tr.Body) {
			out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(q.Line), Msg: q.Msg})
		}
	}
	return out, nil
}

// L14. A bibliography entry stands as printed.
//
// The historical note of chapter III came back with its bibliography in
// Vietnamese: Vorlesungen über die Geschichte der antiken Mathematik is a
// Vietnamese title in that file, and the work behind it cannot be looked up
// under a name no library has. The entry is the one part of a note that is
// there to be followed rather than read.
//
// The rule is the run's, translate.AuditBiblio, so a file written before the
// run had it is caught here rather than never. That is why the audit runs over
// the corpus and not only over what is being written today.
//
// Hard. A citation that leads nowhere is worse than no citation, because a
// reader cannot tell from the page that it leads nowhere.
func l14(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		for _, q := range translate.AuditBiblio(p.en.Body, p.tr.Body) {
			out = append(out, Finding{File: p.tr.Path, Line: 1, Msg: q.Msg})
		}
	}
	return out, nil
}

// L16. No paragraph of the machine English came back in French.
//
// L07 and L11 ask whether a translation carries the writing of its language,
// and for Vietnamese, Chinese and Japanese that is a question the letters
// answer. English out of French is two languages in one alphabet, so the
// letters answer nothing and both rules pass a paragraph that was never
// translated at all. Measured on the corpus when this was written: seven files
// of content/en-mt held eighteen paragraphs of French, one of them the note to
// the reader of Topologie algebrique from its first line to its last, and the
// audit had nothing to say about any of them.
//
// The test is the French word list against the English one, on the paragraph
// rather than on a run of words inside it. Three French words and no English
// word is the line, and it is drawn there because one French word is a name the
// printing keeps and two are a name and a preposition, while an English
// paragraph that holds no English word at all is a paragraph of pure
// mathematics, which paragraphs already drops.
//
// Hard. A paragraph left in French in the English tree is a paragraph the
// Vietnamese will be made from, so it does not stop at one file.
func l16(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		if !strings.EqualFold(p.en.Lang, "fr") {
			continue
		}
		for i, para := range paragraphs(p.tr.Body) {
			fr := glossary.FrenchWords(para.text)
			if fr < 3 || englishWords(para.text) > 0 {
				continue
			}
			if translate.BiblioEntry(para.text) {
				continue // it stands as printed, and L14 is what watches that
			}
			out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(para.line),
				Msg: fmt.Sprintf("paragraph %d carries %d French words and no English: %s",
					i+1, fr, ellipsis(para.text, 50))})
		}
	}
	return out, nil
}

// englishWords counts the words that hold an English sentence together and
// carry no mathematics of their own. A paragraph with two of them is an English
// paragraph; what is left of a display when the dollars come off has none.
//
// The list itself is in the glossary package beside the script test, for the
// reason the script test is there: L11 asks the same question of a run of words
// that this asks of a paragraph, the run that writes the translations asks it a
// third time before an answer goes anywhere near a file, and three copies of a
// word list drift three ways.
func englishWords(s string) int { return glossary.EnglishWords(s) }

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
	for i, line := range strings.Split(mathtex.BlankDisplays(body), "\n") {
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

// stripMath takes the inline mathematics out of a line of prose. It is
// mathtex.Strip, which the run uses as well, so that the rule the audit reports
// and the rule the run refuses on cannot come apart.
func stripMath(s string) string { return mathtex.Strip(s) }

// smallModel is the name of a model that is a cut down version of another one.
//
// Matched on the name and not on a list of models, because the list changes
// under us and the naming does not: a provider that serves a smaller variant
// says so in the suffix.
//
// lightning is in the list because nemotron-3.5-lightning-free is the cut down
// nemotron and says so the way flash and turbo say it, and without the word
// here the rule read seven files of chapter I as full model work. The suffix is
// not always last: that name carries -free after it, so the word is matched
// where it stands rather than at the end.
var smallModel = regexp.MustCompile(`(?i)[-_](mini|nano|lite|small|flash|turbo|lightning)\b`)

// SmallModel is the same test, for the run rather than for the audit.
//
// L08 finds a cut down model after the file is written, which for a section of
// fifteen chunks is eleven minutes too late and for a chapter of twenty six
// sections is a night of it. The run says so on the first chunk that comes back
// that way, and it is the same rule saying it, because two answers to "is this a
// small model" is one answer too many.
func SmallModel(name string) bool { return smallModel.MatchString(name) }

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
		if !SmallModel(model) {
			continue
		}
		out = append(out, Finding{File: d.Path, Line: 1,
			Msg: fmt.Sprintf("was translated by %s, which is a cut down model, so the section is worth doing again", model)})
	}
	return out, nil
}

// L15. No translation was written on the free gateway.
//
// L08 reads the model name for a cut down variant of a model, which is a suffix
// on a name: mini, nano, flash, lightning. The free gateway is not that. It is
// a different provider serving different models altogether, and none of their
// names carries a suffix that says anything, so a section written on
// nemotron-3-ultra-free reads to L08 as full model work and to a reader of the
// corpus as nothing at all.
//
// It matters because of when the gateway gets used. Nobody reaches for it while
// the subscription has allowance left; it is what the run falls back to when
// codex is out and the boxes will not answer, which is to say it writes the
// files nothing else would write, and those are usually the awkward ones. Eight
// chunks of the historical note of chapter IV are the case in front of us.
//
// Soft, for L08's reason. The text may well be right, and the corpus should not
// go red because the good routes were unavailable on the day. What it should do
// is say so, so that a later pass with allowance can ask again for these and
// not discover them by reading.
//
// The test is the -free suffix the gateway puts on every model it serves, in
// route.Default and in the catalogue behind it, and not a list of names, for
// the reason smallModel gives: the list changes under us and the naming does
// not.
func l15(c *Corpus) ([]Finding, error) {
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
		if !FreeGatewayModel(model) {
			continue
		}
		out = append(out, Finding{File: d.Path, Line: 1, Msg: fmt.Sprintf(
			"was translated by %s, which is a free gateway model, so the section is worth doing again", model)})
	}
	return out, nil
}

// FreeGatewayModel says whether a model name is one the free gateway serves.
//
// A file translated on two routes records both names, so the test is per name
// rather than on the whole string: one gateway answer anywhere in a file is a
// file worth asking for again, which is how L08 reads a cut down model too.
func FreeGatewayModel(name string) bool {
	for _, part := range strings.Split(name, ", ") {
		if freeModel.MatchString(strings.TrimSpace(part)) {
			return true
		}
	}
	return false
}

// freeModel is the suffix the gateway puts on the models it gives away.
var freeModel = regexp.MustCompile(`(?i)[-_]free$`)

// L10. No English term was left standing.
//
// L07 catches a paragraph that came back in English. This catches a word.
//
// The two fail differently and only one of them is visible. A paragraph that
// was not translated is obvious to anybody who opens the file; a single English
// word inside a Vietnamese sentence is not, and it survives every rule the
// corpus had. L07 asks whether the paragraph is written in the language, and a
// sentence with one English noun in it is still written in the language, so it
// passes. L06 asks whether the rendering is present, and a section that renders
// the term correctly in nine places and leaves it in English in the tenth has
// the rendering present, so it passes too. The corpus shipped exactly that:
// "Theo Corollary da dan" in the appendix on the trace, where every other
// mention of the word is "he qua".
//
// The test is the glossary read the other way round. L06 asks whether the
// rendering is there; this asks whether the English is still there. A term the
// glossary has a rendering for is a term the translator was shown the rendering
// for, in the prompt, for the chunk it appears in, so the English standing in
// the finished file is not a judgement call about vocabulary. It is the one
// word the model did not do what it was told with.
//
// Hard, because there is nothing to weigh. A row whose rendering is the English
// word is skipped, since there is nothing to leak, and the mathematics and the
// heading attributes are out before the search starts, so what is left is prose
// the translator was asked to write in another language and did not.
//
// It reaches as far as the glossary does and no further. The same appendix
// opens a sentence with "Denote", which the glossary has no row for and so this
// rule cannot see. The answer to that is a row, not a looser rule: a word list
// of English guessed at from two translated files would be a guess, and this is
// a measurement.
func l10(c *Corpus) ([]Finding, error) {
	g, err := glossary.Load(c.Root)
	if err != nil {
		return nil, err
	}
	ps, out := c.pairs()
	for _, p := range ps {
		// Its own volume's rows, as in L06: a term scoped to another book was
		// never in this file's prompt and cannot be what it was told to write.
		//
		// The test itself is translate.AuditTerms, which is the same test the
		// run makes of every chunk as it comes back. Two implementations of one
		// rule is how the run came to accept what the audit then refused, and
		// how both of them came to read \square inside a one line display as
		// the English word square.
		for _, why := range translate.AuditTerms(p.tr.Lang, g.For(BookOf(p.tr)), p.en.Body, p.tr.Body) {
			term := quoted(why.Msg)
			out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(mentionLine(p.tr.Body, term)),
				Msg: why.Msg})
		}
	}
	return out, nil
}

// quoted is the first quoted run of a complaint, which is the English term the
// rule is about. The complaint is built next door in package translate and the
// term is what a finding has to point a line number at.
func quoted(msg string) string {
	_, rest, ok := strings.Cut(msg, "\"")
	if !ok {
		return ""
	}
	term, _, ok := strings.Cut(rest, "\"")
	if !ok {
		return ""
	}
	return term
}

// mentionLine is the first line of a body whose prose holds this term, so that
// a finding points at the sentence and not at the front matter.
func mentionLine(body, term string) int {
	for i, line := range strings.Split(body, "\n") {
		if glossary.Mentions(strings.ToLower(prose(line)), term) {
			return i + 1
		}
	}
	return 1
}

// baseGlossary reads manifests/glossary.yaml as it stood at a revision.
func baseGlossary(root, base string) (*glossary.Glossary, error) {
	cmd := exec.Command("git", "show", base+":manifests/glossary.yaml")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		// The file not being there at the base is not a fault. It is the
		// commit that introduced the glossary.
		return &glossary.Glossary{}, nil
	}
	return glossary.Parse(out, base+":manifests/glossary.yaml")
}

func needGlossaryBase(c *Corpus) string {
	if c.Opt.Base == "" {
		return "no base revision was given, so there is nothing to compare the glossary against"
	}
	if _, err := os.Stat(filepath.Join(c.Root, "manifests", "glossary.yaml")); err != nil {
		return "there is no manifests/glossary.yaml"
	}
	return ""
}

// L09. The glossary version moves when the renderings do.
//
// Every translated file records the glossary version it was written against,
// and a file is stale when that number is not the glossary's. The number is the
// whole of the staleness model for terminology, and it is a number somebody has
// to raise.
//
// glossary.Save raises it, so the tool cannot forget. A person editing the YAML
// by hand can, and did: a row was added by hand in the same session Save was
// written, and the version stayed where it was, which would have left every
// translated file reporting current against a glossary it had never seen. At
// one file that is a curiosity. At the 344 the corpus already has in English it
// is the corpus quietly disagreeing with itself.
//
// Hard, because a wrong answer here is silent everywhere else. It reports not
// run without a base revision, which is the honest state for a rule that has
// nothing to compare against, and CI gives it one.
func l09(c *Corpus) ([]Finding, error) {
	was, err := baseGlossary(c.Root, c.Opt.Base)
	if err != nil {
		return nil, err
	}
	now, err := glossary.Load(c.Root)
	if err != nil {
		return nil, err
	}
	if glossary.SameTerms(was.Terms, now.Terms) || was.Version != now.Version {
		return nil, nil
	}
	return []Finding{{File: "manifests/glossary.yaml", Line: 1,
		Msg: fmt.Sprintf("the terms changed since %s and the version is still %d, so every translated file will report current against a glossary it was not written against",
			c.Opt.Base, now.Version)}}, nil
}

// BookOf is the volume a content file belongs to, by the short book id, which
// is what a book scoped glossary row is matched against. A solution answers
// "" and that is right: nothing translates a solution against the glossary yet,
// and its front matter names the exercise rather than the volume.
//
// Exported for the same reason Prose is. The adherence report asks L06's
// question a second time, and if it scoped the glossary differently the report
// and the rule would disagree about the same file.
func BookOf(d Doc) string {
	switch {
	case d.Section != nil:
		return d.Section.Book
	case d.Exercise != nil:
		return d.Exercise.Book
	}
	return ""
}

// L17. No translation is a provider error.
//
// A refusal that arrives as an HTTP error, or as an empty body, is caught on
// the write path and the file is left alone. A refusal that arrives as a well
// formed answer whose text happens to be an error message is written to disk
// with full front matter, a translation_model, a translation_run and a
// source_content_sha256, and from that moment it is indistinguishable from a
// translation that worked. It survives -stale and it survives -redo-small if it
// is long enough, and nothing will ever visit it again: it is current, its
// source hash matches, and nothing marks it as suspect.
//
// Ten of them were in the corpus before anybody noticed, and they were noticed
// because somebody was reading the Vietnamese note to the reader for an
// unrelated reason. Nine were nothing but the sentence "Unusual activity has
// been detected from your device. Try again later." and a trace id; the tenth
// had it as one paragraph of fifty eight. So the file has to be read rather
// than its front matter, and a paragraph counts as much as a whole body.
//
// textguard already holds the phrases, because the same answers reach the
// reading side and are refused there. What is checked here is the three kinds
// that are the provider talking instead of the model working -- the anti abuse
// gateway, a model declining, and a model saying the attachment never arrived
// -- together with a body that is empty, since the same fault can produce one.
// The kinds textguard has for a model narrating its work are deliberately not
// checked: they are phrases that a translation of mathematical prose can
// legitimately contain, and this rule is hard.
//
// tamnd/bourbaki-solver#471.
func l17(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Lang == "" || d.Lang == "en" {
			continue
		}
		for _, leak := range textguard.Check(d.Body) {
			if !providerError[leak.Kind] {
				continue
			}
			line := 1
			if leak.Line > 0 {
				line = d.BodyLine(leak.Line)
			}
			out = append(out, Finding{File: d.Path, Line: line, Msg: fmt.Sprintf(
				"the translation is the provider talking and not a translation (%s): %s",
				leak.Kind, leak.Detail)})
		}
	}
	return out, nil
}

// providerError is the textguard kinds that mean no translation was made.
var providerError = map[string]bool{
	"gateway":  true,
	"refusal":  true,
	"no-image": true,
	"empty":    true,
}
