package book

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// The corpus is machine-written Markdown over a small and known vocabulary, and
// this reads that vocabulary rather than CommonMark, for the reason publish
// gives at length: the input is full of TeX and TeX and Markdown fight over the
// same characters. `$x^*\otimes y$` and `$x^*\in E^*$` in one sentence give a
// Markdown parser two asterisks to pair up and the paragraph comes back with an
// emphasis span cut through the middle of two formulae.
//
// Going out to LaTeX rather than to HTML makes that worse and not better,
// because now the output language is TeX too, and a backslash that is prose has
// to be told from a backslash that is a command. So the order here is fixed and
// it is the only order that works: take the mathematics out first, then the
// Markdown escapes, then escape what is left as prose, then read the Markdown,
// then put the escapes back, then put the mathematics back. Anything that reads
// the text before the mathematics is out is reading a string that has TeX in it
// and does not know which parts.

// A Renderer turns one body of corpus Markdown into LaTeX.
type Renderer struct {
	// File and Line say where the body came from, the path as committed and the
	// file line the body's first line sits on, so that a complaint names
	// something somebody can open.
	File string
	Line int
	// Label is asked what to label a heading anchor, and returns empty for an
	// anchor the book does not label. It returns the name and not the \label
	// command, because the class is what decides where a label goes relative to
	// the heading it belongs to, and that decision belongs in one place.
	Label func(anchor string) string
	// Ref is asked what a link in the body should become. It is given the URL as
	// the corpus writes it and returns LaTeX for the whole link, or empty to
	// have the link set as its own text with no reference at all.
	Ref func(url, text string) string
	// Missing collects the characters no font in the build can set, so that the
	// audit can report them against the section they are in rather than against
	// a line of a generated file nobody wrote.
	Missing func(where string, runes []rune)
	// Stray collects the control sequences found in prose that the table in
	// control.go does not know, or knows and cannot complete. They are set as
	// the literal characters they are made of, which is visible on the page and
	// is meant to be, and the audit counts them.
	Stray func(where string, cs []string)
	// Rescued collects the mathematics the corpus wrote in its prose with no
	// dollars round it and this build put dollars round for it. It is not a
	// complaint about the build, it is the size of a job in the corpus, and it
	// is counted for the same reason Stray is: so that a defect somebody has
	// worked around cannot quietly stop being anybody's problem.
	Rescued func(where string, atoms []string)
	// Wide collects the arrays whose preamble came out narrower than their own
	// widest row. The preamble is widened so the build carries on, and the
	// report is what somebody takes back to the page.
	Wide func(where string, a wideArray)
	// Contents is the sentence case title each numbered subsection of this file
	// takes in the table of contents, keyed by its number. It is empty for a
	// language the volume was not printed in, and a subsection it has nothing
	// for is listed under its heading.
	Contents map[int]string
	// Lang is the language the body is in, which the title caser needs: the
	// short words a title leaves in lower case are a different list in each.
	Lang string
	// Aligned counts the displays that were written over several lines and have
	// been set as a calculation aligned on the relation. It is not a complaint,
	// it is how many places the build made a decision about the layout of a
	// formula that the corpus did not spell out.
	Aligned func(where string)
}

// head is the running head of a numbered subsection, in the two forms the class
// picks between. Both are title case, because the head is set as small capitals
// and small capitals of a capitalised string are capitals.
//
// The short form is the title up to its first full stop. The printing shortens a
// head at a sentence when the whole of it will not go, and the class is what
// decides whether it will go, because that is a question about how wide a string
// sets in a font.
func (r Renderer) head(words string) string {
	full := listed(words, r.Lang)
	short := full
	if i := strings.Index(full, ". "); i > 0 {
		short = full[:i+1]
	}
	return fmt.Sprintf("\\bheadfit{%s}{%s}", r.titleText(full), r.titleText(short))
}

// unemphasised takes the Markdown emphasis markers out of a title, leaving the
// words that were inside them.
//
// The contents line and the running head are the same words as the heading in a
// different case, and neither of them wants the heading's italic. The contents
// is set in the contents face and the head in small capitals, and a run of
// italic inside either is markup showing through rather than anything the
// printing does. Where manifests/toc/ has the title this never came up, because
// the manifest holds plain words; where it has not, the title is the heading
// itself, asterisks and all. That is how 38 contents lines across six volumes
// came to read \emph{Modules M-plats}, with the running head title cased off the
// backslash so that it printed \emph{modules M-plats} in the bargain.
//
// Only asterisks are markers here. An underscore in a title is a subscript the
// prose rescue is about to pick up, and a run between dollars is mathematics
// where an asterisk is an adjoint, so those are left exactly as they were.
// The mathematics goes out of the way first rather than the string being cut on
// the dollars and each piece done separately, because a title can put emphasis
// round a formula and the pair of asterisks then lands in two different pieces.
// Integration V, § 6, no. 2 is "*Structure de monoide sur $D(A)$*" and it needs
// both of its asterisks in the same string to lose either of them.
func unemphasised(s string) string {
	var math []string
	s = titleMathRE.ReplaceAllStringFunc(s, func(m string) string {
		math = append(math, m)
		return "\x00t" + itoa(len(math)-1) + "\x00"
	})
	s = boldRE.ReplaceAllString(s, `$1`)
	s = emRE.ReplaceAllString(s, `$1$2$3`)
	if len(math) == 0 {
		return s
	}
	return titleMathBackRE.ReplaceAllStringFunc(s, func(m string) string {
		var i int
		fmt.Sscanf(m, "\x00t%d\x00", &i)
		return math[i]
	})
}

var (
	titleMathRE     = regexp.MustCompile(`\$[^$\n]*\$`)
	titleMathBackRE = regexp.MustCompile("\x00t(\\d+)\x00")
)

// titleText renders a title that may still have its mathematics in dollars.
//
// Every other string that reaches inline came out of a body mask() had already
// been over, so its formulae are placeholders by then and there is not a dollar
// left in it. A subsection title off manifests/toc/ has not been anywhere near
// mask: load reads it straight out of the YAML and the heading writer is handed
// it whole. inline escapes a dollar, because a loose dollar in a body is prose,
// so "$\\tau$-Extensions of Groups" set as \\$$\\tau$\\$-Extensions of Groups,
// two printed dollar signs around the tau in the contents line and in the
// running head of every page of the subsection.
//
// So the title gets what the body got, one span at a time. A title with no
// dollars in it goes through inline exactly as before, and that is every title
// read off a heading and every manifest that has not had its symbols put back
// into math yet.
func (r Renderer) titleText(s string) string {
	spans, unclosed := mathtex.Split(s)
	if len(spans) == 0 || unclosed != nil {
		return r.inline(s)
	}
	rs := []rune(s)
	var b strings.Builder
	at := 0
	for _, sp := range spans {
		d := 1
		if sp.Display {
			d = 2
		}
		if sp.Start-d < at || sp.End+d > len(rs) {
			return r.inline(s)
		}
		b.WriteString(r.inline(string(rs[at : sp.Start-d])))
		text := Math(sp.Text)
		if r.Missing != nil {
			if runes := Missing(text); len(runes) > 0 {
				r.Missing(r.at(sp.Line), runes)
			}
		}
		b.WriteString("$" + text + "$")
		at = sp.End + d
	}
	b.WriteString(r.inline(string(rs[at:])))
	return b.String()
}

// TeX renders a body.
func (r Renderer) TeX(body string) (string, error) {
	masked, spans, err := r.mask(body)
	if err != nil {
		return "", err
	}
	masked, tags := numbers(masked, spans)
	var b strings.Builder
	// The statement of a proposition is set in italic and the discussion after
	// it is not, and the corpus does not mark the join: a #### heading is
	// followed by the statement and then by however many paragraphs of
	// commentary the printing has, all under the one heading. What the printing
	// does mark is where the italic stops, and it stops at the end of the first
	// paragraph. So the first paragraph after the head goes in italic and the
	// rest does not. A proposition whose statement runs to two paragraphs loses
	// the second, which happens, and is a smaller error than setting a page of
	// commentary in italic, which would happen far more often.
	italic := false
	closeItalic := func() {
		if italic {
			b.WriteString("\\bstateend\n")
			italic = false
		}
	}
	bs := blocks(masked)
	for i, block := range bs {
		joined := i+1 < len(bs) && opensDisplay(bs[i+1], spans)
		if strings.HasPrefix(block, "#") {
			closeItalic()
			head, opens := r.heading(block)
			b.WriteString(head)
			italic = opens
			continue
		}
		b.WriteString(r.paragraph(block, joined))
		if !joined {
			closeItalic()
		}
	}
	closeItalic()
	out := r.unmask(b.String(), spans, tags)
	// Nothing in the corpus contains a NUL, so one here is a placeholder this
	// package made and then lost track of. TeX prints it as ^^@ and carries on
	// for a while and then stops in a way that names the generated file rather
	// than the source, which is a bad afternoon. Refusing here names the source.
	if i := strings.IndexByte(out, 0); i >= 0 {
		return "", fmt.Errorf("%s: a placeholder was not restored near %q",
			r.at(1+strings.Count(out[:i], "\n")), nearby(out, i))
	}
	return out, nil
}

// nearby is the text either side of a byte, for a message.
func nearby(s string, i int) string {
	lo, hi := max(0, i-40), min(len(s), i+40)
	return strings.ReplaceAll(s[lo:hi], "\x00", "@")
}

// mask replaces every math span with a placeholder. The placeholder is built
// from NUL, which cannot occur in the corpus: the files are UTF-8 text out of a
// PDF and a NUL in one would have failed the audit long before it got here.
func (r Renderer) mask(body string) (string, []mathtex.Span, error) {
	spans, unclosed := mathtex.Split(body)
	if unclosed != nil {
		return "", nil, fmt.Errorf("%s: a math span opens and never closes", r.at(unclosed.Line))
	}
	rs := []rune(body)
	var b strings.Builder
	at := 0
	for i, s := range spans {
		d := 1
		if s.Display {
			d = 2
		}
		b.WriteString(string(rs[at : s.Start-d]))
		fmt.Fprintf(&b, "\x00m%d\x00", i)
		at = s.End + d
	}
	b.WriteString(string(rs[at:]))
	return b.String(), spans, nil
}

var placeholderRE = regexp.MustCompile("\x00m(\\d+)\x00")

// eqNumRE is a formula number alone on a line, "(12)" or "(iv)".
var eqNumRE = regexp.MustCompile(`^\(([0-9]{1,3}|[ivxlIVXL]{1,5})\)$`)

// eqNumLeadRE is a formula number and its display on one line, which is the
// third of the three ways a number arrives and the one that looks least like a
// fault in the Markdown.
var eqNumLeadRE = regexp.MustCompile("^\\(([0-9]{1,3}|[ivxlIVXL]{1,5})\\)\\s+(\x00m\\d+\x00)$")

// numbers takes the formula numbers off their own lines and gives them to the
// displays they belong to.
//
// The corpus writes them the way they come off the page. Bourbaki sets the
// number in the left margin beside the display, so a reader of a scan sees a
// short line holding "(12)" and then the formula, and that is what got written
// down: "(12)", newline, the display. Left where they are, they set as a
// one word paragraph above a centred formula, which is not what the printing
// has and reads like a mistake.
//
// So they come off and go back on as \tag, and the class numbers on the left.
// A number is only taken when the display it belongs to is the only thing on
// its line, which is what keeps "(2) In Z and more generally" and every other
// enumerated paragraph out of it.
//
// The number arrives in three places and for a while only one of them was read.
// In the margin of a scan the number sits beside the formula, so where it lands
// in the text depends on how the page was read, and a census over the corpus
// found 1530 written before the display, 590 on the same line as it and 366
// after it. Only the first was lifted, so 956 numbered formulae in the library,
// 38 per cent of them, set the number as a one word paragraph of its own and
// left the formula unnumbered. Page 18 of Commutative Algebra I is what that
// looks like: a line reading "(3)", a blank, and then the prose.
//
// Looking backwards is safe for the same reason looking forwards is. A number
// that opens an enumerated item has the item's text after it on the same line,
// so a number alone on a line under a display is not the start of anything.
func numbers(masked string, spans []mathtex.Span) (string, map[int]string) {
	tags := map[int]string{}
	lines := strings.Split(masked, "\n")
	keep := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		// The number and its display on one line. The number comes off and the
		// display stays where it is, which is already a block of its own.
		if m := eqNumLeadRE.FindStringSubmatch(strings.TrimSpace(lines[i])); m != nil {
			if p, ok := soleDisplay(m[2], spans); ok {
				if _, taken := tags[p]; !taken {
					tags[p] = m[1]
					keep = append(keep, m[2])
					continue
				}
			}
		}
		m := eqNumRE.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			keep = append(keep, lines[i])
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j < len(lines) {
			if p, ok := soleDisplay(lines[j], spans); ok {
				if _, taken := tags[p]; !taken {
					tags[p] = m[1]
					i = j - 1 // the blank lines between go too, so the display keeps its block
					continue
				}
			}
		}
		// Nothing ahead of it, so look behind. The display is already in keep,
		// and only the last thing put there that is not blank can be it.
		if p, ok := lastDisplay(keep, spans); ok {
			if _, taken := tags[p]; !taken {
				tags[p] = m[1]
				continue
			}
		}
		keep = append(keep, lines[i])
	}
	return strings.Join(keep, "\n"), tags
}

// lastDisplay is the display a number written under it belongs to: the most
// recent line that is not blank, and only when that line is a display and
// nothing else. Anything else between the two, a word of prose or a heading,
// means the number is not this display's.
func lastDisplay(keep []string, spans []mathtex.Span) (int, bool) {
	for i := len(keep) - 1; i >= 0; i-- {
		if strings.TrimSpace(keep[i]) == "" {
			continue
		}
		return soleDisplay(keep[i], spans)
	}
	return 0, false
}

// soleDisplay says whether a line is one display placeholder and nothing else,
// and which span it is. A line with any prose on it is a paragraph that happens
// to have a formula in it, and the number before it was not a formula number.
//
// A display that opens an environment of its own counts here like any other. It
// used not to, because a \tag has nowhere to go in one, and the number was then
// left behind as a paragraph reading "(23)" above the formula. There are three
// of those in the corpus. unmask now takes such an environment down to the form
// that sets a row inside a display rather than a display of its own, and the
// number goes where the printing has it.
func soleDisplay(line string, spans []mathtex.Span) (int, bool) {
	m := placeholderRE.FindString(strings.TrimSpace(line))
	if m == "" || strings.TrimSpace(placeholderRE.ReplaceAllString(line, "")) != "" {
		return 0, false
	}
	var n int
	fmt.Sscanf(m, "\x00m%d\x00", &n)
	if n >= len(spans) || !spans[n].Display {
		return 0, false
	}
	return n, true
}

// unmask puts the mathematics back as TeX, which is the whole reason a TeX
// corpus was worth insisting on: the span goes out exactly as it came in.
//
// A display is wrapped in \[ \] unless it opens an environment of its own. 426
// of the corpus's 25820 displays begin with \begin{align}, and align inside \[
// is an error in LaTeX rather than a formula, so those go out bare. KaTeX
// accepts both spellings, which is how they got written both ways.
func (r Renderer) unmask(s string, spans []mathtex.Span, tags map[int]string) string {
	if len(spans) == 0 {
		return s
	}
	out := make([]string, len(spans))
	for i, sp := range spans {
		text := Math(sp.Text)
		if r.Missing != nil {
			if runes := Missing(text); len(runes) > 0 {
				r.Missing(r.at(sp.Line), runes)
			}
		}
		if strings.Contains(text, `\begin{`) {
			widened, wide := widen(text)
			text = diagrams(widened)
			for _, w := range wide {
				if r.Wide != nil {
					r.Wide(r.at(sp.Line), w)
				}
			}
		}
		if !sp.Display {
			out[i] = "$" + text + "$"
			continue
		}
		body := strings.TrimSpace(text)
		tag, tagged := tags[i]
		// A number the corpus wrote on the first line inside the display is the
		// same number as one written on the line above it, and the printing sets
		// it in the same place. Left where it is it also hides the environment
		// underneath it, which is how the one in Lie VI, § 3 came to be wrapped
		// in \[ \] and stopped both builds of that volume.
		if n, rest, ok := liftNumber(body); ok {
			body = rest
			if !tagged {
				tag, tagged = n, true
			}
		}
		// A number the corpus wrote inside the display rather than on the line
		// above it is the same number and belongs in the same place. Left where
		// it is, it ends up inside the \begin{aligned} this build puts around a
		// calculation, and amsmath will not set a \tag there and stops: the tag
		// belongs to the display and an aligned is a box inside one. Seven of
		// the seventeen volumes that would not typeset stopped on this.
		if countTags(body) > 1 {
			body, _ = inlineTags(body)
		} else if inner, rest, ok := liftTag(body); ok {
			body = rest
			if !tagged {
				tag, tagged = inner, true
			}
		}
		if ownEnvironment(body) {
			if !tagged {
				out[i] = "\n" + body
				continue
			}
			// The environment sets a display of its own and amsmath refuses to
			// have one inside another, so a number that has to go beside it goes
			// there by taking the environment down to the form that sets rows
			// inside a display somebody else opened.
			if inner, ok := inlineEnvironment(body); ok {
				out[i] = "\n\\[\n\\tag{" + tag + "}\n" + inner + "\n\\]"
			} else {
				out[i] = "\n" + body
			}
			continue
		}
		if a, ok := alignedDisplay(body); ok {
			body = a
			if r.Aligned != nil {
				r.Aligned(r.at(sp.Line))
			}
		}
		if tagged {
			out[i] = "\n\\[\n\\tag{" + tag + "}\n" + body + "\n\\]"
			continue
		}
		out[i] = "\n\\[\n" + body + "\n\\]"
	}
	return placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		var i int
		fmt.Sscanf(m, "\x00m%d\x00", &i)
		return out[i]
	})
}

// alignedDisplay sets a display whose source runs over several lines the way the
// printing sets it, one step to a line aligned on the relation, instead of
// running the whole calculation into one line the page has no room for.
//
// The corpus writes a long calculation the way the page has it, a step to a
// line, and \[ \] turns those newlines into spaces. That is where the widest
// overfull boxes came from: an identity in Algebra III, the one that multiplies
// two sums of four squares, ran 415 pt past a 326 pt measure, which is a formula
// two and a half times the width of the page it is on. There are 323 displays
// written over several lines in the English Algebra alone.
//
// A line that opens with a relation or with a sign is aligned in front of it,
// which is the continuation of the step above. A line that does not is aligned
// in front of its own first relation, and a line with no relation at all ends at
// the alignment column, so that the head of a calculation sits above the first
// "=" rather than beside it. That is the arrangement the printing uses and it is
// what \begin{aligned} gives with an ampersand in those three places.
//
// A display already carrying an ampersand or an environment of its own is left
// alone. An array or a cases is written over several lines too and already says
// where its own columns are, and putting that inside an aligned would align on
// the wrong thing.
func alignedDisplay(text string) (string, bool) {
	if strings.Contains(text, `\begin{`) || strings.Contains(text, "&") {
		return "", false
	}
	lines := displayRows(text)
	if len(lines) < 2 {
		return "", false
	}
	if relationAt(lines[0]) < 0 {
		// The head of the calculation carries no relation of its own, so there
		// is nothing to align it on and it stands on its own line with the rest
		// stepped in under it. That is what page 648 of the English Algebra does
		// with the identity between two sums of four squares: the product stands
		// alone at the margin and the four squared terms follow indented, each
		// under the sign of the one above.
		for i, l := range lines {
			if i == 0 {
				lines[i] = "&" + l
				continue
			}
			lines[i] = `&\quad ` + l
		}
	} else {
		for i, l := range lines {
			lines[i] = ampersand(l)
		}
	}
	return "\\begin{aligned}\n" + strings.Join(lines, " \\\\\n") + "\n\\end{aligned}", true
}

// displayRows is the lines of a display grouped into the rows of an alignment.
//
// A newline in display mathematics is a space, so the corpus writing a
// calculation over several lines is not the corpus saying where the rows go. It
// usually amounts to the same thing, because a step of a calculation is what
// gets written on a line. It does not when a line ends in the middle of
// something that has to finish where it started: a \left with its \right on the
// line below, or an open brace, most often a \frac or a \substack whose second
// argument the corpus wrote underneath the first. Ending a row there puts the
// \left in one cell of the aligned and the \right in the next, and TeX stops
// with "Extra }, or forgotten \right", which is exactly true and says nothing
// about where to look.
//
// So a row runs on until what it holds is closed. In Set Theory III, § 5,
// exercise 5 that is a sum over the subsets of even cardinal written as five
// lines inside one \left( \right), and it belongs in one row of the alignment
// rather than five.
func displayRows(text string) []string {
	var rows, cur []string
	fence, brace := 0, 0
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(l), `\\`))
		if l == "" {
			continue
		}
		cur = append(cur, l)
		f, b := fences(l)
		fence += f
		brace += b
		if fence <= 0 && brace <= 0 {
			rows = append(rows, strings.Join(cur, " "))
			cur, fence, brace = nil, 0, 0
		}
	}
	if len(cur) > 0 {
		rows = append(rows, strings.Join(cur, " "))
	}
	return rows
}

// fences counts what a line of mathematics opens and does not close: \left
// against \right, and braces that are not escaped.
//
// It reads control words rather than searching for the text, because \leftarrow
// and \rightharpoonup begin with the same letters as the two words being counted
// and neither of them opens anything.
func fences(s string) (fence, brace int) {
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '\\' {
			switch rs[i] {
			case '{':
				brace++
			case '}':
				brace--
			}
			continue
		}
		j := i + 1
		for j < len(rs) && isTeXLetter(rs[j]) {
			j++
		}
		if j == i+1 {
			// \{ and \} and \\ and the rest of the single character escapes.
			// The character after the backslash is not a brace of its own and
			// is not the start of a word, so it is stepped over.
			i = j
			continue
		}
		switch string(rs[i:j]) {
		case `\left`:
			fence++
		case `\right`:
			fence--
		}
		i = j - 1
	}
	return fence, brace
}

func isTeXLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// liftNumber takes an equation number off the first line of a display and hands
// it back, so that the caller can set it as the number of the display rather
// than typeset it as a pair of parentheses at the head of the formula.
//
// The number belongs on the line above the $$ and is usually written there, in
// which case numbers has already taken it. The corpus has one written just
// inside instead. It is the same number and it goes to the same place, and
// leaving it where it is also puts it in front of whatever the display opens
// with, which hides an environment that sets its own display from the check for
// one.
func liftNumber(body string) (num, rest string, ok bool) {
	head, tail, cut := strings.Cut(body, "\n")
	if !cut {
		return "", body, false
	}
	m := eqNumRE.FindStringSubmatch(strings.TrimSpace(head))
	if m == nil {
		return "", body, false
	}
	return m[1], strings.TrimSpace(tail), true
}

// inlineForm is the environment that sets the same rows inside a display
// somebody else has opened, for each environment that opens one of its own.
//
// multline and equation have no such form in amsmath. Both set a single
// formula, one broken over lines and one not, and aligned sets either of those
// without the line breaking that is the whole point of multline. That is a
// worse setting of a formula rather than a wrong one, and it is only reached by
// a display that has a number to put beside it, which is three displays in the
// corpus and none of them is a multline.
var inlineForm = map[string]string{
	"align":    "aligned",
	"alignat":  "alignedat",
	"flalign":  "aligned",
	"gather":   "gathered",
	"multline": "aligned",
	"equation": "aligned",
}

// inlineEnvironment rewrites a display that opens an environment of its own
// into the same rows inside an environment that does not.
//
// It refuses anything but a body that is one environment from end to end. A
// display with an environment in the middle of it is a shape the writer has not
// got a reading for, and turning the one it can see inside out would move rows
// of a formula around on a guess.
func inlineEnvironment(body string) (string, bool) {
	body = strings.TrimSpace(body)
	m := displayEnvironment.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	inner, ok := inlineForm[m[1]]
	if !ok {
		return "", false
	}
	open := strings.TrimSpace(m[0])
	star := ""
	if strings.HasSuffix(open, "*}") {
		star = "*"
	}
	end := `\end{` + m[1] + star + `}`
	if !strings.HasSuffix(body, end) {
		return "", false
	}
	// The starred and unstarred spellings differ only in whether the rows are
	// numbered, and a row inside a display somebody else opened is never
	// numbered, so the inner environment has the one spelling.
	rows := strings.TrimSuffix(body[len(open):], end)
	return `\begin{` + inner + `}` + rows + `\end{` + inner + `}`, true
}

// liftTag takes a \tag out of the body of a display and hands it back, so that
// the caller can put it where amsmath will set it.
//
// The argument is read by counting braces rather than by looking for the next
// one, because a tag is not always a number. Of the 1134 in the corpus ten are
// a reference to somewhere else in the Elements, printed beside the formula the
// way a number would be, and one of those is \tag{$A \in \mathcal{S}(X)$}. Read
// to the first closing brace that is \tag{$A \in \mathcal{S, and taking that
// out of the display leaves the rest of the formula behind as text.
func liftTag(body string) (tag, rest string, ok bool) {
	at, open, close, ok := findTag(body, 0)
	if !ok {
		return "", body, false
	}
	return tagText(body[open+1 : close]), strings.TrimSpace(body[:at] + body[close+1:]), true
}

// findTag is where the first \tag at or after from begins, where its argument
// begins, and where its argument ends. The argument is read by counting braces,
// for the reason given above liftTag.
func findTag(body string, from int) (at, open, close int, ok bool) {
	rel := strings.Index(body[from:], `\tag`)
	if rel < 0 {
		return 0, 0, 0, false
	}
	at = from + rel
	open = at + len(`\tag`)
	if open < len(body) && body[open] == '*' {
		open++
	}
	if open >= len(body) || body[open] != '{' {
		return 0, 0, 0, false
	}
	depth := 0
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '\\':
			i++ // an escaped brace is not a brace
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return at, open, i, true
			}
		}
	}
	return 0, 0, 0, false
}

// inlineTags sets every \tag in a display where it already stands, at the right
// of the row it sits on, and hands back how many it moved.
//
// A display with one \tag is a numbered formula, the number belongs beside the
// display as a whole, and liftTag takes it there. A display with several is a
// different thing: each one justifies the step it is written on, the way the
// printing sets "(prop. 38)" out to the right of the line it applies to. There
// is nowhere beside a display for a second number to go, and amsmath will not
// set a \tag inside the \begin{aligned} that a calculation written over several
// lines gets wrapped in, so it stops with "\tag not allowed here". Both builds
// of Lie I to III failed on the same display in III, § 3, no. 1, which justifies
// three of its four steps that way.
func inlineTags(body string) (string, int) {
	n, from := 0, 0
	for {
		at, open, close, ok := findTag(body, from)
		if !ok {
			return body, n
		}
		set := `\qquad(\text{` + tagText(body[open+1:close]) + `})`
		body = body[:at] + set + body[close+1:]
		from = at + len(set)
		n++
	}
}

// countTags is how many \tag a display carries.
func countTags(body string) int {
	n, from := 0, 0
	for {
		_, _, close, ok := findTag(body, from)
		if !ok {
			return n
		}
		n++
		from = close + 1
	}
}

// splitFootnotes takes the footnotes out of a heading and hands them back.
//
// mark is the heading with a \bfnmark where each note was, for the head on
// the page. plain is the heading with nothing where each note was, for the two
// places the same words are written a second time. notes are the bodies, for
// the \footnotetext that has to follow the heading.
func splitFootnotes(s string) (mark, plain string, notes []string) {
	var m, p strings.Builder
	for i := 0; i < len(s); {
		at := strings.Index(s[i:], `\footnote{`)
		if at < 0 {
			m.WriteString(s[i:])
			p.WriteString(s[i:])
			break
		}
		at += i
		m.WriteString(s[i:at])
		p.WriteString(s[i:at])
		open := at + len(`\footnote`)
		body, end, ok := braceArg(s, open)
		if !ok {
			// An opening brace with no closing one is not a footnote, and
			// guessing where it ends would invent a note the page has not got.
			m.WriteString(s[at:])
			p.WriteString(s[at:])
			break
		}
		notes = append(notes, body)
		m.WriteString(`\bfnmark`)
		i = end
	}
	return strings.TrimSpace(m.String()), strings.TrimSpace(p.String()), notes
}

// braceArg reads the group that starts at open and returns what is inside it
// and the offset just past its closing brace.
func braceArg(s string, open int) (arg string, end int, ok bool) {
	if open >= len(s) || s[open] != '{' {
		return "", open, false
	}
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // an escaped brace is a character rather than a group
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1, true
			}
		}
	}
	return "", open, false
}

// tagText is the argument of a \tag, which amsmath sets in text mode.
//
// It is the one piece of a display that is not a formula, and the writer got to
// it through the mathematics, so a degree sign has already become a superscript
// circle. That is right in a formula and it is an error beside one: the only
// tag in the corpus that carries it is a reference reading "n° 6", and n° 6 is
// a phrase rather than a quantity. The rest of what a tag holds sets in text
// mode as it stands, including the ones that put their own dollars around the
// part of themselves that is a formula.
func tagText(s string) string {
	return strings.ReplaceAll(s, `^\circ`, `\textdegree`)
}

// ampersand puts one alignment mark in a line of a calculation.
func ampersand(line string) string {
	switch at := relationAt(line); {
	case at == 0:
		return "&" + line
	case at > 0:
		return strings.TrimRight(line[:at], " ") + " &" + line[at:]
	}
	return line + " &"
}

// relationAt is the byte offset of the first relation in a line that is not
// inside a group, or -1 for a line that has none.
//
// Only outside braces, because the "=" of \text{where $x = y$} and the "-" of
// f^{-1} are parts of something else and aligning on them would take the line
// apart in the middle of a term. A leading sign counts, because a line that
// opens with one is the continuation of the sum above it, but a sign in the
// middle of a line does not: x - y is one term of a step and not a new step.
func relationAt(line string) int {
	depth := 0
	for i := 0; i < len(line); {
		c := line[i]
		switch {
		case c == '{':
			depth++
			i++
		case c == '}':
			depth--
			i++
		case c == '\\':
			j := i + 1
			for j < len(line) && isASCIILetter(line[j]) {
				j++
			}
			if j == i+1 {
				// An escaped character, \{ or \, or the like. Never a relation,
				// and stepping over both bytes is what keeps an escaped brace
				// from moving the depth.
				j = i + 2
			}
			if depth == 0 && relations[line[i:j]] {
				return i
			}
			if depth == 0 && i == 0 && leadingOps[line[i:j]] {
				return 0
			}
			i = j
		case depth == 0 && (c == '=' || c == '<' || c == '>'):
			return i
		case depth == 0 && i == 0 && (c == '+' || c == '-'):
			return 0
		default:
			i++
		}
	}
	return -1
}

func isASCIILetter(b byte) bool { return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' }

// relations are the control words a step of a calculation turns on. The list is
// the relations the corpus actually uses in a display written over more than one
// line, plus the near neighbours of each, and it is matched on the whole control
// word so that \le does not match the front of \left.
var relations = map[string]bool{
	`\le`: true, `\leq`: true, `\ge`: true, `\geq`: true,
	`\ne`: true, `\neq`: true, `\equiv`: true, `\sim`: true,
	`\simeq`: true, `\cong`: true, `\approx`: true, `\propto`: true,
	`\subset`: true, `\subseteq`: true, `\subsetneq`: true,
	`\supset`: true, `\supseteq`: true, `\supsetneq`: true,
	`\in`: true, `\ni`: true, `\notin`: true, `\perp`: true,
	`\to`: true, `\rightarrow`: true, `\longrightarrow`: true,
	`\mapsto`: true, `\longmapsto`: true, `\leftarrow`: true,
	`\Rightarrow`: true, `\Leftrightarrow`: true, `\iff`: true,
	`\implies`: true, `\prec`: true, `\succ`: true,
	`\ll`: true, `\gg`: true, `\doteq`: true, `\models`: true,
}

// leadingOps are the operators that begin a continuation line. They count only
// at the head of a line, where they carry the sum on from the line above.
var leadingOps = map[string]bool{
	`\pm`: true, `\mp`: true, `\cup`: true, `\cap`: true,
	`\times`: true, `\otimes`: true, `\oplus`: true, `\wedge`: true,
	`\vee`: true, `\cdot`: true, `\circ`: true, `\bigcup`: true,
	`\bigcap`: true, `\bigoplus`: true, `\bigotimes`: true,
}

// displayEnvironment is the environments that set their own display and must
// not be put inside one. The matrices and cases and arrays are not here: those
// go inside a display and want the \[ \] around them.
var displayEnvironment = regexp.MustCompile(`^\s*\\begin\{(align|alignat|gather|multline|equation|flalign)\*?\}`)

func ownEnvironment(tex string) bool { return displayEnvironment.MatchString(tex) }

// blocks cuts a masked body into headings and paragraphs. A heading is its own
// block whether or not a blank line follows it, because a file that lost one
// should still come out as a heading and not as a paragraph beginning with
// hashes.
func blocks(masked string) []string {
	var out []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for line := range strings.SplitSeq(masked, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			flush()
		case strings.HasPrefix(line, "#"):
			flush()
			out = append(out, line)
		default:
			cur = append(cur, line)
		}
	}
	flush()
	return out
}

// headingRE reads a heading and the attribute block assembly writes on it: the
// label, the classes, and tag=XXXX. It is the same expression publish uses,
// deliberately, because the two have to agree about what a heading is.
var headingRE = regexp.MustCompile(`^(#{1,6})\s+(.*?)(?:\s*\{#([a-z0-9-]+)((?:\s+[^}\s]+)*)\})?\s*$`)

var tagAttrRE = regexp.MustCompile(`\btag=([0-9A-Z]{4})\b`)

// numberedRE is a no. heading, "3. ASSOCIATIVE LAWS". The number is the one the
// § prints and the class sets it in the margin, so it is pulled out rather than
// left in the title.
var numberedRE = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)

// statementRE is a statement heading, "Proposition 7" or "Theorem 2
// (Commutativity theorem)". Bourbaki numbers most statements within the § and
// leaves some unnumbered, and both forms are here.
var statementRE = regexp.MustCompile(`^([A-Za-zÀ-ÿ]+(?:\s+[a-z]+)*?)(?:\s+(\d+))?\s*(?:\(([^)]*)\))?\s*$`)

// roman is the statement kinds whose text the printing sets upright. Everything
// else that reaches \bstate is a proposition or a definition or one of their
// relatives, and those are italic. The list is in the three languages the corpus
// has, because the kind is the word the page uses and nothing translates it back.
var roman = map[string]bool{
	"Remark": true, "Remarks": true, "Remarque": true, "Remarques": true,
	"Nhận xét": true,
	"Example":  true, "Examples": true, "Exemple": true, "Exemples": true,
	"Ví dụ": true,
	"Note":  true, "Notes": true, "Ghi chú": true,
}

// heading renders a heading and says whether it opens an italic statement, in
// which case the caller closes it after the next paragraph.
func (r Renderer) heading(line string) (string, bool) {
	m := headingRE.FindStringSubmatch(line)
	if m == nil {
		return r.paragraph(line, false), false
	}
	level, text, anchor, attrs := len(m[1]), strings.TrimSpace(m[2]), m[3], m[4]
	label := ""
	if r.Label != nil && anchor != "" {
		label = r.Label(anchor)
	}
	switch level {
	case 1, 2:
		// A level one or two heading this far into a body is not the file naming
		// itself, because StripTitle has already taken that off the front. Four
		// files in the corpus have one, where the printing runs two divisions
		// together under one head.
		return fmt.Sprintf("\\bpart{%s}{%s}\n\n", r.inline(text), label), false
	case 3:
		if n := numberedRE.FindStringSubmatch(text); n != nil {
			// The third argument is the title the contents lists it under, which
			// is the same words in a different case and is not derivable from
			// the second. Where manifests/toc/ has not got it, the head goes in
			// both places.
			words := n[2]
			if no, err := strconv.Atoi(n[1]); err == nil && r.Contents[no] != "" {
				words = r.Contents[no]
			}
			// A footnote in the title cannot travel with it. Three of the five
			// arguments are the title, and two of those are read again later,
			// once by \addcontentsline into the .toc and once by \markright
			// into the running head, and a \footnote read a second time is the
			// error "Use of \@xfootnote doesn't match its definition". The
			// printing has one of these, on no. 10 of Integration VI, § 2, and
			// it stopped both volumes that carry that chapter. So the head
			// keeps a mark, the note follows the heading, and the contents line
			// and the running head have neither.
			mark, _, notes := splitFootnotes(n[2])
			_, contents, _ := splitFootnotes(words)
			contents = unemphasised(contents)
			out := fmt.Sprintf("\\bno{%s}{%s}{%s}{%s}{%s}\n\n",
				n[1], r.inline(mark), r.titleText(contents), r.head(contents), label)
			for _, note := range notes {
				out += "\\footnotetext{" + r.inline(note) + "}\n\n"
			}
			return out, false
		}
		return fmt.Sprintf("\\bnamed{%s}{%s}\n\n", r.inline(text), label), false
	}
	kind, number, note := statement(text)
	tag := ""
	if t := tagAttrRE.FindStringSubmatch(attrs); t != nil {
		tag = t[1]
	}
	head := fmt.Sprintf("\\bstate{%s}{%s}{%s}{%s}{%s}",
		r.inline(kind), number, r.inline(note), tag, label)
	if roman[kind] {
		return head + "\n", false
	}
	return head + "\\bstatebegin\n", true
}

// statement cuts a statement heading into its three parts. A heading it cannot
// read comes back whole as the kind, which sets it as it stands rather than
// losing half of it to a regular expression that was written for the other
// nineteen thousand.
func statement(text string) (kind, number, note string) {
	m := statementRE.FindStringSubmatch(text)
	if m == nil {
		return text, "", ""
	}
	return strings.TrimSpace(m[1]), m[2], strings.TrimSpace(m[3])
}

// opensDisplay says whether a block starts with a displayed formula. The corpus
// writes a display with a blank line in front of it, which makes it the start of
// a block of its own, but the printing sets it inside the sentence that leads
// into it: "may be expressed by the formulae", then the formula, then whatever
// qualifies it. So a block that opens with a display is joined to the one
// before, and the blank line between them goes.
func opensDisplay(block string, spans []mathtex.Span) bool {
	loc := placeholderRE.FindStringIndex(block)
	if loc == nil || strings.TrimSpace(block[:loc[0]]) != "" {
		return false
	}
	var n int
	fmt.Sscanf(block[loc[0]:loc[1]], "\x00m%d\x00", &n)
	return n < len(spans) && spans[n].Display
}

// displayOnly says whether a block is a displayed formula and nothing else.
func displayOnly(block string) bool {
	return placeholderRE.MatchString(block) &&
		strings.TrimSpace(placeholderRE.ReplaceAllString(block, "")) == ""
}

// paragraph renders one block. It is told whether a display comes next, because
// that decides whether the block ends with a blank line or not, and the answer
// is worth a line of explanation.
//
// A blank line before \[ leaves TeX in vertical mode. amsmath then opens a
// paragraph of its own to hold the display, and the page gets a whole empty line
// above the formula on top of \abovedisplayskip. The printing has the formula
// hanging off the end of the sentence that introduces it, with no more space
// than a display normally takes. So the blank line goes only after the display
// and never before it, and the same rule joins two displays that follow one
// another.
func (r Renderer) paragraph(block string, joined bool) string {
	end := "\n\n"
	if joined {
		// Nothing at all, not a newline: a display carries its own line break in
		// front of it, and a second one here is the blank line this is avoiding.
		end = ""
	}
	if displayOnly(block) {
		return block + end
	}
	return r.inline(strings.ReplaceAll(block, "\n", " ")) + end
}

// Markdown escapes. The corpus writes \* for the star that opens a forward
// looking passage, 595 of them, and \_ and a few others where an OCR read a
// character Markdown would otherwise have eaten. They have to come out before
// the prose is escaped, or the backslash becomes a printed backslash, and they
// have to stay out of reach of the emphasis reader, or the star opens an italic
// that runs to the end of the paragraph.
var mdEscapeRE = regexp.MustCompile(`\\([*_$#\[\]()~^\\&%{}])`)

const (
	escOpen  = "\x00e"
	escClose = "\x00"
)

// The placeholder holds an index and not the character itself, because the
// character itself would still be a brace or a backslash and the prose escaper
// runs next and would escape it where it stood. That happened: a \{ in prose
// came out of the escaper as the placeholder with a backslash inside it, which
// then matched nothing, and a NUL reached the typesetter.
var escRestoreRE = regexp.MustCompile("\x00e(\\d+)\x00")

var (
	boldRE = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	// emRE is a single pair of asterisks. The corpus has 20853 of them in
	// English alone, which is Bourbaki setting a term in italic where it is
	// defined, and a book that printed the asterisks instead would be unreadable
	// in twenty thousand places.
	emRE   = regexp.MustCompile(`(?:^|([^*\x00]))\*([^*\n]+)\*(?:$|([^*]))`)
	linkRE = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	// imageRE is a Markdown image, which in this corpus is never an image.
	//
	// 62 files carry one and not a single target exists. Some point at
	// ../images/fig_1.png, which is a directory the corpus does not have and by
	// policy will not have, and some point at an imgur URL the reading invented.
	// What is real is that the printing has a figure there and that the reading
	// wrote down what it saw, which is the alt text.
	//
	// It has to be matched before linkRE, and matched at all. Before, because
	// linkRE matches the bracket part of it and leaves the exclamation mark
	// standing, which is how the pdf of General Topology came to have a paragraph
	// reading "!Figure 2". And at all, because an image handed to the reference
	// check is a reference to a file, and the check is about anchors, so every one
	// of the 62 was counted as a cross reference pointing at nothing.
	imageRE = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]*)\)`)
)

// inline renders the inside of a paragraph or a heading.
func (r Renderer) inline(s string) string {
	s, ctl := r.controls(s)
	var esc []string
	s = mdEscapeRE.ReplaceAllStringFunc(s, func(m string) string {
		esc = append(esc, mdEscapeRE.FindStringSubmatch(m)[1])
		return escOpen + itoa(len(esc)-1) + escClose
	})
	s = escapeTeX(s)
	s = boldRE.ReplaceAllString(s, `\textbf{$1}`)
	s = emRE.ReplaceAllString(s, `$1\emph{$2}$3`)
	s = imageRE.ReplaceAllString(s, `\bfigure{$1}`)
	s = linkRE.ReplaceAllStringFunc(s, func(m string) string {
		p := linkRE.FindStringSubmatch(m)
		if r.Ref == nil {
			return p[1]
		}
		if out := r.Ref(p[2], p[1]); out != "" {
			return out
		}
		return p[1]
	})
	if r.Missing != nil {
		if runes := Missing(s); len(runes) > 0 {
			r.Missing(r.at(0), runes)
		}
	}
	s = Text(s)
	if len(esc) > 0 {
		s = escRestoreRE.ReplaceAllStringFunc(s, func(m string) string {
			var i int
			fmt.Sscanf(m, "\x00e%d\x00", &i)
			return literal(esc[i])
		})
	}
	// The control sequences go back last, after the character table has run, so
	// that the TeX this wrote is not read as prose and rewritten.
	if len(ctl) == 0 {
		return s
	}
	return ctlRE.ReplaceAllStringFunc(s, func(m string) string {
		var i int
		fmt.Sscanf(m, "\x00c%d\x00", &i)
		return ctl[i]
	})
}

// teXSpecial is the ten characters TeX reads rather than prints. The backslash
// and the braces are first, since replacing them after the others would escape
// the backslashes the others just wrote.
var teXSpecial = strings.NewReplacer(
	`\`, `\textbackslash{}`,
	`{`, `\{`,
	`}`, `\}`,
	`$`, `\$`,
	`&`, `\&`,
	`#`, `\#`,
	`%`, `\%`,
	`_`, `\_`,
	`~`, `\textasciitilde{}`,
	`^`, `\textasciicircum{}`,
)

func escapeTeX(s string) string { return teXSpecial.Replace(s) }

// literal is one character of prose the corpus escaped, written so TeX prints
// it. The star is the interesting one and the reason the whole placeholder
// dance exists: it has to survive the emphasis reader and then be printed.
func literal(ch string) string {
	switch ch {
	case `\`:
		return `\textbackslash{}`
	case `~`:
		return `\textasciitilde{}`
	case `^`:
		return `\textasciicircum{}`
	case `{`, `}`, `$`, `&`, `#`, `%`, `_`:
		return `\` + ch
	}
	return ch
}

// at is where a body line is in the corpus, in the file:line form an editor and
// a CI annotation both take.
func (r Renderer) at(line int) string {
	if r.File == "" {
		return fmt.Sprintf("line %d", line)
	}
	return fmt.Sprintf("%s:%d", r.File, r.Line+line-1)
}

// StripTitle takes the heading a file opens with off the front of its body,
// when that heading is the file naming itself.
//
// Every section file opens with its own title as a level two heading, every
// chapter front page with the chapter number and the chapter title, and every
// historical note with the words HISTORICAL NOTE. The document sets all three
// from the front matter, where they are right in every language and where an
// appendix has not had its Roman numeral eaten by an OCR, so the copy in the
// body would print twice.
//
// It stops at the first block that is not a level one or two heading, so a
// level two heading further down, which four files in the corpus have, stays
// where it is.
func StripTitle(body string) string {
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		m := headingRE.FindStringSubmatch(lines[i])
		if m == nil || len(m[1]) > 2 {
			break
		}
		i++
	}
	return strings.TrimLeft(strings.Join(lines[i:], "\n"), "\n")
}

// exercisesAnchorRE is the pointer the corpus writes at the foot of a § at the
// place the printing prints the exercises, "### Exercises {#alg-i-s1-exercises}"
// and a line under it linking the directory they are in.
var exercisesAnchorRE = regexp.MustCompile(`(?m)^#{1,6}[^\n]*\{#[a-z0-9-]+-exercises[^}]*\}\s*$`)

// StripExercisePointer takes that pointer off the end of a body.
//
// It is a link between two files of a repository and it has no meaning in a
// book: the book sets the exercises themselves, in the place the pointer sits,
// out of the files the pointer points at. Leaving it would print a heading
// saying Exercises followed by a sentence telling the reader to see the
// exercises, immediately above the exercises.
func StripExercisePointer(body string) string {
	loc := exercisesAnchorRE.FindStringIndex(body)
	if loc == nil {
		return body
	}
	return strings.TrimRight(body[:loc[0]], "\n") + "\n"
}
