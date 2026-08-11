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

// Scope is what a statement's number is counted within. Bourbaki does not
// number everything the same way, and a label that assumed it did would put two
// different statements at the same address.
type Scope int

const (
	// ScopeSection is numbered straight through the §. A Proposition 6 is the
	// sixth Proposition of its §, whatever no. it stands in.
	ScopeSection Scope = iota
	// ScopeSubsec is numbered within the no. and starts again at the next one.
	ScopeSubsec
	// ScopeParent is numbered under the statement it follows and starts again
	// at the next one.
	ScopeParent
)

// Scope of a Kind, measured on the 505 pages of Algebra chapter VIII.
//
// Definitions, Propositions, Theorems, Lemmas and the one Scholium run straight
// through their §: no § of the chapter numbers two of any of them alike.
//
// Corollaries do not. § 5 no. 3 prints Corollary 1 to Corollary 5 and then a
// Corollary 1 again, because a Corollary is numbered under the statement it
// hangs from and the second run hangs from a different Proposition. Counting
// them within the no. instead leaves that pair on one address, and counting
// them within the § leaves twelve of the twenty-five sections with a clash.
// Under the statement they follow, all 112 numbered Corollaries of the chapter
// are distinct and none is left without a parent.
//
// Remarks and Examples are numbered within the no.: § 1 prints an Example 5 in
// no. 1 and another in no. 2, and § 20 a Remark 3 in no. 3 and a Remark 1 in
// no. 8.
func (k Kind) Scope() Scope {
	switch k {
	case KindCorollary:
		return ScopeParent
	case KindRemark, KindExample:
		return ScopeSubsec
	}
	return ScopeSection
}

// Ref locates a statement in the corpus.
//
// Which of the fields under Kind are set follows the Kind's Scope, and a
// statement Bourbaki leaves unnumbered is located by the no. it stands in and
// its place in that no. whatever its Kind.
type Ref struct {
	Book    string // "alg"
	Chapter string // "VIII"
	Section int    // 1

	// Appendix says the section is an Appendix rather than a §. The two are
	// numbered separately and both from one, so Appendix 1 of chapter VIII and
	// its § 1 are different sections with the same number, and without this
	// their statements would share a label.
	Appendix bool

	Kind   Kind
	Number int // 0 when the statement is unnumbered

	// ParentKind and ParentNumber are the statement this one is numbered
	// under, set for a numbered Corollary: Corollary 2 of Theorem 1.
	ParentKind   Kind
	ParentNumber int

	Subsec     int // the no. it stands in, set when the Kind is counted within one and for every unnumbered statement
	Occurrence int // 1-based place within Subsec, only when Number is 0
}

// Label builds the permanent full label for a statement, the string that
// tags/tags maps a tag to.
//
//	alg-viii-s1-prop-6        a numbered Proposition
//	alg-viii-s5-thm-1-cor-2   Corollary 2 of Theorem 1
//	alg-viii-s1-n2-exa-5      Example 5 of no. 2
//	alg-viii-s1-n3-cor-1      the first unnumbered Corollary in no. 3
//	alg-viii-a2-prop-3        a numbered Proposition of Appendix 2
//
// The label says where the book puts the statement and nothing about where the
// file puts it, so it survives re-extraction, re-assembly and any amount of
// editing. That is what makes it worth pointing a permanent tag at.
func (r Ref) Label() string {
	base := r.SectionLabel()
	if r.Number == 0 {
		return fmt.Sprintf("%s-n%d-%s-%d", base, r.Subsec, r.Kind, r.Occurrence)
	}
	if r.ParentKind != "" {
		return fmt.Sprintf("%s-%s-%d-%s-%d", base, r.ParentKind, r.ParentNumber, r.Kind, r.Number)
	}
	if r.Subsec > 0 {
		return fmt.Sprintf("%s-n%d-%s-%d", base, r.Subsec, r.Kind, r.Number)
	}
	return fmt.Sprintf("%s-%s-%d", base, r.Kind, r.Number)
}

// SectionLabel is the label of the section the statement sits in, which is what
// every label of that section is built on: alg-viii-s1, alg-viii-a2.
func (r Ref) SectionLabel() string {
	in := "s"
	if r.Appendix {
		in = "a"
	}
	return fmt.Sprintf("%s-%s-%s%d", r.Book, strings.ToLower(r.Chapter), in, r.Section)
}

var (
	numberedLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-(?P<in>[sa])(?P<sec>\d+)-(?P<kind>[a-z]+)-(?P<num>\d+)$`)
	childLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-(?P<in>[sa])(?P<sec>\d+)-(?P<pkind>[a-z]+)-(?P<pnum>\d+)-(?P<kind>[a-z]+)-(?P<num>\d+)$`)
	subsecLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-(?P<in>[sa])(?P<sec>\d+)-n(?P<no>\d+)-(?P<kind>[a-z]+)-(?P<num>\d+)$`)
)

// ParseLabel is the inverse of Ref.Label.
//
// One shape carries two meanings and is read by the Kind's Scope:
// alg-viii-s1-n2-exa-5 is Example 5 of no. 2, because an Example is numbered
// within the no., and alg-viii-s1-n2-cor-5 is the fifth unnumbered Corollary of
// no. 2, because a Corollary is not. The two cannot be told apart from the
// string alone, and they do not have to be: whichever way it is read, Label
// gives the same string back, so a label is still a key. What it means is that
// an unnumbered Remark and a numbered one in the same no. can collide, which is
// why assembly refuses to write a section with two statements at one label.
func ParseLabel(label string) (Ref, error) {
	if m := childLabelRe.FindStringSubmatch(label); m != nil {
		sec, _ := strconv.Atoi(m[4])
		pnum, _ := strconv.Atoi(m[6])
		num, _ := strconv.Atoi(m[8])
		return Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Appendix: m[3] == "a", Section: sec,
			ParentKind: Kind(m[5]), ParentNumber: pnum, Kind: Kind(m[7]), Number: num,
		}, nil
	}
	if m := subsecLabelRe.FindStringSubmatch(label); m != nil {
		sec, _ := strconv.Atoi(m[4])
		no, _ := strconv.Atoi(m[5])
		n, _ := strconv.Atoi(m[7])
		r := Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Appendix: m[3] == "a", Section: sec,
			Kind: Kind(m[6]), Subsec: no,
		}
		if r.Kind.Scope() == ScopeSubsec {
			r.Number = n
		} else {
			r.Occurrence = n
		}
		return r, nil
	}
	if m := numberedLabelRe.FindStringSubmatch(label); m != nil {
		sec, _ := strconv.Atoi(m[4])
		num, _ := strconv.Atoi(m[6])
		return Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Appendix: m[3] == "a", Section: sec,
			Kind: Kind(m[5]), Number: num,
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
