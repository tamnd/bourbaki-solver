package share

import (
	"fmt"
	"strings"
	"testing"
)

// prose is a page's worth of words, distinct per page, so that a shingle taken
// from one page is not found on another by accident.
func prose(page int) string {
	var b strings.Builder
	for i := range 40 {
		fmt.Fprintf(&b, "the assembly %s of weight %s is a term in the theory of page %s. ",
			word(page*100+i), word(page*100+i+1), word(page))
	}
	return b.String()
}

var syllables = []string{"al", "be", "ga", "de", "ep", "ze", "et", "th", "io", "ka"}

func word(n int) string {
	s := ""
	for n > 0 || s == "" {
		s += syllables[n%len(syllables)]
		n /= len(syllables)
	}
	return s
}

func section() Printed {
	return Printed{
		Section: 1,
		Title:   "Terms and relations",
		Numbers: []Numbered{{No: 1, Title: "Signs and assemblies"}, {No: 2, Title: "Criteria of substitution"}},
		Pages: []PrintedPage{
			{PDFPage: 22, Text: "### 1. SIGNS AND ASSEMBLIES\n\n" + prose(22) + "\n\nCF1. If $A$ is a relation.\n"},
			{PDFPage: 23, Text: prose(23) + "\n\n### 2. CRITERIA OF SUBSTITUTION\n\n" + prose(24) + "\n\nC1 (*Syllogism*). Let $A$ be a relation.\n"},
		},
	}
}

func target() Target { return Target{Book: "sets", Chapter: 1, Section: 1} }

func whole() string {
	return "## 1. TERMS AND RELATIONS\n\n### 1. SIGNS AND ASSEMBLIES\n\n" + prose(22) +
		"\n\nCF1. If $A$ is a relation.\n\n" + prose(23) +
		"\n\n## 2. CRITERIA OF SUBSTITUTION\n\n" + prose(24) +
		"\n\nC1. Let $A$ be a relation.\n"
}

func TestAnImportOfTheWholeSectionPasses(t *testing.T) {
	r := Audit(target(), whole(), section())
	if !r.OK() {
		t.Fatalf("want a pass, got %d hard findings: %v", r.Hard(), r.Findings)
	}
	if r.Numbers != 2 || r.Labels != 2 || r.Pages != 2 {
		t.Fatalf("want 2 no., 2 labels and 2 pages counted, got %d, %d, %d", r.Numbers, r.Labels, r.Pages)
	}
}

func TestTheSectionHeadIsNotReadAsOneOfItsOwnNumbers(t *testing.T) {
	// "## 1. TERMS AND RELATIONS" is the § and "### 1. SIGNS AND ASSEMBLIES" is
	// no. 1 of it. Both are a 1 with a title after it and only one of them is a
	// no., which is the whole difficulty of reading a transcription's heads.
	r := Audit(target(), whole(), section())
	for _, f := range r.Findings {
		if f.Rule == "numbering" {
			t.Fatalf("the § head was read as a no.: %s", f.Text)
		}
	}
}

func TestAMissingNumberIsHard(t *testing.T) {
	body := strings.Replace(whole(), "## 2. CRITERIA OF SUBSTITUTION", "## 2. THE WRONG NUMBER IS NOT THE POINT", 1)
	body = strings.Replace(body, "## 2. THE WRONG NUMBER IS NOT THE POINT", "", 1)
	r := Audit(target(), body, section())
	if !hasHard(r, "numbering") {
		t.Fatalf("want the missing no. 2 reported hard, got %v", r.Findings)
	}
}

func TestNumbersOutOfOrderAreHard(t *testing.T) {
	body := "### 2. CRITERIA OF SUBSTITUTION\n\n" + prose(24) + "\n\nC1. Let $A$ be a relation.\n\n" +
		"### 1. SIGNS AND ASSEMBLIES\n\n" + prose(22) + "\n\nCF1. If $A$ is a relation.\n\n" + prose(23)
	r := Audit(target(), body, section())
	if !hasHard(r, "numbering") {
		t.Fatalf("want the order reported hard, got %v", r.Findings)
	}
}

func TestATitleInAnotherCaseIsNotAFailure(t *testing.T) {
	body := strings.Replace(whole(), "### 1. SIGNS AND ASSEMBLIES", "### 1. Signs and Assemblies", 1)
	r := Audit(target(), body, section())
	if !r.OK() {
		t.Fatalf("want a pass, got %v", r.Findings)
	}
	for _, f := range r.Findings {
		if f.Rule == "numbering" {
			t.Fatalf("small caps against upper and lower case is not a difference in the book: %s", f.Text)
		}
	}
}

func TestAMissingLabelIsHard(t *testing.T) {
	body := strings.Replace(whole(), "CF1. If $A$ is a relation.", "", 1)
	r := Audit(target(), body, section())
	if !hasHard(r, "labels") {
		t.Fatalf("want CF1 reported hard, got %v", r.Findings)
	}
}

func TestALabelLiftedIntoAHeadingIsStillTheLabel(t *testing.T) {
	// How a transcription writes it: the label as a heading of its own and the
	// statement under it, rather than run into the head of the paragraph.
	body := strings.Replace(whole(), "CF1. If $A$ is a relation.", "### CF1\n\nIf $A$ is a relation.", 1)
	r := Audit(target(), body, section())
	if !r.OK() {
		t.Fatalf("want a pass, got %v", r.Findings)
	}
}

func TestANamedCriterionIsTheSameLabel(t *testing.T) {
	// The pages carry "C1 (*Syllogism*)." and the import carries "C1.".
	if got := labels("C1 (*Syllogism*). Let $A$ be a relation.\n"); len(got) != 1 || got[0] != "C1" {
		t.Fatalf("want [C1], got %v", got)
	}
}

func TestAStatementIsOneLabelWhateverCaseItIsSetIn(t *testing.T) {
	got := labels("PROPOSITION 4. *Let*\n\nProposition 4. Let\n\n#### Proposition 4\n")
	for _, l := range got {
		if l != "Proposition 4" {
			t.Fatalf("want every one read as Proposition 4, got %v", got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("want all three read, got %v", got)
	}
}

func TestAPageMissingFromTheImportIsHard(t *testing.T) {
	body := strings.Replace(whole(), prose(23), "", 1)
	body = strings.Replace(body, prose(24), "", 1)
	r := Audit(target(), body, section())
	if !hasHard(r, "pages") {
		t.Fatalf("want the dropped page reported hard, got %v", r.Findings)
	}
}

func TestMathematicsIsNotWhatAPageIsFoundBy(t *testing.T) {
	// The same words, every formula written the other way. Two readings of one
	// page argue about the markup and agree about the prose, and the audit has
	// to be on the side of the prose or it fails every import there is.
	p := section()
	body := strings.NewReplacer(`$A$`, `\(A\)`, `\mathscr{T}`, `\mathcal{T}`).Replace(whole())
	r := Audit(target(), body, p)
	if !r.OK() {
		t.Fatalf("want a pass, got %v", r.Findings)
	}
}

func TestAPageWithNoProseIsSaidRatherThanScored(t *testing.T) {
	p := section()
	p.Pages = append(p.Pages, PrintedPage{PDFPage: 24, Text: "$$\n\\tau_x(A)\n$$\n"})
	r := Audit(target(), whole(), p)
	if !r.OK() {
		t.Fatalf("a page of nothing but a display is not a missing page: %v", r.Findings)
	}
	if !has(r, "pages", "no prose") {
		t.Fatalf("want the page said out loud, got %v", r.Findings)
	}
}

func hasHard(r *Result, rule string) bool {
	for _, f := range r.Findings {
		if f.Hard && f.Rule == rule {
			return true
		}
	}
	return false
}

func has(r *Result, rule, text string) bool {
	for _, f := range r.Findings {
		if f.Rule == rule && strings.Contains(f.Text, text) {
			return true
		}
	}
	return false
}
