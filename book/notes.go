package book

import (
	"fmt"
	"regexp"
	"strings"
)

// A noteList is the footnotes of one XHTML document, in the order the text
// calls them.
//
// The numbers restart at 1 in every document and are not the numbers on the
// printed page. They cannot be: the printing numbers from 1 on each page and a
// § of General Topology runs to thirty pages, so the page numbers repeat inside
// one document, and an id has to be unique in the file it is in or the link
// takes the reader to whichever note the reading system picked. What the reader
// wants of a mark is to reach the note and get back, and both of those are
// links rather than numbers.
type noteList struct {
	n     int
	items []string
}

// call adds a note and hands back the mark that goes where the text called it.
//
// The mark and the note point at one another. epub:type says which is which, so
// a reading system that has a popup footnote shows the note in place instead of
// leaving the page, and one that has not follows the link like any other.
func (l *noteList) call(body string) string {
	l.n++
	num := itoa(l.n)
	l.items = append(l.items, fmt.Sprintf(
		"<div class=\"fn\" id=\"fn-%s\" epub:type=\"footnote\"><a class=\"fnback\" href=\"#fnref-%s\">%s</a> %s</div>\n",
		num, num, num, body))
	return fmt.Sprintf(
		"<sup class=\"fnmark\" id=\"fnref-%s\"><a epub:type=\"noteref\" href=\"#fn-%s\">%s</a></sup>",
		num, num, num)
}

// orphan adds a note with no mark, which is what a \footnotetext in the corpus
// is: the reading found the note at the foot of the page and did not find the
// call. It takes no number, because a number beside a note nothing points at
// says there is a mark to look for and there is not.
func (l *noteList) orphan(body string) {
	l.items = append(l.items, "<div class=\"fn\">"+body+"</div>\n")
}

// flush is the notes as they are set at the end of the document, under a rule,
// which is where the printing puts them on a page and the nearest an EPUB has.
//
// A note is a div and not a p because a note can hold a display. Two do in Set
// Theory alone: the note on the term denoted by "1" in III, § 3 sets the τ
// expression it is talking about, and note 47 of the historical note sets the
// trigonometric series that defines a set of uniqueness. A display is a div, a p
// cannot hold one, and a reading system given one closes the paragraph where the
// display starts and sets the rest of the note as a paragraph of its own.
func (l *noteList) flush() string {
	if len(l.items) == 0 {
		return ""
	}
	return "<aside class=\"notes\" epub:type=\"footnotes\">\n<hr class=\"notesrule\"/>\n" +
		strings.Join(l.items, "") + "</aside>\n"
}

// noteRefRE is a footnote call in the prose, with the space in front of it that
// the corpus writes and the printing does not.
//
// Bourbaki sets the mark snug against the word it belongs to, "prime numbers¹;",
// and the reading wrote "prime numbers [^1];" because that is where the eye put
// it. Left in, the space sets the mark adrift half an em from its word, which is
// how a printed page says the mark belongs to the next word instead.
//
// The label is letters and digits and nothing else on purpose. Written wider it
// swallows \[^1\], which is the corpus escaping a bracket it wants printed, and
// the escape is four characters of prose that has no note behind it.
var noteRefRE = regexp.MustCompile(`[ \t]?\[\^([A-Za-z0-9_-]{1,12})\]`)

// noteDefRE is a footnote definition, which stands at the head of its own line.
var noteDefRE = regexp.MustCompile(`^\[\^([A-Za-z0-9_-]{1,12})\]:[ \t]*(.*)$`)

// footnotes takes the Markdown footnote definitions out of a masked body and
// puts each note's text where the note is called, as a \footnote.
//
// Most of the corpus's footnotes are written the way Markdown writes one: a
// [^1] where the printing has the mark, and a [^1]: line further down holding
// the note. That is what the OCR normaliser produces out of the (*) and the (†)
// the pages actually carry, and it is the only one of the three spellings in the
// corpus that records which note belongs to which call. content/ has 436
// definitions and 343 calls in it, over four languages.
//
// Neither builder could read it. A definition line is not a heading and not a
// display, so it came through as an ordinary paragraph, and a call is four
// characters of prose. 38 of the 129 built volumes printed a marker literally,
// 905 times over: the EPUB set a paragraph reading "[^1]: Notably Democritus,
// ..." and the pdf, having escaped the caret on the way past, set
// "[\textasciicircum{}1]: Notably Democritus, ...". A footnote printed as its own
// source in the middle of a page is worse than a footnote dropped, because the
// reader has to work out what it is before deciding to ignore it.
//
// The rewrite is to \footnote and not to anything new, because the corpus
// already writes 118 inline \footnote{...} of its own and both builders have a
// reading for one. So nothing downstream of here learns a third thing: the
// definitions come out, the text goes to the call, and the note is set by
// whichever path the volume is being built along.
//
// A call with no definition behind it is left exactly as it stands. There are 65
// of those and not one is a footnote: they are a superscript the reading lost,
// $\boldsymbol{\gamma}$[^1]$_G$ and its like in Topological Vector Spaces V,
// where a (p) over a letter came back as a footnote call. Inventing a note for
// one would put an empty note at the foot of the page and take the reader to it.
//
// A label called twice gets the note twice, at both marks. There are six of
// those and three of them are the same defect in a second language: two are a
// mark the reading doubled, "self-evident [^39].[^39]" in the historical note to
// Set Theory, and one is Algebra IV, where two sections that each printed a note
// 1 were assembled into one file and only one definition survived. Setting the
// note at both marks is wrong in the first case and right in the second, and it
// is the way round that leaves nothing printing its own brackets.
func footnotes(masked string) string {
	if !strings.Contains(masked, "[^") {
		return masked
	}
	lines := strings.Split(masked, "\n")
	text, at := map[string]string{}, map[string]int{}
	kept := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		m := noteDefRE.FindStringSubmatch(lines[i])
		if m == nil {
			kept = append(kept, lines[i])
			continue
		}
		// A note written over more than one line runs on until a blank line or
		// until the next definition, which is Markdown's own rule for one. The
		// corpus has a single note written that way, on the ordering of N in the
		// Summary of Results of Set Theory, and 53 definitions that sit directly
		// under another definition and are not a continuation of it.
		body := []string{m[2]}
		for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && !noteDefRE.MatchString(lines[i+1]) {
			i++
			body = append(body, strings.TrimSpace(lines[i]))
		}
		text[m[1]] = strings.TrimSpace(strings.Join(body, " "))
		at[m[1]] = len(kept)
	}
	if len(text) == 0 {
		return masked
	}
	called := map[string]bool{}
	for i, line := range kept {
		kept[i] = noteRefRE.ReplaceAllStringFunc(line, func(m string) string {
			label := noteRefRE.FindStringSubmatch(m)[1]
			body, ok := text[label]
			if !ok {
				return m
			}
			called[label] = true
			return `\footnote{` + body + `}`
		})
	}
	// A note nothing calls goes to the end of the text it was printed under,
	// which is the last thing written above it. There is one in the corpus. The
	// alternative is to drop it, and the note is the part of a footnote worth
	// keeping: a reader who never sees the mark has lost a marker, and a reader
	// who never sees the note has lost the sentence it carried.
	for label, body := range text {
		if called[label] {
			continue
		}
		for i := at[label] - 1; i >= 0; i-- {
			if strings.TrimSpace(kept[i]) == "" {
				continue
			}
			kept[i] += `\footnote{` + body + `}`
			break
		}
	}
	return strings.Join(kept, "\n")
}
