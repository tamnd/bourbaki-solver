package publish

import (
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The cross-index: every statement of a § page says what it cites and what
// cites it, a citation in the prose is a link to what it names, and each
// volume has an index of its statements by kind. It is all read off the
// reference graph by label, so it reaches the volumes that have no tags yet,
// and it is said in the language of the page, with English standing in for a
// statement a translation has not reached.

// kinds is the order the statement index lists the kinds in.
var kinds = []string{"definition", "theorem", "proposition", "lemma", "corollary", "example", "remark", "other"}

var kindNames = map[string]map[string]string{
	"en": {"definition": "Definitions", "theorem": "Theorems", "proposition": "Propositions", "lemma": "Lemmas",
		"corollary": "Corollaries", "example": "Examples", "remark": "Remarks", "other": "Other"},
	"fr": {"definition": "Définitions", "theorem": "Théorèmes", "proposition": "Propositions", "lemma": "Lemmes",
		"corollary": "Corollaires", "example": "Exemples", "remark": "Remarques", "other": "Autres"},
	"vi": {"definition": "Định nghĩa", "theorem": "Định lý", "proposition": "Mệnh đề", "lemma": "Bổ đề",
		"corollary": "Hệ quả", "example": "Ví dụ", "remark": "Nhận xét", "other": "Khác"},
}

func kindName(lang, kind string) string { return kindNames[uiLang(lang)][kind] }

// target is where a label lands from a page in lang: the URL, the name it is
// shown under, and the language of the page it lands on, which is lang unless
// the translation has not reached it.
type target struct {
	URL, Name, Lang string
	Sec             *Section
}

// anchor finds a label from a page in lang. here is the § the link is written
// on, so a statement of the same § is a link down the page rather than a
// reload, and so a statement of another Book is named with its Book.
func (s *Site) anchor(lang, label string, here *Section) (target, bool) {
	if at, ok := s.unitIn(lang, label); ok {
		u := s.SectionURL(at.sec)
		if here != nil && at.sec == here {
			u = ""
		}
		return target{URL: u + "#" + label, Name: s.place(at.unit.Heading.Text, at.sec, here), Lang: at.sec.Lang, Sec: at.sec}, true
	}
	if ex := s.exByLabel(lang, label); ex != nil {
		return target{URL: s.ExerciseURL(ex), Name: s.place(ex.Name(), nil, here), Lang: ex.Lang}, true
	}
	if sec := s.sectionByLabel(lang, label); sec != nil {
		// A § is named by its title after where it stands, which already
		// carries its number: I § 7. Free monoids, not § 7. Free monoids, I § 7.
		name := navName(sec)
		if where := strings.TrimSuffix(strings.TrimPrefix(s.place("", sec, here), ", "), short(sec)); where != "" {
			name = where + name
		}
		return target{URL: s.SectionURL(sec), Name: name, Lang: sec.Lang, Sec: sec}, true
	}
	return target{}, false
}

// place is a name with as much of where it is as a reader on here needs:
// nothing more inside the same §, the chapter and § inside the same Book, and
// the Book as well beyond it.
func (s *Site) place(name string, sec, here *Section) string {
	if sec == nil || (here != nil && sec == here) {
		return name
	}
	where := sec.Meta.Chapter + " " + short(sec)
	if sec.Meta.Chapter == "" {
		where = short(sec)
	}
	if here == nil || here.Meta.Book != sec.Meta.Book {
		where = infoOf(sec.Meta.Book).Abbr + " " + where
	}
	return name + ", " + strings.TrimSpace(where)
}

func (s *Site) unitIn(lang, label string) (unitAt, bool) {
	m := s.units[label]
	if m == nil {
		return unitAt{}, false
	}
	if at, ok := m[lang]; ok {
		return at, true
	}
	if at, ok := m["en"]; ok {
		return at, true
	}
	for _, l := range s.Langs {
		if at, ok := m[l]; ok {
			return at, true
		}
	}
	return unitAt{}, false
}

func (s *Site) exByLabel(lang, label string) *Exercise {
	if ex := s.exIn[lang+"\x00"+label]; ex != nil {
		return ex
	}
	return s.byExLabel[label]
}

func (s *Site) sectionByLabel(lang, label string) *Section {
	if sec := s.secIn[lang+"\x00"+label]; sec != nil {
		return sec
	}
	return s.byLabel[label]
}

// xrefShown is how many rows a card lists of each direction before it says
// how many more there are. A definition at the foot of a Book is cited by
// hundreds of statements, and a card is not the place for all of them.
const xrefShown = 24

// xref is the foot of a statement card: what it cites and what cites it.
func (s *Site) xref(here *Section, label string) string {
	uses, usedBy := s.uses[label], s.usedBy[label]
	if len(uses) == 0 && len(usedBy) == 0 {
		return ""
	}
	lang := here.Lang
	var b strings.Builder
	b.WriteString(`<details class="xref"><summary>`)
	var parts []string
	if len(uses) > 0 {
		parts = append(parts, fmt.Sprintf("%s <b>%d</b>", esc(T(lang, "x.cites")), len(uses)))
	}
	if len(usedBy) > 0 {
		parts = append(parts, fmt.Sprintf("%s <b>%d</b>", esc(T(lang, "x.citedby")), len(usedBy)))
	}
	b.WriteString(strings.Join(parts, " · "))
	b.WriteString("</summary>")
	row := func(key string, edges []*Edge, end func(*Edge) string) {
		if len(edges) == 0 {
			return
		}
		fmt.Fprintf(&b, `<p><span class="k">%s</span> `, esc(T(lang, key)))
		var links []string
		for i, e := range edges {
			if i == xrefShown {
				links = append(links, fmt.Sprintf(`<span class="more">+%d</span>`, len(edges)-xrefShown))
				break
			}
			l := end(e)
			t, ok := s.anchor(lang, l, here)
			if !ok {
				links = append(links, fmt.Sprintf(`<span class="out" title="%s">%s</span>`, esc(missing(l)), esc(e.Raw)))
				continue
			}
			links = append(links, fmt.Sprintf(`<a href="%s" class="ref"%s>%s</a>`, esc(t.URL), langAttr(t.Lang, lang), esc(t.Name)))
		}
		b.WriteString(strings.Join(links, ", "))
		b.WriteString("</p>")
	}
	row("x.cites", uses, func(e *Edge) string { return e.ToLabel })
	row("x.citedby", usedBy, func(e *Edge) string { return e.FromLabel })
	b.WriteString("</details>\n")
	return b.String()
}

// langAttr marks a link that lands in another language than the page's, which
// is what a reader of a translation gets for a statement not yet translated.
func langAttr(to, from string) string {
	if to == from || to == "" {
		return ""
	}
	return fmt.Sprintf(` hreflang="%s" data-lang="%s"`, to, to)
}

// linked is a § body with every citation the graph resolved made a link to what
// it names. The graph is read off the English, so it is the English page that
// carries them; every language has the same statements on its cards.
//
// The edge says the file line and the words; the words are looked for on that
// line and the lines either side, outside the mathematics and outside a link
// already written, and an edge whose words cannot be found is left as prose.
func (s *Site) linked(sec *Section) string {
	edges := s.citeAt[sec.Path]
	if len(edges) == 0 {
		return sec.Body
	}
	lines := strings.Split(sec.Body, "\n")
	math := displayLines(lines)
	done := map[string]bool{}
	for _, e := range edges {
		if e.ToLabel == "" || e.How == "out-of-corpus" || e.Raw == "" {
			continue
		}
		// The § a sentence stands in is where the reader already is.
		if e.ToLabel == sectionLabel(sec) {
			continue
		}
		key := fmt.Sprintf("%d\x00%s", e.Line, e.Raw)
		if done[key] {
			continue
		}
		t, ok := s.anchor(sec.Lang, e.ToLabel, sec)
		if !ok || strings.ContainsAny(t.URL, " ()") {
			continue
		}
		at := e.Line - sec.Head
		for _, i := range []int{at, at - 1, at + 1, at - 2} {
			if i < 0 || i >= len(lines) || math[i] || strings.HasPrefix(lines[i], "#") {
				continue
			}
			if l, ok := linkIn(lines[i], e.Raw, t.URL); ok {
				lines[i] = l
				done[key] = true
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// displayLines marks the lines a link must stay out of: those of a display
// formula, which runs over several lines between $$ and $$, and those of a
// fenced block. A link there is TeX KaTeX refuses, not a link.
func displayLines(lines []string) []bool {
	out := make([]bool, len(lines))
	open, fence := false, false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = !fence
			out[i] = true
			continue
		}
		n := strings.Count(l, "$$")
		out[i] = fence || open || n > 0 || strings.Contains(l, `\begin{`) || strings.Contains(l, `\[`)
		if n%2 == 1 {
			open = !open
		}
	}
	return out
}

// linkIn writes the first free occurrence of raw on line as a link to url.
func linkIn(line, raw, url string) (string, bool) {
	if strings.ContainsAny(raw, "[]$") {
		return line, false
	}
	from := 0
	for {
		i := strings.Index(line[from:], raw)
		if i < 0 {
			return line, false
		}
		i += from
		pre := line[:i]
		free := strings.Count(pre, "$")%2 == 0 && strings.Count(pre, "[") == strings.Count(pre, "]") &&
			!strings.HasSuffix(pre, "](")
		if free {
			return pre + "[" + raw + "](" + url + ")" + line[i+len(raw):], true
		}
		from = i + len(raw)
	}
}

// readable is a stretch of Markdown as a line of text, with the commoner TeX
// written as the characters it sets. It is for the statement index, where a
// formula set properly would cost more than the rest of the row together.
func readable(md string, max int) string {
	var b strings.Builder
	last := 0
	for _, m := range mathRE.FindAllStringSubmatchIndex(md, -1) {
		b.WriteString(prose(md[last:m[0]]))
		if m[2] >= 0 {
			b.WriteString(" " + texText(md[m[2]:m[3]]) + " ")
		} else if m[4] >= 0 {
			b.WriteString(texText(md[m[4]:m[5]]))
		}
		last = m[1]
	}
	b.WriteString(prose(md[last:]))
	t := strings.TrimSpace(spaceRE.ReplaceAllString(b.String(), " "))
	t = strings.Trim(t, "*_ ")
	if utf8.RuneCountInString(t) > max {
		r := []rune(t)[:max]
		t = strings.TrimRight(string(r), " ,.;:") + "…"
	}
	return t
}

var texSym = map[string]string{
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "varepsilon": "ε", "zeta": "ζ", "eta": "η",
	"theta": "θ", "vartheta": "ϑ", "iota": "ι", "kappa": "κ", "lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ", "pi": "π",
	"rho": "ρ", "sigma": "σ", "tau": "τ", "upsilon": "υ", "phi": "φ", "varphi": "φ", "chi": "χ", "psi": "ψ", "omega": "ω",
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ", "Pi": "Π", "Sigma": "Σ", "Phi": "Φ", "Psi": "Ψ",
	"Omega": "Ω", "in": "∈", "notin": "∉", "ni": "∋", "subset": "⊂", "supset": "⊃", "subseteq": "⊆", "supseteq": "⊇",
	"cap": "∩", "cup": "∪", "bigcap": "⋂", "bigcup": "⋃", "to": "→", "rightarrow": "→", "leftarrow": "←",
	"mapsto": "↦", "Rightarrow": "⇒", "Leftrightarrow": "⇔", "iff": "⇔", "implies": "⇒", "times": "×", "cdot": "·",
	"circ": "∘", "otimes": "⊗", "oplus": "⊕", "bigotimes": "⨂", "bigoplus": "⨁", "leq": "≤", "le": "≤", "geq": "≥",
	"ge": "≥", "neq": "≠", "ne": "≠", "sim": "∼", "simeq": "≃", "cong": "≅", "approx": "≈", "equiv": "≡", "infty": "∞",
	"sum": "∑", "prod": "∏", "int": "∫", "partial": "∂", "nabla": "∇", "forall": "∀", "exists": "∃", "neg": "¬",
	"wedge": "∧", "vee": "∨", "emptyset": "∅", "varnothing": "∅", "setminus": "∖", "pm": "±", "mid": "|",
	"ldots": "…", "cdots": "⋯", "dots": "…", "star": "⋆", "ast": "∗", "langle": "⟨", "rangle": "⟩", "ell": "ℓ",
	"prime": "′", "perp": "⊥", "lbrace": "{", "rbrace": "}", "footnote": "",
}

var (
	texBB    = regexp.MustCompile(`\\mathbb\{([A-Za-z]+)\}`)
	texFrak  = regexp.MustCompile(`\\mathfrak\{([A-Za-z]+)\}`)
	texDrop  = regexp.MustCompile(`\\(?:left|right|big|Big|bigg|Bigg|math(?:rm|bf|it|sf|tt|cal|scr)|operatorname|text(?:rm|it|bf)?|boldsymbol|rm|bf|it|displaystyle)\b`)
	texSpace = regexp.MustCompile(`\\[,;:!]|\\q?quad\b`)
	texCmd   = regexp.MustCompile(`\\([A-Za-z]+)`)
	texFrac  = regexp.MustCompile(`\\frac\s*\{([^{}]*)\}\s*\{([^{}]*)\}`)
	// texScript is a sub- or superscript of one character or a short group
	// with no command in it, the only kind with a Unicode spelling.
	texScript = regexp.MustCompile(`[_^](?:\{[0-9A-Za-z+\-=()]{1,4}\}|[0-9A-Za-z+\-])`)
	texEsc    = regexp.MustCompile(`\\([{}|_%&#$])`)
)

var (
	subs = runeMap("0123456789+-=()aehijklmnoprstuvx", "₀₁₂₃₄₅₆₇₈₉₊₋₌₍₎ₐₑₕᵢⱼₖₗₘₙₒₚᵣₛₜᵤᵥₓ")
	sups = runeMap("0123456789+-=()abcdefghijklmnoprstuvwxyzABDEGHIJKLMNOPRTUVW",
		"⁰¹²³⁴⁵⁶⁷⁸⁹⁺⁻⁼⁽⁾ᵃᵇᶜᵈᵉᶠᵍʰⁱʲᵏˡᵐⁿᵒᵖʳˢᵗᵘᵛʷˣʸᶻᴬᴮᴰᴱᴳᴴᴵᴶᴷᴸᴹᴺᴼᴾᴿᵀᵁⱽᵂ")
)

func runeMap(from, to string) map[rune]rune {
	m := map[rune]rune{}
	t := []rune(to)
	for i, c := range []rune(from) {
		m[c] = t[i]
	}
	return m
}

// group is a fraction's numerator or denominator, bracketed when it is more
// than one thing.
func group(x string) string {
	x = strings.TrimSpace(x)
	if len([]rune(x)) <= 1 || !strings.ContainsAny(x, " +-") {
		return x
	}
	return "(" + x + ")"
}

// texText is the search page's tex(), in Go.
func texText(s string) string {
	letters := func(base rune, holes map[byte]string) func(string) string {
		return func(m string) string {
			x := m[strings.Index(m, "{")+1 : len(m)-1]
			var b strings.Builder
			for i := 0; i < len(x); i++ {
				c := x[i]
				if h, ok := holes[c]; ok {
					b.WriteString(h)
				} else if c <= 'Z' {
					b.WriteRune(base + rune(c-'A'))
				} else {
					b.WriteRune(base + 26 + rune(c-'a'))
				}
			}
			return b.String()
		}
	}
	s = texBB.ReplaceAllStringFunc(s, letters(0x1D538, map[byte]string{'C': "ℂ", 'H': "ℍ", 'N': "ℕ", 'P': "ℙ", 'Q': "ℚ", 'R': "ℝ", 'Z': "ℤ"}))
	s = texFrak.ReplaceAllStringFunc(s, letters(0x1D504, map[byte]string{'C': "ℭ", 'H': "ℌ", 'I': "ℑ", 'R': "ℜ", 'Z': "ℨ"}))
	s = texDrop.ReplaceAllString(s, "")
	for i := 0; i < 3 && texFrac.MatchString(s); i++ {
		s = texFrac.ReplaceAllStringFunc(s, func(m string) string {
			p := texFrac.FindStringSubmatch(m)
			return group(p[1]) + "/" + group(p[2])
		})
	}
	s = texScript.ReplaceAllStringFunc(s, func(m string) string {
		x := strings.Trim(m[1:], "{}")
		table := subs
		if m[0] == '^' {
			table = sups
		}
		var b strings.Builder
		for _, c := range x {
			v, ok := table[c]
			if !ok {
				return m
			}
			b.WriteRune(v)
		}
		return b.String()
	})
	s = texSpace.ReplaceAllString(s, " ")
	s = texCmd.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := texSym[m[1:]]; ok {
			return v
		}
		return m[1:]
	})
	s = texEsc.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("{", "", "}", "").Replace(s)
	return s
}

// StatementsURL is a Book's index of statements in one language.
func (s *Site) StatementsURL(lang, book string) string { return s.url(lang, book, "statements") }

// statementRow is one row of a statement index.
type statementRow struct {
	Kind, Label, Head, Tag, Snip string
	Sec                          *Section
	UsedBy                       int
}

// rowsOf is every statement of a §, in the order it prints them.
func (s *Site) rowsOf(sec *Section) []statementRow {
	var out []statementRow
	for _, u := range sec.Units {
		body := u.Body
		// The statement, not its proof: the first paragraph says what it is.
		if i := strings.Index(body, "\n\n"); i > 0 {
			body = body[:i]
		}
		out = append(out, statementRow{Kind: labelKind(u.Heading.Label), Label: u.Heading.Label,
			Head: u.Heading.Text, Tag: u.Heading.Tag, Snip: readable(body, 180), Sec: sec,
			UsedBy: len(s.usedBy[u.Heading.Label])})
	}
	return out
}

// groups is a Book's §§ under the heading the index gives each run of them.
func groups(lang string, sh *shelf) (names []string, secs [][]*Section) {
	if len(sh.Front) > 0 {
		names, secs = append(names, T(lang, "front")), append(secs, sh.Front)
	}
	for _, c := range sh.Chaps {
		names, secs = append(names, chapLabel(lang, c)), append(secs, c.Sections)
	}
	if len(sh.Back) > 0 {
		names, secs = append(names, T(lang, "back")), append(secs, sh.Back)
	}
	return names, secs
}

// rows is every statement of a Book in one language, in book order.
func (s *Site) rows(sh *shelf) []statementRow {
	var out []statementRow
	_, secs := groups(sh.Lang, sh)
	for _, g := range secs {
		for _, sec := range g {
			out = append(out, s.rowsOf(sec)...)
		}
	}
	return out
}

// statementsPage is /<lang>/<book>/statements/: every statement of the Book,
// by chapter and §, with chips that keep one kind and a box that filters.
func (s *Site) statementsPage(sh *shelf) (string, error) {
	lang := sh.Lang
	rows := s.rows(sh)
	count := map[string]int{}
	for _, r := range rows {
		count[r.Kind]++
	}
	var b strings.Builder
	info := infoOf(sh.Book)
	fmt.Fprintf(&b, "<header class=\"doc-head\"><p class=\"eyebrow\"><a href=%q>%s %s</a> · %s</p><h1>%s</h1>",
		s.BookURL(lang, sh.Book), esc(T(lang, "book")), esc(info.Abbr), esc(native(lang)), esc(T(lang, "ix.title")))
	fmt.Fprintf(&b, "<p class=\"lede\">%s</p></header>\n", esc(fmt.Sprintf(T(lang, "ix.lede"), len(rows), bookName(lang, sh.Book, sh.Book))))
	b.WriteString("<div class=\"ix-tools\" data-ix>\n")
	fmt.Fprintf(&b, "<button type=\"button\" class=\"chip on\" data-k=\"\">%s <span class=\"x\">%d</span></button>\n", esc(T(lang, "ix.all")), len(rows))
	for _, k := range kinds {
		if count[k] == 0 {
			continue
		}
		fmt.Fprintf(&b, "<button type=\"button\" class=\"chip k-%s\" data-k=\"%s\"><span class=\"dot\"></span>%s <span class=\"x\">%d</span></button>\n",
			k, k, esc(kindName(lang, k)), count[k])
	}
	fmt.Fprintf(&b, "<input type=\"search\" class=\"ix-q\" placeholder=\"%s\" aria-label=\"%s\">\n", esc(T(lang, "ix.filter")), esc(T(lang, "ix.filter")))
	fmt.Fprintf(&b, "<label class=\"ix-sort\"><input type=\"checkbox\" data-sort> %s</label>\n", esc(T(lang, "ix.cited")))
	b.WriteString("</div>\n<div class=\"ix\">\n")
	names, secs := groups(lang, sh)
	for i, g := range secs {
		head := false
		for _, sec := range g {
			rs := s.rowsOf(sec)
			if len(rs) == 0 {
				continue
			}
			if !head {
				fmt.Fprintf(&b, "<section class=\"ix-ch\"><h2>%s</h2>\n", esc(names[i]))
				head = true
			}
			fmt.Fprintf(&b, "<div class=\"ix-sec\"><h3><a href=%q>%s</a></h3>\n<ul class=\"ix-list\">\n", s.SectionURL(sec), esc(navName(sec)))
			for _, r := range rs {
				s.ixRow(&b, lang, r, "")
			}
			b.WriteString("</ul></div>\n")
		}
		if head {
			b.WriteString("</section>\n")
		}
	}
	b.WriteString("</div>\n")
	langs := s.langLinks(lang, func(l string) string {
		if s.shelfOf(l, sh.Book) == nil {
			return ""
		}
		return s.StatementsURL(l, sh.Book)
	})
	return s.render(layout{Title: T(lang, "ix.title") + " · " + bookName(lang, sh.Book, sh.Book), Lang: lang, Here: "library",
		Side: s.sideFor(lang, sh.Book), Wide: true, Langs: langs,
		Crumbs:  []crumb{{T(lang, "library"), s.url(homeLang(lang))}, {bookName(lang, sh.Book, sh.Book), s.BookURL(lang, sh.Book)}},
		Content: template.HTML(b.String())})
}

// ixRow is one statement in an index: its kind, its name, the start of what it
// says, how much cites it, and its tag.
func (s *Site) ixRow(b *strings.Builder, lang string, r statementRow, where string) {
	fmt.Fprintf(b, "<li class=\"k-%s\" data-k=\"%s\" data-n=\"%d\"><a href=\"%s#%s\"><span class=\"dot\"></span><b>%s</b></a>",
		r.Kind, r.Kind, r.UsedBy, s.SectionURL(r.Sec), r.Label, esc(r.Head))
	if where != "" {
		fmt.Fprintf(b, " <span class=\"where\">%s</span>", esc(where))
	}
	if r.Snip != "" {
		fmt.Fprintf(b, " <span class=\"snip\">%s</span>", esc(r.Snip))
	}
	if r.UsedBy > 0 {
		fmt.Fprintf(b, " <span class=\"n\" title=\"%s\">↩ %d</span>", esc(T(lang, "x.citedby")), r.UsedBy)
	}
	if r.Tag != "" {
		fmt.Fprintf(b, " <a class=\"tag\" href=%q>%s</a>", s.TagURL(r.Tag), r.Tag)
	}
	b.WriteString("</li>\n")
}

func (s *Site) secsOf(sh *shelf, chapter string) []*Section {
	for _, c := range sh.Chaps {
		if c.Name == chapter {
			return c.Sections
		}
	}
	return nil
}

// citedPage is /<lang>/statements/: every Book's index with its counts, and
// the statements of the language the rest of the treatise leans on most.
func (s *Site) citedPage(lang string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "<header class=\"doc-head\"><p class=\"eyebrow\">%s</p><h1>%s</h1><p class=\"lede\">%s</p></header>\n",
		esc(native(lang)), esc(T(lang, "ix.title")), esc(T(lang, "ix.all.lede")))
	b.WriteString("<table class=\"ix-books\"><thead><tr><th></th>")
	for _, k := range kinds[:len(kinds)-1] {
		fmt.Fprintf(&b, "<th class=\"k-%s\"><span class=\"dot\"></span>%s</th>", k, esc(kindName(lang, k)))
	}
	b.WriteString("</tr></thead><tbody>\n")
	var all []statementRow
	for _, sh := range s.books(lang) {
		rows := s.rows(sh)
		if len(rows) == 0 {
			continue
		}
		all = append(all, rows...)
		count := map[string]int{}
		for _, r := range rows {
			count[r.Kind]++
		}
		fmt.Fprintf(&b, "<tr><th><a href=%q>%s</a></th>", s.StatementsURL(lang, sh.Book), esc(bookName(lang, sh.Book, sh.Book)))
		for _, k := range kinds[:len(kinds)-1] {
			if count[k] == 0 {
				b.WriteString("<td></td>")
				continue
			}
			fmt.Fprintf(&b, "<td>%d</td>", count[k])
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody></table>\n")
	sort.SliceStable(all, func(i, j int) bool { return all[i].UsedBy > all[j].UsedBy })
	fmt.Fprintf(&b, "<h2>%s</h2>\n<ol class=\"ix-list ranked\">\n", esc(T(lang, "ix.most")))
	for i, r := range all {
		if i == 100 || r.UsedBy == 0 {
			break
		}
		s.ixRow(&b, lang, r, strings.TrimPrefix(s.place("", r.Sec, nil), ", "))
	}
	b.WriteString("</ol>\n")
	langs := s.langLinks(lang, func(l string) string { return s.url(l, "statements") })
	return s.render(layout{Title: T(lang, "ix.title") + " · Bourbaki", Lang: lang, Here: "library", Wide: true, Langs: langs,
		Crumbs:  []crumb{{T(lang, "library"), s.url(homeLang(lang))}},
		Content: template.HTML(b.String())})
}

// subsecRE is the heading of a numbered subsection, "### 3. DERIVATIVE OF A
// PRODUCT", which is what the statements of a § are grouped under.
var subsecRE = regexp.MustCompile(`^(\d+)\s*[.:]`)

// subsecID is the anchor of a numbered subsection, alg-i-s1-n3 for no. 3,
// which is the shape the labels of the statements under it already take.
func subsecID(secLabel, words string) string {
	if secLabel == "" {
		return ""
	}
	m := subsecRE.FindStringSubmatch(strings.TrimSpace(words))
	if m == nil {
		return ""
	}
	n, _ := strconv.Atoi(m[1])
	return fmt.Sprintf("%s-n%d", secLabel, n)
}
