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

// The French printing states the same results in its own words, and they
// resolve to the same slugs, because a Lemme and a Lemma are one lemma of the
// Éléments printed twice and the slug is what its permanent tag is named after.
var frKindByHeading = map[string]Kind{
	"définition":   KindDefinition,
	"définitions":  KindDefinition,
	"proposition":  KindProposition,
	"propositions": KindProposition,
	"théorème":     KindTheorem,
	"théorèmes":    KindTheorem,
	"lemme":        KindLemma,
	"lemmes":       KindLemma,
	"corollaire":   KindCorollary,
	"corollaires":  KindCorollary,
	"remarque":     KindRemark,
	"remarques":    KindRemark,
	"exemple":      KindExample,
	"exemples":     KindExample,
	"scholie":      KindScholium,
	"exercice":     KindExercise,
}

// KindFromHeading resolves the printed word, in any case and in either
// language, to a Kind.
func KindFromHeading(s string) (Kind, bool) {
	w := strings.ToLower(strings.TrimSpace(s))
	if k, ok := kindByHeading[w]; ok {
		return k, true
	}
	k, ok := frKindByHeading[w]
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

// HeadingIn returns the word printed for this Kind in a language. It is what a
// reader sees over a statement, and nothing but that: the label under it, and
// so the tag, is the same in every language.
func (k Kind) HeadingIn(lang string) string {
	if lang != "fr" {
		return k.Heading()
	}
	switch k {
	case KindDefinition:
		return "Définition"
	case KindTheorem:
		return "Théorème"
	case KindLemma:
		return "Lemme"
	case KindCorollary:
		return "Corollaire"
	case KindRemark:
		return "Remarque"
	case KindExample:
		return "Exemple"
	case KindScholium:
		return "Scholie"
	case KindExercise:
		return "Exercice"
	case KindEquation:
		return "Formule"
	}
	// Proposition is spelt the same in both.
	return k.Heading()
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

// ChapterOrder is RomanOrder on a volume that prints chapters and the number
// itself on a volume that does not.
//
// Three things in this corpus are bound as a volume and divided into no
// chapters. Elements of the History of Mathematics is one, a set of
// twenty eight notes under no chapter heading at all, and the two fascicules of
// Varietes differentielles are the others. pagemap gives each of them a span
// named with an arabic number, "1" for the whole of the history and "1" and "2"
// for the two fascicules, and assemble writes the sections under a directory of
// that name, so content/en/hist/1 is where the notes are.
//
// Everything that walks content/ in the book's own order therefore meets a
// chapter directory that is not a roman numeral, and refusing it stops the walk
// on the whole corpus rather than on the one volume. Reading it as the number
// it is puts those volumes in the order they are bound, which is the order the
// book has, and leaves every volume that does print chapters exactly as it was.
func ChapterOrder(s string) (int, error) {
	if n, err := RomanOrder(s); err == nil {
		return n, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("not a chapter numeral: %q", s)
	}
	return n, nil
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

	// Div says the statement belongs to the divisibility numbering, which is a
	// second run of numbers alongside the ordinary one inside the same §.
	//
	// Chapter VI of Algebra 4 to 7 states the theory of ordered groups twice.
	// It proves each result in additive notation and then states it again in
	// the multiplicative notation divisibility is written in, and it marks the
	// second one by putting DIV in the head: "PROPOSITION 11 (DIV). —". Both
	// runs are numbered from one in the same §, so § 1 has two Proposition 7
	// and two Corollary 5, and without this they share a label and the chapter
	// refuses to assemble. 27 statements of that chapter are marked this way.
	//
	// It is a mark on the head and not a name, which is why it is not carried
	// through as one: "COROLLARY 2 (DIV) (Euclid's lemma)" has both, and the
	// name is Euclid's lemma.
	Div bool

	// Repeated says the printing gave this statement a number it had already
	// given to an earlier statement of the same kind in the same §, and that this
	// is the later of the two.
	//
	// It is a misprint and not a second numbering, which is what tells it apart
	// from Div. § 3 of chapter III of Groupes et algebres de Lie prints Definition
	// 7 at no. 7, on page 139 of the volume, and prints DEFINITION 7 again at no.
	// 12, on page 153, and then goes on to Definition 8. Both page images say so.
	// Nothing in the volume cites the second one, and the two citations that
	// matter point the other way: page 140 cites "la condition a) de la def. 7",
	// which is the first one, and page 153 cites "le cas general de la def. 8",
	// which is the one after the repeat. So the numbering the book uses to cite by
	// is 1 to 8 with no gap, and the second Definition 7 sits outside it.
	//
	// The label is what gives way, since the heading has to keep printing what the
	// page prints. The first statement keeps lie-iii-s3-def-7 and the later one is
	// lie-iii-s3-def-7-bis, so no citation moves, Definition 8 stays where it is,
	// and the repeated one is still addressable and still carries a tag.
	//
	// Shifting the later one to 8 and everything after it up by one was the other
	// way, and it is wrong here: it would take the label lie-iii-s3-def-8 off the
	// statement page 153 cites by that name.
	Repeated bool

	Kind   Kind
	Number int // 0 when the statement is unnumbered

	// ParentKind and ParentNumber are the statement this one is numbered
	// under, set for a numbered Corollary: Corollary 2 of Theorem 1.
	ParentKind   Kind
	ParentNumber int

	Subsec int // the no. it stands in, set when the Kind is counted within one and for every unnumbered statement

	// Occurrence is the 1-based place of an unnumbered statement among its own
	// kind in the no., and is set only when Number is 0.
	//
	// It counts past the numbered ones rather than starting again at 1, because
	// the two end up in the same shape of label and a no. can hold both: § 5 no.
	// 1 of chapter VII of Lie 7 to 9 prints "Remarks. 1) ... 2) ..." and then,
	// four paragraphs later, a bare "Remark." That last one is the third remark
	// of the no. and is named so.
	Occurrence int
}

// Label builds the permanent full label for a statement, the string that
// tags/tags maps a tag to.
//
//	alg-viii-s1-prop-6        a numbered Proposition
//	alg-viii-s5-thm-1-cor-2   Corollary 2 of Theorem 1
//	alg-viii-s1-n2-exa-5      Example 5 of no. 2
//	alg-viii-s1-n3-cor-1      the first unnumbered Corollary in no. 3
//	alg-viii-a2-prop-3        a numbered Proposition of Appendix 2
//	alg-vi-s1-div-prop-11     Proposition 11 (DIV) of § 1
//	lie-iii-s3-def-7-bis      the second statement the printing numbered 7
//
// The label says where the book puts the statement and nothing about where the
// file puts it, so it survives re-extraction, re-assembly and any amount of
// editing. That is what makes it worth pointing a permanent tag at.
//
// The divisibility mark goes next to the section rather than next to the kind,
// because that is where it belongs: it is a second numbering inside the § and
// not a property of the one statement, so every label under it, the Corollary
// as much as the Proposition it hangs from, sits inside the same div.
//
// The repeat mark goes at the far end for the opposite reason: it is a property
// of the one statement, it says nothing about anything numbered under it, and
// putting it last leaves every other label in the § exactly as it was. See
// Ref.Repeated.
func (r Ref) Label() string {
	base := r.SectionLabel()
	if r.Div {
		base += "-div"
	}
	bis := ""
	if r.Repeated {
		bis = "-bis"
	}
	if r.Number == 0 {
		return fmt.Sprintf("%s-n%d-%s-%d%s", base, r.Subsec, r.Kind, r.Occurrence, bis)
	}
	if r.ParentKind != "" {
		return fmt.Sprintf("%s-%s-%d-%s-%d%s", base, r.ParentKind, r.ParentNumber, r.Kind, r.Number, bis)
	}
	if r.Subsec > 0 {
		return fmt.Sprintf("%s-n%d-%s-%d%s", base, r.Subsec, r.Kind, r.Number, bis)
	}
	return fmt.Sprintf("%s-%s-%d%s", base, r.Kind, r.Number, bis)
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
	// The optional div group is the divisibility numbering of chapter VI of
	// Algebra 4 to 7. It cannot be confused with a kind, because a kind is
	// always followed by a number and div never is.
	//
	// The optional bis group at the end is a number the printing gave twice, and
	// it cannot be confused with anything either: it is the one part of a label
	// that is not followed by a number. See Ref.Repeated.
	numberedLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-(?P<in>[sa])(?P<sec>\d+)(?P<div>-div)?-(?P<kind>[a-z]+)-(?P<num>\d+)(?P<bis>-bis)?$`)
	childLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-(?P<in>[sa])(?P<sec>\d+)(?P<div>-div)?-(?P<pkind>[a-z]+)-(?P<pnum>\d+)-(?P<kind>[a-z]+)-(?P<num>\d+)(?P<bis>-bis)?$`)
	subsecLabelRe = regexp.MustCompile(
		`^(?P<book>[a-z0-9]+)-(?P<ch>[ivxlcdm]+)-(?P<in>[sa])(?P<sec>\d+)(?P<div>-div)?-n(?P<no>\d+)-(?P<kind>[a-z]+)-(?P<num>\d+)(?P<bis>-bis)?$`)
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
		pnum, _ := strconv.Atoi(m[7])
		num, _ := strconv.Atoi(m[9])
		return Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Appendix: m[3] == "a", Section: sec,
			Div:        m[5] != "",
			ParentKind: Kind(m[6]), ParentNumber: pnum, Kind: Kind(m[8]), Number: num,
			Repeated: m[10] != "",
		}, nil
	}
	if m := subsecLabelRe.FindStringSubmatch(label); m != nil {
		sec, _ := strconv.Atoi(m[4])
		no, _ := strconv.Atoi(m[6])
		n, _ := strconv.Atoi(m[8])
		r := Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Appendix: m[3] == "a", Section: sec,
			Div: m[5] != "", Kind: Kind(m[7]), Subsec: no, Repeated: m[9] != "",
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
		num, _ := strconv.Atoi(m[7])
		return Ref{
			Book: m[1], Chapter: strings.ToUpper(m[2]), Appendix: m[3] == "a", Section: sec,
			Div: m[5] != "", Kind: Kind(m[6]), Number: num, Repeated: m[8] != "",
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
