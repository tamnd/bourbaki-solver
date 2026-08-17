package assemble

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The seven pages Theory of Sets opens with, in the shape the reading left
// them: the title on the first page, the running head filed in the front matter
// of the second, the same head written into the body of the third because that
// page was read differently, the printed number bare at the foot of some of
// them and dropped from others, and five sentences cut in half by a page break.
func introPages(t *testing.T) map[int]corpus.PageFile {
	t.Helper()
	pages := []struct {
		n     int
		head  string
		folio int
		body  string
	}{
		{15, "", 7, "# INTRODUCTION\n\nEver since the time of the Greeks, mathematics has involved proof; and it is even doubted by some whether"},
		{16, "INTRODUCTION", 0, "proof, in the sense the Greeks gave to this word, is to be found outside mathematics.\n\nBy analysis of the mechanism of proofs it has been possible to discern the structure underlying\n\n8"},
		{17, "", 0, "# INTRODUCTION\n\nboth vocabulary and syntax.\n\nThe axiomatic method is nothing but this art.\n\n9"},
	}
	out := map[int]corpus.PageFile{}
	for _, p := range pages {
		out[p.n] = corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "ens-i-iv", PDFPage: p.n, RunningHead: p.head, Folio: p.folio,
			Method: corpus.MethodOCR,
		}, Body: p.body}
	}
	return out
}

func TestIntroduction(t *testing.T) {
	in := corpus.Introduction{Title: "INTRODUCTION", Page: 7, FirstPDFPage: 15, LastPDFPage: 17}
	p, err := Introduction(in, introPages(t))
	if err != nil {
		t.Fatal(err)
	}
	want := "## INTRODUCTION\n\n" +
		"Ever since the time of the Greeks, mathematics has involved proof; and it is even doubted by some whether proof, in the sense the Greeks gave to this word, is to be found outside mathematics.\n\n" +
		"By analysis of the mechanism of proofs it has been possible to discern the structure underlying both vocabulary and syntax.\n\n" +
		"The axiomatic method is nothing but this art.\n"
	if p.Body != want {
		t.Errorf("the introduction came out as\n%s\nwant\n%s", p.Body, want)
	}
	if len(p.Runs) != 1 {
		t.Fatalf("the introduction is %d runs", len(p.Runs))
	}
	// The folio of the last two pages is not in their front matter, and page 9
	// prints none at all, so both are counted along from page 7.
	if r := p.Runs[0]; r.First != 15 || r.Last != 17 || r.FirstFolio != 7 || r.LastFolio != 9 {
		t.Errorf("the run is %+v", r)
	}
	if p.Extraction() != "ocr" {
		t.Errorf("extraction = %q", p.Extraction())
	}
}

// A page nobody has read stops the run rather than being passed over. Seven
// pages assembled out of six is a file that looks whole and is not.
func TestIntroductionRefusesAPageNobodyHasRead(t *testing.T) {
	pages := introPages(t)
	delete(pages, 16)
	in := corpus.Introduction{Title: "INTRODUCTION", Page: 7, FirstPDFPage: 15, LastPDFPage: 17}
	_, err := Introduction(in, pages)
	if err == nil {
		t.Fatal("the introduction assembled without page 16")
	}
	if !strings.Contains(err.Error(), "page 16 is not read yet") {
		t.Errorf("it stopped with %v", err)
	}
}

func TestRunsOn(t *testing.T) {
	cases := []struct {
		prev, next string
		want       bool
	}{
		// The evidence is the words: a paragraph that stops in mid clause and
		// one that opens on a lower case word are two halves of one.
		{"incorrect use of intuition", "or argument by analogy. In practice,", true},
		{"which have their historical origin", "in another specialization of the", true},
		{"(comparable,", "for example, to the following", true},
		// A finished sentence and a new paragraph stay apart. The indent is the
		// only evidence there is here and this volume records none, so nothing
		// is guessed.
		{"cardinal numbers.", "Thus, written in accordance with", false},
		{"cardinal numbers.", "thus, written in accordance with", false},
		{"the whole of it!", "he wrote", false},
		{"is (see chapter I.)", "so it follows", false},
		// The shapes that are never half of a paragraph.
		{"the assembly", "$$x = y$$", false},
		{"$$x = y$$", "is a term", false},
		{"a heading follows", "# INTRODUCTION", false},
		{"# INTRODUCTION", "Ever since the time of the Greeks", false},
		{"a note follows", "[^1]: The word is used here", false},
		{"the exercises open", "1) Let A be a ring", false},
		{"", "or argument by analogy", false},
	}
	for _, c := range cases {
		if got := runsOn(c.prev, c.next); got != c.want {
			t.Errorf("runsOn(%q, %q) = %v, want %v", c.prev, c.next, got, c.want)
		}
	}
}

func TestDropTitle(t *testing.T) {
	cases := []struct {
		body, want string
	}{
		{"# INTRODUCTION\n\nEver since the Greeks", "\nEver since the Greeks"},
		{"## Introduction\n\ntext", "\ntext"},
		{"text\n\n# INTRODUCTION\n\nmore", "text\n\n\nmore"},
		// A heading that is not the title stays, and so does the word in prose.
		{"# CHAPTER I\n\ntext", "# CHAPTER I\n\ntext"},
		{"the INTRODUCTION says", "the INTRODUCTION says"},
	}
	for _, c := range cases {
		if got := dropTitle(c.body, "INTRODUCTION"); got != c.want {
			t.Errorf("dropTitle(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}
