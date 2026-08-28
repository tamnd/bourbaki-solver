package assemble

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Introduction assembles the Book's own introduction, the pages that stand
// before chapter I and belong to no chapter.
//
// Chapter is driven by the table of contents, and the table of contents holds
// chapters. Theory of Sets opens with seven pages that are in none of them:
// Bourbaki on what a proof is, why the Elements is formalized and what the
// axiomatic method buys. It is the part that says what the rest is for, and
// because nothing walked it the volume was read, assembled, tagged and
// translated with the first thing in the book missing.
//
// The pages are given rather than found, and Book.Introduction says why: a
// heading reading INTRODUCTION is also the running head of every page after the
// first, so looking for one finds seven beginnings.
func Introduction(in corpus.Introduction, pages map[int]corpus.PageFile) (Piece, error) {
	var parts []part
	for p := in.FirstPDFPage; p <= in.LastPDFPage; p++ {
		f, ok := pages[p]
		if !ok {
			return Piece{}, fmt.Errorf("the introduction runs pdf %d to %d and page %d is not read yet",
				in.FirstPDFPage, in.LastPDFPage, p)
		}
		// The number at the foot and the title at the head are both furniture.
		// Seven pages of furniture left in would put six bare numbers and six
		// repetitions of the word INTRODUCTION between the paragraphs.
		body, folio := corpus.CutFolio(f.Body)
		body = dropTitle(body, in.Title)
		if f.Meta.Folio > 0 {
			folio = f.Meta.Folio
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		parts = append(parts, part{page: p, label: f.Meta.PageLabel, folio: folio,
			method: f.Meta.Method, body: body})
	}
	if len(parts) == 0 {
		return Piece{}, fmt.Errorf("pdf pages %d to %d are empty", in.FirstPDFPage, in.LastPDFPage)
	}
	fillFolios(parts)

	blocks := gather(parts)
	body := "## " + in.Title + "\n\n" + strings.Join(blocks, "\n\n") + "\n"

	first, last := parts[0], parts[len(parts)-1]
	p := Piece{
		Body: body,
		Runs: []Run{{
			First: first.page, Last: last.page,
			FirstLabel: first.label, LastLabel: last.label,
			FirstFolio: first.folio, LastFolio: last.folio,
		}},
	}
	for _, q := range parts {
		p.Methods = append(p.Methods, q.method)
	}
	return p, nil
}

// gather is join for an introduction: the paragraphs of every page in order,
// with a paragraph broken by the end of a page put back together.
//
// join takes the following page's word for it, through the continues field
// extraction fills in from the indent of its first line. Not one of the 380
// pages of Theory of Sets carries that field, so there is no word to take, and
// the introduction would come out with a sentence cut in half at each of five
// page breaks: "incorrect use of intuition" ends page 7 and "or argument by
// analogy" opens page 8.
//
// So it is read off the text instead, and only where the text leaves nothing to
// read. A page whose last paragraph ends in mid clause is a page whose last
// paragraph was not finished, and a page opening on a lower case word is not
// opening a paragraph, because Bourbaki does not begin one that way. Where the
// page before ends on a full stop the indent is the only evidence there is, and
// nothing is guessed: the two stay apart, which is what the corpus already does
// with every other page of this volume.
func gather(parts []part) []string {
	var out []string
	for _, p := range parts {
		bs := split(p.body)
		if len(bs) == 0 {
			continue
		}
		if len(out) > 0 && runsOn(out[len(out)-1], bs[0]) {
			out[len(out)-1] = glue(out[len(out)-1], bs[0])
			bs = bs[1:]
		}
		out = append(out, bs...)
	}
	return out
}

// runsOn says the foot of one page and the head of the next are two halves of
// one paragraph, on the evidence of the words alone.
func runsOn(prev, next string) bool {
	if prev == "" || next == "" || itemOpen(next) {
		return false
	}
	// A heading is never half of anything, a display is set on lines of its own,
	// and a footnote definition stands on its own line. This is what joinable
	// refuses for the same reasons.
	for _, s := range []string{prev, next} {
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "$$") ||
			strings.HasSuffix(s, "$$") || noteRE.MatchString(s) {
			return false
		}
	}
	return open(prev) && lower(next)
}

// open says a paragraph was cut off rather than finished. A sentence of this
// printing ends on a full stop, a question mark or an exclamation mark, and the
// closing quotation mark or bracket that may follow one.
func open(s string) bool {
	r := []rune(strings.TrimRight(s, " "))
	for i := len(r) - 1; i >= 0; i-- {
		switch r[i] {
		case '”', '"', '’', ')', ']', '*', '_':
			continue // the marks that can close after the stop
		case '.', '!', '?':
			return false
		}
		return true
	}
	return false
}

// lower says a paragraph opens on a lower case word, which a paragraph of this
// printing does not do unless it is the second half of one.
func lower(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return unicode.IsLower(r)
		}
		if !unicode.IsSpace(r) && r != '*' && r != '“' && r != '‘' {
			return false // a paragraph opening on a sign or a formula
		}
	}
	return false
}

// dropTitle takes off a heading that is the introduction's own title.
//
// The first page carries it because it is the title. Every page after it
// carries it as the running head, which extraction normally files in the front
// matter and on page 12 of Theory of Sets wrote into the body instead. One
// title is wanted and Introduction writes it back on at the top, so all of them
// come off here.
//
// A title read as prose comes off too. Eleven of the notes to the reader open
// with the title set in caps and small caps, which the reading turns into an
// ordinary line rather than a heading: page 5 of Algebre I a III is the line
// "Mode d'emploi de ce traite" and then the first numbered paragraph. A line on
// its own that is the title is the title.
func dropTitle(body, title string) string {
	want := strings.ToLower(strings.TrimSpace(title))
	lines := strings.Split(body, "\n")
	out := lines[:0]
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.ToLower(strings.TrimSpace(strings.TrimLeft(s, "# "))) == want {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
