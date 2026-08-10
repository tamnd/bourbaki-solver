// Package corpus models the structure of the Bourbaki corpus: books, chapters,
// sections, statements, and the permanent tags that identify them.
package corpus

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Kind is the sort of numbered statement Bourbaki uses.
type Kind string

const (
	KindDefinition  Kind = "def"
	KindProposition Kind = "prop"
	KindTheorem     Kind = "thm"
	KindLemma       Kind = "lem"
	KindCorollary   Kind = "cor"
	KindRemark      Kind = "rem"
	KindExample     Kind = "exa"
	KindScholium    Kind = "sch"
	KindExercise    Kind = "ex"
	KindEquation    Kind = "eq"
)

// kindByHeading maps the word Bourbaki prints to the slug we store.
var kindByHeading = map[string]Kind{
	"definition":  KindDefinition,
	"definitions": KindDefinition,
	"proposition": KindProposition,
	"theorem":     KindTheorem,
	"lemma":       KindLemma,
	"corollary":   KindCorollary,
	"remark":      KindRemark,
	"remarks":     KindRemark,
	"example":     KindExample,
	"examples":    KindExample,
	"scholium":    KindScholium,
	"exercise":    KindExercise,
}

// KindFromHeading resolves the printed word, in any case, to a Kind.
func KindFromHeading(s string) (Kind, bool) {
	k, ok := kindByHeading[strings.ToLower(strings.TrimSpace(s))]
	return k, ok
}

// Heading returns the singular word printed in the book for this Kind.
func (k Kind) Heading() string {
	switch k {
	case KindDefinition:
		return "Definition"
	case KindProposition:
		return "Proposition"
	case KindTheorem:
		return "Theorem"
	case KindLemma:
		return "Lemma"
	case KindCorollary:
		return "Corollary"
	case KindRemark:
		return "Remark"
	case KindExample:
		return "Example"
	case KindScholium:
		return "Scholium"
	case KindExercise:
		return "Exercise"
	case KindEquation:
		return "Equation"
	}
	return string(k)
}

// Chapters of the Éléments are numbered in Roman. We keep them uppercase in
// identifiers and lowercase in labels, mirroring the Stacks Project's
// all-lowercase label convention.
var romanValue = map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}

var romanRe = regexp.MustCompile(`^[IVXLCDM]+$`)

// RomanOrder converts a Roman chapter numeral to its integer value, so that
// chapters sort I, II, III, VIII rather than lexicographically.
func RomanOrder(s string) (int, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !romanRe.MatchString(s) {
		return 0, fmt.Errorf("not a roman numeral: %q", s)
	}
	total, prev := 0, 0
	for i := len(s) - 1; i >= 0; i-- {
		v := romanValue[s[i]]
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	return total, nil
}

// Ref locates a statement in the corpus. Subsec and Occurrence are only used
// for statements Bourbaki leaves unnumbered, such as a bare "Corollary".
type Ref struct {
	Book       string // "alg"
	Chapter    string // "VIII"
	Section    int    // 1
	Kind       Kind
	Number     int // 0 when the statement is unnumbered
	Subsec     int // the "no." it sits in, only meaningful when Number is 0
	Occurrence int // 1-based ordinal within Subsec, only when Number is 0
}

// Label builds the permanent full label for a statement, the string that
// tags/tags maps a tag to.
//
//	alg-viii-s1-prop-6      a numbered Proposition
//	alg-viii-s1-n3-cor-1    the first unnumbered Corollary in no. 3
func (r Ref) Label() string {
	base := fmt.Sprintf("%s-%s-s%d", r.Book, strings.ToLower(r.Chapter), r.Section)
	if r.Number > 0 {
		return fmt.Sprintf("%s-%s-%d", base, r.Kind, r.Number)
	}
	return fmt.Sprintf("%s-n%d-%s-%d", base, r.Subsec, r.Kind, r.Occurrence)
}

var (
	numberedLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-s(?P<sec>\d+)-(?P<kind>[a-z]+)-(?P<num>\d+)$`)
	unnumberedLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-s(?P<sec>\d+)-n(?P<no>\d+)-(?P<kind>[a-z]+)-(?P<occ>\d+)$`)
)

// ParseLabel is the inverse of Ref.Label.
func ParseLabel(label string) (Ref, error) {
	if m := unnumberedLabelRe.FindStringSubmatch(label); m != nil {
		sec, _ := strconv.Atoi(m[3])
		no, _ := strconv.Atoi(m[4])
		occ, _ := strconv.Atoi(m[6])
		return Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Section: sec,
			Kind: Kind(m[5]), Subsec: no, Occurrence: occ,
		}, nil
	}
	if m := numberedLabelRe.FindStringSubmatch(label); m != nil {
		sec, _ := strconv.Atoi(m[3])
		num, _ := strconv.Atoi(m[5])
		return Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Section: sec,
			Kind: Kind(m[4]), Number: num,
		}, nil
	}
	return Ref{}, fmt.Errorf("malformed label: %q", label)
}

// PageLabel is a Bourbaki page reference such as "A VIII.13": the Book's
// letter, the chapter in Roman, and the page within that chapter. Bourbaki
// cross-references are page-based, so this is a primary key, not decoration.
type PageLabel struct {
	Book    string // "A" for Algebra
	Chapter string // "VIII"
	Page    int    // 13
}

// The three volumes in scope print the same label three different ways, so the
// separators are deliberately loose:
//
//	A VIII.13    Algebra chapter 8, 2023, Springer Nature
//	A.IV.3       Algebra chapters 4 to 7, 2003, Springer, recto
//	A. IV. 2     the same volume, verso
//	A.V . 36     the same volume, where the scan spaced it oddly
//
// A comma is not accepted between the Book and the chapter, because that is how
// a prose cross-reference reads ("Set Theory, III, § 4") and those are parsed
// elsewhere.
var pageLabelRe = regexp.MustCompile(`\b(?P<book>[A-Z]{1,3})[.\s]\s*(?P<ch>[IVXLCDM]+)\s*[.,]\s*(?P<p>\d{1,4})\b`)

// ParsePageLabel finds a page label anywhere in s, as it appears in a running
// head such as "A VIII.8" or inside a cross-reference.
func ParsePageLabel(s string) (PageLabel, bool) {
	m := pageLabelRe.FindStringSubmatch(s)
	if m == nil {
		return PageLabel{}, false
	}
	p, err := strconv.Atoi(m[3])
	if err != nil {
		return PageLabel{}, false
	}
	return PageLabel{Book: m[1], Chapter: m[2], Page: p}, true
}

// SectionLocator is the other kind of running head, "§ 6.5", meaning § 6 no. 5.
// The 1998 printing of chapters I to III carries no page label at all, only the
// chapter numeral on one side and this locator on the other, so for that volume
// it is the only anchor the page map has to work with.
type SectionLocator struct {
	Section int
	Subsec  int // 0 when the head prints only the section
}

var sectionLocatorRe = regexp.MustCompile(`§\s*(?P<sec>\d{1,2})(?:\s*\.\s*(?P<no>\d{1,2}))?`)

// ParseSectionLocator finds a section locator anywhere in s.
func ParseSectionLocator(s string) (SectionLocator, bool) {
	m := sectionLocatorRe.FindStringSubmatch(s)
	if m == nil {
		return SectionLocator{}, false
	}
	sec, err := strconv.Atoi(m[1])
	if err != nil || sec == 0 {
		return SectionLocator{}, false
	}
	loc := SectionLocator{Section: sec}
	if m[2] != "" {
		loc.Subsec, _ = strconv.Atoi(m[2])
	}
	return loc, true
}

func (l SectionLocator) String() string {
	if l.Subsec > 0 {
		return fmt.Sprintf("§%d.%d", l.Section, l.Subsec)
	}
	return fmt.Sprintf("§%d", l.Section)
}

func (p PageLabel) String() string {
	return fmt.Sprintf("%s %s.%d", p.Book, p.Chapter, p.Page)
}

// Valid reports whether the label is well formed and its chapter is a real
// Roman numeral.
func (p PageLabel) Valid() bool {
	if p.Book == "" || p.Page <= 0 {
		return false
	}
	_, err := RomanOrder(p.Chapter)
	return err == nil
}
