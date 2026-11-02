package gtranslate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tamnd/bourbaki-solver/glossary"
)

// Version is the version of everything in this package that shapes a draft:
// what is masked, what is pinned, what is corrected afterwards. It goes into
// the prompt hash a draft records, so a change here restales the drafts it
// would have written differently and nothing else.
const Version = "gtranslate-13"

// PromptSHA256 is what a Google draft records as its prompt_sha256. A model
// run has its own and so counts every draft stale, which is the point.
func PromptSHA256(source, target string) string {
	sum := sha256.Sum256([]byte(Version + "\x00" + source + "\x00" + target))
	return hex.EncodeToString(sum[:])
}

// Drafter drafts one language from another against a glossary.
type Drafter struct {
	Client         *Client
	Glossary       *glossary.Glossary
	Source, Target string
}

// code is the language code Google knows a corpus language by.
func code(lang string) string {
	switch lang {
	case "zh":
		return "zh-CN"
	case "en-mt":
		return "en"
	}
	return lang
}

// articleless are the targets that do not write articles.
var articleless = map[string]bool{"vi": true, "zh": true, "ja": true}

// ErrLost is what a passage fails with when Google's answer lost or doubled
// one of the pieces it was asked to keep.
var ErrLost = errors.New("google lost a protected piece")

// Draft translates one passage and says how many of its sentences it left in
// the source language.
//
// A passage whose protected pieces do not all come back is asked again block
// by block, which is where Google loses a token: a long paragraph it decides to
// restructure. A block that still fails is asked a sentence at a time, and a
// sentence that fails even alone, where Google doubles a formula or drops one
// however it is asked, is left as the source wrote it. An untranslated
// sentence is a worse page than a translated one, and a page with a formula
// missing is worse than both; the caller decides how many it will take.
func (d Drafter) Draft(ctx context.Context, body string) (string, int, error) {
	body = quietHeadings(body)
	pins, loose := Terms(d.Glossary, d.Source, d.Target, body)
	learned, err := d.Client.Learn(ctx, code(d.Source), code(d.Target), loose)
	if err != nil {
		return "", 0, err
	}
	left := 0
	out, err := d.once(ctx, body, pins)
	if errors.Is(err, ErrLost) {
		var parts []string
		for _, block := range strings.Split(body, "\n\n") {
			got, berr := d.once(ctx, block, pins)
			if errors.Is(berr, ErrLost) {
				var n int
				got, n, berr = d.sentences(ctx, block, pins)
				left += n
			}
			if berr != nil {
				return "", 0, berr
			}
			parts = append(parts, got)
		}
		out, err = strings.Join(parts, "\n\n"), nil
	}
	if err != nil {
		return "", 0, err
	}
	return Fix(tidy(body, out), loose, learned), left, nil
}

var (
	headingLine = regexp.MustCompile(`(?m)^(#{1,6}[ \t]+)([^\n]+)$`)
	capsWord    = regexp.MustCompile(`[A-Za-z]+`)
	romanWord   = regexp.MustCompile(`^[IVXLC]+$`)
)

// quietHeadings writes a heading the printing set in capitals in lower case
// with a capital first letter. Japanese Google leaves a word in capitals as it
// is, a name it does not know, and "### 1. DIAGRAMS" came back in English. A
// roman numeral keeps its capitals, and so does a heading with a formula in
// it, whose letters are not words.
func quietHeadings(body string) string {
	return headingLine.ReplaceAllStringFunc(body, func(line string) string {
		sub := headingLine.FindStringSubmatch(line)
		text := sub[2]
		if strings.Contains(text, "$") || text != strings.ToUpper(text) || !capsWord.MatchString(text) {
			return line
		}
		var b strings.Builder
		at, first := 0, true
		for _, loc := range capsWord.FindAllStringIndex(text, -1) {
			gap, w := text[at:loc[0]], text[loc[0]:loc[1]]
			first = first || strings.Contains(gap, ".")
			b.WriteString(gap)
			switch {
			case romanWord.MatchString(w) && (len(w) > 1 || !first):
			case len(w) == 1 && w != "A":
			case first:
				w = w[:1] + strings.ToLower(w[1:])
			default:
				w = strings.ToLower(w)
			}
			b.WriteString(w)
			at, first = loc[1], false
		}
		b.WriteString(text[at:])
		text = b.String()
		return sub[1] + text
	})
}

// sentenceEnd is the space after the stop that ends a sentence and the capital,
// bold or link that opens the next. "Prop. 2" and "cf. $x$" are followed by
// neither, so they are not cut.
var sentenceEnd = regexp.MustCompile("([.;?!])[ \t]+([A-Z*\\[])")

var abbrevBefore = regexp.MustCompile(`[A-Za-z]+\.$`)

// abbreviations are the words the corpus abbreviates with a stop, which never
// end a sentence.
var abbreviations = map[string]bool{
	"p": true, "pp": true, "no": true, "nos": true, "prop": true, "cor": true, "th": true,
	"def": true, "lem": true, "rem": true, "ex": true, "cf": true, "vol": true, "chap": true,
	"resp": true, "e.g": true, "i.e": true, "etc": true, "loc": true, "cit": true, "op": true, "ibid": true,
}

// sentences drafts a block one sentence at a time, and leaves a sentence Google
// cannot give back whole in the source language.
func (d Drafter) sentences(ctx context.Context, block string, pins []Pin) (string, int, error) {
	var pieces []string
	at := 0
	for _, loc := range sentenceEnd.FindAllStringSubmatchIndex(block, -1) {
		// A stop inside a formula, "\text{resp. A}", is not the end of a
		// sentence, and a cut there would send half a formula to Google.
		if strings.Count(block[:loc[2]], "$")%2 == 1 {
			continue
		}
		// Nor is the stop of an abbreviation: "VI, p. *17*" cut after the p.
		// sends the page number to Google without its p., and it came back as
		// "p. 1".
		if w := abbrevBefore.FindString(block[:loc[3]]); w != "" && abbreviations[strings.ToLower(strings.TrimSuffix(w, "."))] {
			continue
		}
		pieces = append(pieces, block[at:loc[3]])
		at = loc[4]
	}
	pieces = append(pieces, block[at:])
	var b strings.Builder
	left := 0
	for i, s := range pieces {
		got, err := d.once(ctx, s, pins)
		if errors.Is(err, ErrLost) {
			got, err = s, nil
			left++
		}
		if err != nil {
			return "", 0, err
		}
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(got)
	}
	return b.String(), left, nil
}

// linkRE is a Markdown link, its text and its target.
var linkRE = regexp.MustCompile(`(!?)\[([^\]\n]+)\]\(([^)\s]*)\)`)

func (d Drafter) once(ctx context.Context, body string, pins []Pin) (string, error) {
	if strings.TrimSpace(body) == "" {
		return body, nil
	}
	// A link's text is translated on its own and the link then kept whole.
	// Google reads the bracket that opens it as prose and moves it, and
	// "[Exercises § 1](exercises/s1/)" came back as "§ 1](exercises/s1/)参见[习题".
	var lerr error
	body = linkRE.ReplaceAllStringFunc(body, func(l string) string {
		sub := linkRE.FindStringSubmatch(l)
		if lerr != nil || strings.HasPrefix(sub[2], "^") {
			return l
		}
		text, err := d.once(ctx, sub[2], pins)
		if errors.Is(err, ErrLost) {
			return l
		}
		if err != nil {
			lerr = err
			return l
		}
		return sub[1] + "[" + strings.TrimSpace(text) + "](" + sub[3] + ")"
	})
	if lerr != nil {
		return "", lerr
	}
	m := Mask(body, pins, d.Source == "en" && articleless[d.Target])
	// Nothing left to translate: a display formula on its own, a label.
	if strings.TrimSpace(tokenRE.ReplaceAllString(m.Text, "")) == "" {
		return body, nil
	}
	got, err := d.Client.Translate(ctx, code(d.Source), code(d.Target), m.Text)
	if err != nil {
		return "", err
	}
	back, err := m.Unmask(got)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrLost, err)
	}
	return back, nil
}

var (
	spacedBold = regexp.MustCompile(`\*\*[ \t]+([^*\n]+?)[ \t]+\*\*`)
	spacedLink = regexp.MustCompile(`[ \t]+\]\(`)
	spacedHead = regexp.MustCompile(`(?m)^(#{1,6})([^#\s])`)
	wideHead   = regexp.MustCompile(`(?m)^＃{1,6}`)
)

// tidy undoes what Google does to Markdown it does not know is Markdown. Each
// repair is made only where the source did not have the same thing, so a
// source that really writes "** x **" keeps it.
func tidy(src, out string) string {
	if !spacedBold.MatchString(src) {
		out = spacedBold.ReplaceAllString(out, "**$1**")
	}
	if !spacedLink.MatchString(src) {
		out = spacedLink.ReplaceAllString(out, "](")
	}
	// Chinese Google writes a heading's number signs full width, "＃＃ 介绍".
	out = wideHead.ReplaceAllStringFunc(out, func(h string) string {
		return strings.Repeat("#", utf8.RuneCountInString(h))
	})
	if !spacedHead.MatchString(src) {
		out = spacedHead.ReplaceAllString(out, "$1 $2")
	}
	// Google drops the trailing newline and sometimes the leading one.
	if strings.HasSuffix(src, "\n") && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}
