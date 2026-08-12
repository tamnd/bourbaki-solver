package publish

import (
	"fmt"
	"strings"
	"testing"
)

// The one thing a Markdown renderer over this corpus has to get right. Two
// formulae in a sentence give a Markdown parser two asterisks to pair up, and
// pairing them cuts an emphasis span through the middle of both. Nothing else
// in the file would look wrong, which is why this is a test and not a comment.
func TestMathIsNotMarkdown(t *testing.T) {
	body := `Let $x^*\otimes y$ be the image of $x^*\in E^*$ under the map.`
	got, err := Renderer{}.HTML(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<em>") || strings.Contains(got, "<strong>") {
		t.Errorf("mathematics was read as emphasis:\n%s", got)
	}
	for _, want := range []string{`<span class="math">$x^*\otimes y$</span>`, `<span class="math">$x^*\in E^*$</span>`} {
		if !strings.Contains(got, want) {
			t.Errorf("want %s in:\n%s", want, got)
		}
	}
}

// A display is a block, and putting it inside a <p> is markup no two browsers
// lay out the same way.
func TestADisplayIsNotInsideAParagraph(t *testing.T) {
	body := "We have\n\n$$\n\\pi (a)\\circ u = u\\circ \\pi (a)\n$$\n\nfor $a\\in A$."
	got, err := Renderer{}.HTML(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<div class="math display">`) {
		t.Fatalf("no display block:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "<div") && strings.Contains(line, "<p>") {
			t.Errorf("a display sits inside a paragraph: %s", line)
		}
	}
}

// Everything is escaped and the three constructs the corpus uses are put back
// afterwards, so a less-than sign in the prose can never open a tag.
func TestProseCannotOpenATag(t *testing.T) {
	got, err := Renderer{}.HTML("If a < b then <script>alert(1)</script> is prose.")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<script>") {
		t.Errorf("markup leaked out of the prose:\n%s", got)
	}
	if !strings.Contains(got, "a &lt; b") {
		t.Errorf("want an escaped less-than in:\n%s", got)
	}
}

// A span that opens and never closes is a broken file, and rendering it would
// swallow the rest of the section into one formula. It has to be an error.
func TestAnUnclosedFormulaIsAnError(t *testing.T) {
	var r Renderer
	if _, err := r.HTML("The map $f: E\\rightarrow F is linear."); err == nil {
		t.Error("an unclosed formula rendered without complaint")
	}
}

func TestHeadingsCarryTheirLabelAndTag(t *testing.T) {
	body := "## § 14. A Title\n\n#### Theorem 1 {#alg-viii-s14-thm-1 .statement tag=00GJ}\n\nProse.\n"
	hs := Headings(body)
	if len(hs) != 2 {
		t.Fatalf("read %d headings, want 2: %+v", len(hs), hs)
	}
	if hs[0].Level != 2 || hs[0].Text != "§ 14. A Title" || hs[0].Statement {
		t.Errorf("the § heading came out as %+v", hs[0])
	}
	h := hs[1]
	if h.Level != 4 || h.Text != "Theorem 1" || h.Label != "alg-viii-s14-thm-1" || h.Tag != "00GJ" || !h.Statement {
		t.Errorf("the statement heading came out as %+v", h)
	}
}

// A unit runs to the next heading of any level and not to the next statement,
// because the prose that opens the following no. is not part of the proof above
// it. Getting this wrong puts someone else's paragraph on a tag page.
func TestAUnitStopsAtTheNextHeading(t *testing.T) {
	body := strings.Join([]string{
		"#### Theorem 1 {#alg-viii-s14-thm-1 .statement tag=00GJ}",
		"",
		"The statement, and its proof.",
		"",
		"### 2. The next no.",
		"",
		"Prose that belongs to nobody.",
		"",
		"#### Lemma 2 {#alg-viii-s14-lem-2 .statement tag=00GO}",
		"",
		"The second one.",
	}, "\n")
	us := Units(body)
	if len(us) != 2 {
		t.Fatalf("cut %d units, want 2", len(us))
	}
	if us[0].Body != "The statement, and its proof." {
		t.Errorf("the first unit is %q", us[0].Body)
	}
	if us[1].Body != "The second one." {
		t.Errorf("the second unit is %q", us[1].Body)
	}
}

// The tag on a statement heading becomes a link to its page, which is what
// makes every statement in the running text reachable by its permanent name.
func TestAStatementHeadingLinksItsTag(t *testing.T) {
	r := Renderer{Link: func(tag string) string { return "/tag/" + tag + "/" }}
	got, err := r.HTML("#### Theorem 1 {#alg-viii-s14-thm-1 .statement tag=00GJ}\n")
	if err != nil {
		t.Fatal(err)
	}
	want := `<h4 id="alg-viii-s14-thm-1" class="statement">Theorem 1 <a class="tag" href="/tag/00GJ/" title="permanent tag">00GJ</a></h4>`
	if strings.TrimSpace(got) != want {
		t.Errorf("got  %s\nwant %s", strings.TrimSpace(got), want)
	}
}

// The corpus writes its links relative to the chapter directory its files sit
// in, and a section page is one level below the chapter on the site. Passed
// through unchanged the link still looks like a link and lands nowhere.
func TestARelativeLinkIsResolvedAgainstTheChapter(t *testing.T) {
	r := Renderer{Dir: "/en/alg/VIII/"}
	got, err := r.HTML("See the [exercises for § 1](exercises/s1/).")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `<a href="/en/alg/VIII/exercises/s1/">exercises for § 1</a>`) {
		t.Errorf("the link was not resolved:\n%s", got)
	}
	for _, url := range []string{"/tag/00GJ/", "https://example.org/x", "#alg-viii-s1-thm-1"} {
		got, err := r.HTML("See [it](" + url + ").")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `href="`+url+`"`) {
			t.Errorf("%s was rewritten:\n%s", url, got)
		}
	}
}

// Masking has to be reversible on the nose. If it is not, the corpus loses a
// character somewhere in the middle of a formula and nothing says so.
func TestMaskingLosesNothing(t *testing.T) {
	body := "Let $a$ and $b$ be in\n\n$$\nE\\otimes F\n$$\n\nand write $c$ for the rest."
	masked, spans, err := mask(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 4 {
		t.Fatalf("took out %d spans, want 4", len(spans))
	}
	back := placeholderRE.ReplaceAllStringFunc(masked, func(m string) string {
		var i int
		if _, err := fmt.Sscanf(m, "\x00m%d\x00", &i); err != nil {
			t.Fatal(err)
		}
		if spans[i].Display {
			return "$$" + spans[i].Text + "$$"
		}
		return "$" + spans[i].Text + "$"
	})
	if back != body {
		t.Errorf("round trip changed the body:\ngot  %q\nwant %q", back, body)
	}
}
