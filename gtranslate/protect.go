package gtranslate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// A Pin is a term Google is not trusted with: From as the source spells it,
// To as the glossary writes it in the target language.
type Pin struct {
	From, To string
}

// Masked is a passage with everything Google must not touch taken out and a
// token left in its place.
type Masked struct {
	Text  string
	keep  []string
	pins  map[int]bool // tokens that stand for a word of prose and not for markup
	cites map[int]bool // tokens that stand for a citation address
	upper map[int]bool // pin tokens whose word opened a sentence
}

// Token is how the n-th kept piece is written into the text sent to Google.
// The brackets are the mathematical white square brackets: Google copies them
// through untouched, and the corpus has none of its own.
func Token(n int) string { return fmt.Sprintf("⟦%d⟧", n) }

var tokenRE = regexp.MustCompile(`⟦\s*(\d+)\s*⟧`)

// The markup that has to come back byte for byte. Order matters: a link target
// can hold an attribute brace, so links go before attributes.
var markup = []*regexp.Regexp{
	regexp.MustCompile("(?s)```.*?```"),             // fenced code
	regexp.MustCompile("`[^`\n]+`"),                 // inline code
	regexp.MustCompile(`<!--(?s:.*?)-->`),           // comments
	regexp.MustCompile(`</?[A-Za-z][^>\n]*>`),       // html tags
	regexp.MustCompile(`\[\^[^\]\s]+\]:?`),          // footnote markers and definitions
	regexp.MustCompile(`\]\([^)\s]*\)`),             // link targets, with the bracket that closes the text
	regexp.MustCompile(`\{[#.][^}\n]*\}`),           // pandoc attributes
	regexp.MustCompile(`(?m)^\s*(?:[-*_]\s*){3,}$`), // rules
	citation,
	// A roman numeral standing as a word: "Chapter I", "Appendix I", the
	// label of "**I. Localization.**", and the set the English writes without
	// dollars in "let I be a set". Google reads a lone I as the pronoun and
	// gives back 我.
	regexp.MustCompile(`\b[IVXLC]+\b`),
	// Labels written as plain text, alg-ii-s1-def-1 and the like.
	regexp.MustCompile(`\b[a-z]{2,4}-[ivxlc0-9]+-[a-z0-9-]+\b`),
}

// citation is a citation address, which stays as the English writes it in
// every language.
var citation = regexp.MustCompile(`\b[IVXLC]+,\s*§§?\s*\d+(?:\s*,?\s*[Nn]o\.\s*\d+)?|§§?\s*\d+(?:\s*,?\s*[Nn]o\.\s*\d+)?|\b[Nn]o\.\s*\d+|\bp\.\s*\d+|\bcf\.`)

// Mask takes the mathematics, the markup and the pinned terms out of body.
//
// articles is whether an English article in front of a pinned term goes too.
// A target that has no articles, Vietnamese, Chinese or Japanese, is better
// off without it: Google leaves an "a" in front of a token it cannot read as a
// noun, and that "a" comes back untranslated.
func Mask(body string, pins []Pin, articles bool) Masked {
	m := Masked{pins: map[int]bool{}, upper: map[int]bool{}, cites: map[int]bool{}}
	put := func(s string) string {
		m.keep = append(m.keep, s)
		return Token(len(m.keep) - 1)
	}

	// Mathematics first, by the corpus's own reading of where it is, so that a
	// dollar inside a code span or a brace inside a formula is never cut.
	spans, unclosed := mathtex.Split(body)
	if unclosed == nil && len(spans) > 0 {
		rs := []rune(body)
		var b strings.Builder
		at := 0
		for _, s := range spans {
			delim := 1
			if s.Display {
				delim = 2
			}
			open, end := s.Start-delim, s.End+delim
			if open < at || end > len(rs) {
				continue
			}
			b.WriteString(string(rs[at:open]))
			b.WriteString(put(string(rs[open:end])))
			at = end
		}
		b.WriteString(string(rs[at:]))
		body = b.String()
	}

	for _, re := range markup {
		f := put
		if re == citation {
			f = func(s string) string {
				t := put(s)
				m.cites[len(m.keep)-1] = true
				return t
			}
		}
		body = re.ReplaceAllStringFunc(body, f)
	}

	if articles {
		// An article at the start of an emphasis is one Google keeps: it reads
		// "*a law of action*" as a name and gives back "*a luật tác động*".
		body = emphasisArticle.ReplaceAllString(body, "$1")
	}
	body = m.pin(body, pins, articles, put)
	m.Text = body
	return m
}

var emphasisArticle = regexp.MustCompile(`((?:^|[\s(])(?:\*{1,2}|_))(?:[Aa]n?|[Tt]he)\s+`)

var articleRE = regexp.MustCompile(`(?:^|[\s(])((?:[Aa]n?|[Tt]he)\s+)$`)

func (m *Masked) pin(body string, pins []Pin, articles bool, put func(string) string) string {
	if len(pins) == 0 {
		return body
	}
	sorted := append([]Pin(nil), pins...)
	sort.SliceStable(sorted, func(i, j int) bool { return len(sorted[i].From) > len(sorted[j].From) })
	var alts []string
	to := map[string]string{}
	for _, p := range sorted {
		k := strings.ToLower(strings.Join(strings.Fields(p.From), " "))
		if k == "" || to[k] != "" {
			continue
		}
		to[k] = p.To
		alts = append(alts, strings.ReplaceAll(regexp.QuoteMeta(k), " ", `\s+`))
	}
	re := regexp.MustCompile(`(?i)(` + strings.Join(alts, "|") + `)(s|es|x)?`)

	var b strings.Builder
	at := 0
	for _, loc := range re.FindAllStringSubmatchIndex(body, -1) {
		start, end := loc[0], loc[1]
		if start < at || !edge(body, start, end) {
			continue
		}
		word := body[loc[2]:loc[3]]
		rendering := to[strings.ToLower(strings.Join(strings.Fields(word), " "))]
		if rendering == "" {
			continue
		}
		before := body[at:start]
		upper := startsUpper(word)
		if articles {
			if a := articleRE.FindStringSubmatchIndex(before); a != nil {
				upper = upper || startsUpper(before[a[2]:a[3]])
				before = before[:a[2]]
			}
		}
		// A capital on a common noun means the word opened a sentence or a
		// heading, and the rendering opens it too.
		upper = upper && sentenceStart(before)
		b.WriteString(before)
		tok := put(rendering)
		n := len(m.keep) - 1
		m.pins[n] = true
		m.upper[n] = upper
		b.WriteString(tok)
		at = end
	}
	b.WriteString(body[at:])
	return b.String()
}

// edge says the match is a whole word: no letter on either side of it.
func edge(s string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:start])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			return false
		}
	}
	if end < len(s) {
		r, _ := utf8.DecodeRuneInString(s[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			return false
		}
	}
	return true
}

func startsUpper(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

// sentenceStart says the text in front of a word ends where a sentence or a
// heading begins.
func sentenceStart(before string) bool {
	t := strings.TrimRight(before, " \t")
	if t == "" || strings.HasSuffix(t, "\n") {
		return true
	}
	t = strings.TrimRight(t, "*_")
	if t == "" {
		return true
	}
	switch t[len(t)-1] {
	case '.', '!', '?', ':', '#', '\n':
		return true
	}
	return false
}

// Unmask puts back what Mask took out. It fails when Google dropped or
// doubled a token, because a lost token is a lost formula and there is no
// putting that back from the answer alone.
func (m Masked) Unmask(answer string) (string, error) {
	seen := make([]int, len(m.keep))
	var bad error
	out := tokenRE.ReplaceAllStringFunc(answer, func(t string) string {
		sub := tokenRE.FindStringSubmatch(t)
		var n int
		fmt.Sscanf(sub[1], "%d", &n)
		if n < 0 || n >= len(m.keep) {
			bad = fmt.Errorf("the answer holds a token %s that was never sent", t)
			return t
		}
		seen[n]++
		s := m.keep[n]
		if m.upper[n] {
			s = upperFirst(s)
		}
		if m.pins[n] {
			// A pin is a word, and Google sometimes glues a token to the word
			// beside it. The space goes back where a letter touches it.
			return "\x00" + s + "\x01"
		}
		if m.cites[n] {
			// Google glues a token to what is in front of it, and "推论2§ 1" or
			// "§ 2no. 5" is no longer read as a citation by anyone.
			return "\x02" + s
		}
		return s
	})
	if bad != nil {
		return "", bad
	}
	var lost, doubled []string
	for n, c := range seen {
		switch {
		case c == 0:
			lost = append(lost, short(m.keep[n]))
		case c > 1 && !m.pins[n]:
			// A pinned term said twice is clumsy prose, which a draft is
			// allowed; a formula said twice is not mathematics the source has.
			doubled = append(doubled, short(m.keep[n]))
		}
	}
	if len(lost) > 0 {
		return "", fmt.Errorf("google dropped %d protected pieces: %s", len(lost), strings.Join(lost, ", "))
	}
	if len(doubled) > 0 {
		return "", fmt.Errorf("google repeated %d protected pieces: %s", len(doubled), strings.Join(doubled, ", "))
	}
	return respaceWords(out), nil
}

func respaceWords(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch s[i] {
		case 0:
			i++
			next, _ := utf8.DecodeRuneInString(s[i:])
			prev, _ := utf8.DecodeLastRuneInString(b.String())
			switch {
			case cjk(next) && prev == ' ':
				// Google pads a token with spaces, and Chinese and Japanese
				// write none: "根据 群 与" is "根据群与".
				t := strings.TrimRight(b.String(), " ")
				if r, _ := utf8.DecodeLastRuneInString(t); cjk(r) {
					b.Reset()
					b.WriteString(t)
				}
			case !cjk(next) && (unicode.IsLetter(prev) || unicode.IsDigit(prev)) && !cjk(prev):
				b.WriteByte(' ')
			}
		case 1:
			i++
			last, _ := utf8.DecodeLastRuneInString(b.String())
			rest := strings.TrimLeft(s[i:], " ")
			next, _ := utf8.DecodeRuneInString(rest)
			switch {
			case cjk(last) && cjk(next):
				i = len(s) - len(rest)
			case !cjk(last) && !cjk(next) && i < len(s) && (unicode.IsLetter(next) || unicode.IsDigit(next)) && rest == s[i:]:
				b.WriteByte(' ')
			}
		case 2:
			i++
			if prev, _ := utf8.DecodeLastRuneInString(b.String()); prev < utf8.RuneSelf && (unicode.IsLetter(prev) || unicode.IsDigit(prev)) {
				b.WriteByte(' ')
			}
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// cjk says r is written without spaces around it.
func cjk(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
