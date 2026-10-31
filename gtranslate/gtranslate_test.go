package gtranslate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
)

func TestMaskKeepsMathMarkupAndCitations(t *testing.T) {
	body := "#### Proposition 3 {#alg-ii-s1-prop-3 .statement tag=05JO}\n\n" +
		"Let $A$ be a ring and $$x^2 = 1$$ (I, § 8, no. 3). See [Prop. 2](#alg-x), `code` and [^1].\n"
	m := Mask(body, []Pin{{From: "ring", To: "vành"}, {From: "proposition", To: "mệnh đề"}}, true)
	for _, gone := range []string{"$A$", "$$x^2 = 1$$", "{#alg", "](#alg-x)", "`code`", "[^1]", "§ 8, no. 3", "ring", "Proposition"} {
		if strings.Contains(m.Text, gone) {
			t.Errorf("%q went to Google: %s", gone, m.Text)
		}
	}
	if strings.Contains(m.Text, " a ⟦") {
		t.Errorf("the article in front of a pinned noun was sent: %s", m.Text)
	}
	back, err := m.Unmask(m.Text)
	if err != nil {
		t.Fatal(err)
	}
	want := "#### Mệnh đề 3 {#alg-ii-s1-prop-3 .statement tag=05JO}\n\n" +
		"Let $A$ be vành and $$x^2 = 1$$ (I, § 8, no. 3). See [Prop. 2](#alg-x), `code` and [^1].\n"
	if back != want {
		t.Errorf("unmask gave\n%s\nwant\n%s", back, want)
	}
}

func TestUnmaskRefusesALostOrDoubledToken(t *testing.T) {
	m := Mask("Let $A$ and $B$ be sets.", nil, false)
	if _, err := m.Unmask("Cho ⟦0⟧ là tập hợp."); err == nil {
		t.Error("a dropped formula was accepted")
	}
	if _, err := m.Unmask("Cho ⟦0⟧ và ⟦1⟧ và ⟦1⟧."); err == nil {
		t.Error("a doubled formula was accepted")
	}
	if got, err := m.Unmask("Cho ⟦ 0 ⟧ và ⟦1 ⟧ là tập hợp."); err != nil || got != "Cho $A$ và $B$ là tập hợp." {
		t.Errorf("a token Google spaced out was not read back: %q %v", got, err)
	}
}

func TestUnmaskSpacesAGluedPin(t *testing.T) {
	m := Mask("every ring is", []Pin{{From: "ring", To: "vành"}}, false)
	got, err := m.Unmask("mọi⟦0⟧đều")
	if err != nil || got != "mọi vành đều" {
		t.Errorf("got %q %v", got, err)
	}
}

func TestPinsPluralsAndKeepsWholeWords(t *testing.T) {
	m := Mask("The rings and modules; a submodule.", []Pin{{"ring", "vành"}, {"module", "môđun"}}, true)
	if strings.Contains(m.Text, "rings") || strings.Contains(m.Text, " modules") {
		t.Errorf("a plural was not pinned: %s", m.Text)
	}
	if !strings.Contains(m.Text, "submodule") {
		t.Errorf("a word inside a longer word was pinned: %s", m.Text)
	}
	back, _ := m.Unmask(m.Text)
	if !strings.HasPrefix(back, "Vành") {
		t.Errorf("a pinned noun that opened the sentence lost its capital: %s", back)
	}
}

func TestNounPhrase(t *testing.T) {
	for en, want := range map[string]bool{
		"ring": true, "free module": true, "proposition": true, "homothety": true,
		"commutative": false, "free": false, "isomorphic to": false, "algebraic over": false,
		"totally ordered": false, "left exact": false, "group structure": true, "now prove": false,
	} {
		if got := nounPhrase(en); got != want {
			t.Errorf("nounPhrase(%q) = %v, want %v", en, got, want)
		}
	}
}

func TestFixPutsTheGlossaryOverGoogle(t *testing.T) {
	loose := []Pin{{"free", "tự do"}, {"commutative", "giao hoán"}}
	learned := Learned{"free": "miễn phí", "commutative": "giao hoán"}
	got := Fix("Mô đun này là miễn phí. Miễn phí!", loose, learned)
	if got != "Mô đun này là tự do. Tự do!" {
		t.Errorf("got %q", got)
	}
	if got := Fix("đã tự do và miễn phí", loose, learned); got != "đã tự do và miễn phí" {
		t.Errorf("an answer that already has the glossary's word was changed: %q", got)
	}
}

func TestSplitCutsAtLines(t *testing.T) {
	text := strings.Repeat("a line of text\n", 50)
	parts := Split(strings.TrimSuffix(text, "\n"), 100)
	if strings.Join(parts, "\n") != strings.TrimSuffix(text, "\n") {
		t.Error("the pieces do not join back")
	}
	for _, p := range parts {
		if len(p) > 100 {
			t.Errorf("a piece of %d bytes", len(p))
		}
	}
}

// A fake Google that upper-cases whatever it is sent, the way gtx answers.
func fake(t *testing.T) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.PostForm.Get("q")
		var sents []map[string]string
		for _, line := range strings.SplitAfter(q, "\n") {
			sents = append(sents, map[string]string{"trans": strings.ToUpper(line)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"sentences": sents})
	}))
	t.Cleanup(srv.Close)
	c := New(t.TempDir())
	c.Gap = 0
	gtxURL := srv.URL
	c.endpoints = []endpoint{{
		name: "fake",
		build: func(ctx context.Context, sl, tl, text string) (*http.Request, error) {
			return gtx.build(ctx, sl, tl, text)
		},
		parse: gtx.parse,
	}}
	c.HTTP = &http.Client{Transport: rewrite{gtxURL}}
	return c
}

type rewrite struct{ to string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	target, _ := http.NewRequest(req.Method, r.to+u.Path+"?"+u.RawQuery, req.Body)
	target.Header = req.Header
	return http.DefaultTransport.RoundTrip(target)
}

func TestDraftEndToEnd(t *testing.T) {
	g := &glossary.Glossary{Terms: []glossary.Term{
		{EN: "ring", VI: "vành"}, {EN: "commutative", VI: "giao hoán"},
	}}
	d := Drafter{Client: fake(t), Glossary: g, Source: "en", Target: "vi"}
	got, _, err := d.Draft(context.Background(), "Let $a$ be a commutative ring.\n")
	if err != nil {
		t.Fatal(err)
	}
	// The fake upper-cases prose, so the math and the pinned noun are what
	// come back in lower case.
	if !strings.Contains(got, "$a$") || !strings.Contains(got, "vành") {
		t.Errorf("got %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("the trailing newline was lost")
	}
	if d.Client.Requests == 0 {
		t.Error("nothing was asked")
	}
	// The second draft of the same text is read from the cache.
	before := d.Client.Requests
	if _, _, err := d.Draft(context.Background(), "Let $a$ be a commutative ring.\n"); err != nil {
		t.Fatal(err)
	}
	if d.Client.Requests != before {
		t.Error("the same passage was asked twice")
	}
}

func TestNothingIsPinnedIntoEnglish(t *testing.T) {
	g := &glossary.Glossary{Terms: []glossary.Term{
		{EN: "algebra", FR: "algèbre"}, {EN: "now prove", FR: "montrer"}, {EN: "smooth", FR: "lisse"},
	}}
	pins, loose := Terms(g, "fr", "en", "Montrer que deux algèbres lisses sont isomorphes.")
	if len(pins) != 0 {
		t.Errorf("pinned into English: %v", pins)
	}
	if len(loose) != 1 || loose[0].To != "algebra" {
		t.Errorf("corrected afterwards: %v, want only the noun", loose)
	}
}

// A block whose formula Google doubles is asked again a sentence at a time.
func TestADoubledFormulaIsAskedSentenceBySentence(t *testing.T) {
	c := fake(t)
	c.HTTP = &http.Client{Transport: doubler{c.HTTP.Transport, ""}}
	d := Drafter{Client: c, Glossary: &glossary.Glossary{}, Source: "en", Target: "zh"}
	got, left, err := d.Draft(context.Background(), "Let $f$ be a map with $\\text{resp. B}$. Then $g$ is one.")
	if err != nil || left != 0 {
		t.Fatal(left, err)
	}
	if strings.Count(got, "$f$") != 1 || strings.Count(got, "$g$") != 1 || !strings.Contains(got, "$\\text{resp. B}$") {
		t.Errorf("got %q", got)
	}
}

// doubler repeats the first token of any request holding two sentences, the
// way Google does when it splits one.
type doubler struct {
	next http.RoundTripper
	when string
}

func (d doubler) RoundTrip(req *http.Request) (*http.Response, error) {
	_ = req.ParseForm()
	q := req.PostForm.Get("q")
	when := d.when
	if when == "" {
		when = ". "
	}
	if strings.Contains(q, when) {
		q += " ⟦0⟧"
	}
	form := req.PostForm
	form.Set("q", q)
	r, _ := http.NewRequest(req.Method, req.URL.String(), strings.NewReader(form.Encode()))
	r.Header = req.Header
	return d.next.RoundTrip(r)
}

func TestUnmaskClosesTheSpacesGoogleLeavesInChinese(t *testing.T) {
	m := Mask("By groups with operators, a ring.", []Pin{{"group", "群"}, {"ring", "环"}}, true)
	got, err := m.Unmask("根据 ⟦0⟧ 与运算符，一个⟦1⟧。")
	if err != nil || got != "根据群与运算符，一个环。" {
		t.Errorf("got %q %v", got, err)
	}
}

// A sentence Google doubles a formula in however it is asked stays as the
// source wrote it, and is counted.
func TestASentenceGoogleCannotKeepWholeIsLeftInTheSource(t *testing.T) {
	c := fake(t)
	c.HTTP = &http.Client{Transport: doubler{c.HTTP.Transport, "⟦"}}
	d := Drafter{Client: c, Glossary: &glossary.Glossary{}, Source: "en", Target: "zh"}
	got, left, err := d.Draft(context.Background(), "Let $f$ be a map. Then the end.")
	if err != nil {
		t.Fatal(err)
	}
	if left != 1 || !strings.HasPrefix(got, "Let $f$ be a map. THEN THE END.") {
		t.Errorf("got %q, %d left", got, left)
	}
}

func TestAnArticleOpeningAnEmphasisIsDropped(t *testing.T) {
	m := Mask("(2) *a law of action* and *The* end, *an x*.", nil, true)
	if m.Text != "(2) *law of action* and *The* end, *x*." {
		t.Errorf("got %q", m.Text)
	}
	if got := Mask("*a law*", nil, false).Text; got != "*a law*" {
		t.Errorf("an article dropped going into a language that has them: %q", got)
	}
}

func TestAGluedCitationIsSpacedApart(t *testing.T) {
	m := Mask("Corollary 2 of § 1, no. 4 and Chapter II, § 2 no. 5.", nil, true)
	got, err := m.Unmask("推论2⟦0⟧和章⟦1⟧。")
	if err != nil || got != "推论2 § 1, no. 4和章II, § 2 no. 5。" {
		t.Errorf("got %q %v", got, err)
	}
}

func TestAWideHeadingAndARomanLabelComeBack(t *testing.T) {
	if got := tidy("## Introduction\n", "＃＃ 介绍\n"); got != "## 介绍\n" {
		t.Errorf("got %q", got)
	}
	for _, src := range []string{"**I. Localization.** Let", "Chapter I (*)", "Let I be a set.", "(Vol. II, p. 640)"} {
		if m := Mask(src, nil, true); regexp.MustCompile(`\b[IV]+\b`).MatchString(m.Text) {
			t.Errorf("a roman numeral went to Google: %s", m.Text)
		}
	}
	if m := Mask("It is Clear. Vanishing.", nil, true); m.Text != "It is Clear. Vanishing." {
		t.Errorf("a word was masked: %s", m.Text)
	}
}
