package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A translation of Bourbaki is a translation of the prose and a copy of the
// mathematics. The formulae, the tags, the headings and the statement counts
// all have to come through untouched, and a model asked to translate a page
// will quietly renumber a proposition or drop a display if nothing is watching.
// These rules are what watches.
//
// Nothing in the corpus is translated yet, so every one of them reports not
// run. That is the honest state and it is written into the report rather than
// left as a row of passes: a rule with nothing to look at has not passed.
//
// The numbers in L07 are the one place here that is not measured, because there
// is no translation to measure against. They are marked as such where they are
// used and M7 has to set them from the first real run.

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
// Bourbaki's vocabulary is fixed and a translation that renders "anneau
// artinien" three ways across one § is a translation nobody can search. The
// glossary is the agreed rendering per term per language; this counts the terms
// that appear and the ones rendered as the glossary says.
//
// Soft, and not run: there is no glossary yet, and there is nothing to hold to
// one. M7 builds both.
func l06(c *Corpus) ([]Finding, error) { return nil, nil }

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
// The twelve-word floor below is the one number in this package that has not
// been measured, because there is no translation to measure it against. It is
// there to keep short fragments, a heading in a table or a bare formula caption,
// out of the count. M7 has to set it from the first real run and say what it
// set it from.
func l07(c *Corpus) ([]Finding, error) {
	ps, out := c.pairs()
	for _, p := range ps {
		for i, para := range paragraphs(p.tr.Body) {
			words := len(strings.Fields(para.text))
			if words < 12 {
				continue
			}
			if translatedInto(p.tr.Lang, para.text) {
				continue
			}
			out = append(out, Finding{File: p.tr.Path, Line: p.tr.BodyLine(para.line),
				Msg: fmt.Sprintf("paragraph %d is %d words with nothing of %s in it: %s",
					i+1, words, p.tr.Lang, ellipsis(para.text, 50))})
		}
	}
	return out, nil
}

// translatedInto asks whether this text carries the script of the language.
func translatedInto(lang, text string) bool {
	switch lang {
	case "zh", "ja":
		for _, r := range text {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
				return true
			}
		}
		return false
	case "vi":
		for _, r := range text {
			if r > unicode.MaxASCII && unicode.IsLetter(r) {
				return true
			}
		}
		return false
	}
	// A language nobody has written a test for is not failed for it.
	return true
}

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
