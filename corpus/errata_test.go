package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeErrata(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ErrataPath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// A corpus with no errata in it is the ordinary case and has no file.
func TestNoErrataFileIsNoErrata(t *testing.T) {
	m, err := LoadErrata(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 0 || len(m.Lookup("en")) != 0 {
		t.Errorf("an absent file gave %d entries", len(m.Entries))
	}
}

// The English of 1974 and the French of 1981 are different books, and an error
// in one is not an error in the other. Exercise 3 a) of VIII § 1 is the case:
// the English says finite where the French says infinite, and it is the English
// that is wrong.
func TestAnErratumBelongsToOnePrinting(t *testing.T) {
	root := writeErrata(t, `errata:
    - label: alg-viii-s1-ex-3
      lang: en
      errata:
        - says: K is finite-dimensional over $K'$
          read: K is infinite-dimensional over $K'$
          why: the French of 1981 reads "de dimension infinie sur $K'$"
    - label: alg-viii-s2-ex-7
      lang: fr
      errata:
        - says: un
          read: deux
          why: because
`)
	m, err := LoadErrata(root)
	if err != nil {
		t.Fatal(err)
	}
	en := m.Lookup("en")
	if len(en) != 1 || len(en["alg-viii-s1-ex-3"]) != 1 {
		t.Fatalf("the English lookup has %d labels in it", len(en))
	}
	if got := en["alg-viii-s1-ex-3"][0].Read; !strings.Contains(got, "infinite") {
		t.Errorf("read is %q", got)
	}
	if _, ok := m.Lookup("fr")["alg-viii-s1-ex-3"]; ok {
		t.Error("the English erratum was handed out as a French one")
	}
	if len(m.Lookup("vi")) != 0 {
		t.Error("a printing with no errata got some")
	}
}

// Everything here is written by a person to be applied by a build, and the
// failure mode is that it is written and never applied. So the shapes that
// cannot apply are refused where they are read.
func TestAnErratumThatWouldGoUnreadIsRefused(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"no label", `errata:
    - lang: en
      errata:
        - {says: a, read: b, why: c}
`, "no label"},
		{"no lang", `errata:
    - label: alg-viii-s1-ex-3
      errata:
        - {says: a, read: b, why: c}
`, "no lang"},
		{"nothing to correct", `errata:
    - label: alg-viii-s1-ex-3
      lang: en
      errata: []
`, "lists no errata"},
		{"no reason", `errata:
    - label: alg-viii-s1-ex-3
      lang: en
      errata:
        - {says: a, read: b}
`, "no reason"},
		{"nothing to read instead", `errata:
    - label: alg-viii-s1-ex-3
      lang: en
      errata:
        - {says: a, why: c}
`, "what to read instead"},
		{"entered twice", `errata:
    - label: alg-viii-s1-ex-3
      lang: en
      errata:
        - {says: a, read: b, why: c}
    - label: alg-viii-s1-ex-3
      lang: en
      errata:
        - {says: d, read: e, why: f}
`, "twice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadErrata(writeErrata(t, c.body))
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the error does not say %q: %v", c.want, err)
			}
		})
	}
}

// Two people adding an entry on the same day should not conflict over where in
// the file it went.
func TestTheManifestIsWrittenSorted(t *testing.T) {
	m := &ErrataManifest{Entries: []LabelErrata{
		{Label: "alg-viii-s9-ex-1", Lang: "en", Errata: []Erratum{{Says: "a", Read: "b", Why: "c"}}},
		{Label: "alg-viii-s1-ex-3", Lang: "fr", Errata: []Erratum{{Says: "a", Read: "b", Why: "c"}}},
		{Label: "alg-viii-s1-ex-3", Lang: "en", Errata: []Erratum{{Says: "a", Read: "b", Why: "c"}}},
	}}
	b, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	if !strings.HasPrefix(out, "# Where a printing is wrong.") {
		t.Errorf("the file does not say who writes it:\n%s", out)
	}
	first, second, third := strings.Index(out, "lang: en"), strings.Index(out, "lang: fr"), strings.Index(out, "s9")
	if !(first < second && second < third) {
		t.Errorf("out of order at %d, %d, %d:\n%s", first, second, third, out)
	}
	if m.Entries[0].Label != "alg-viii-s9-ex-1" {
		t.Error("Bytes sorted the manifest it was called on")
	}
}

// A volume's table of contents is a claim the volume makes about itself, and a
// volume can be wrong about itself. Theory of Sets lists no. 5 of § 7 of
// chapter III on page 201 and prints its heading on 202. That correction is not
// against a labelled piece of anything, so it is kept apart from the rest.
func TestTheContentsOfAVolumeHasItsOwnErrata(t *testing.T) {
	root := writeErrata(t, `errata:
    - label: alg-viii-s1-ex-3
      lang: en
      errata:
        - {says: a, read: b, why: c}
contents:
    - book: ens-i-iv
      errata:
        - says: '5. Direct limits 201'
          read: '5. Direct limits 202'
          why: the volume prints the heading on 202
`)
	m, err := LoadErrata(root)
	if err != nil {
		t.Fatal(err)
	}
	got := m.ContentsErrata("ens-i-iv")
	if len(got) != 1 || got[0].Read != "5. Direct limits 202" {
		t.Fatalf("the contents errata of ens-i-iv are %+v", got)
	}
	if len(m.ContentsErrata("alg-viii")) != 0 {
		t.Error("a volume with no contents errata got some")
	}
	if len(m.Lookup("en")) != 1 {
		t.Error("the contents errata disturbed the labelled ones")
	}
}

// The same failure mode as above, in the same place: written down, never
// applied.
func TestAContentsErratumThatWouldGoUnreadIsRefused(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"no book", `contents:
    - errata:
        - {says: a, read: b, why: c}
`, "no book"},
		{"nothing to correct", `contents:
    - book: ens-i-iv
      errata: []
`, "lists no errata"},
		{"no reason", `contents:
    - book: ens-i-iv
      errata:
        - {says: a, read: b}
`, "no reason"},
		{"entered twice", `contents:
    - book: ens-i-iv
      errata:
        - {says: a, read: b, why: c}
    - book: ens-i-iv
      errata:
        - {says: d, read: e, why: f}
`, "twice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadErrata(writeErrata(t, c.body))
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the error does not say %q: %v", c.want, err)
			}
		})
	}
}

// A running head is corrected against the PDF page it is printed on, because a
// mangled head is not unique in a volume the way a contents line is.
func TestTheRunningHeadsOfAVolumeAreCorrectedByPage(t *testing.T) {
	root := writeErrata(t, `heads:
    - book: alg-iv-vii-fr
      errata:
        - pdf_page: 370
          says: A V I . 37
          read: A VII.37
          why: the scan lost a stroke off the numeral
`)
	m, err := LoadErrata(root)
	if err != nil {
		t.Fatal(err)
	}
	got := m.HeadErrata("alg-iv-vii-fr")
	if len(got) != 1 || got[0].PDFPage != 370 || got[0].Read != "A VII.37" {
		t.Fatalf("the head errata of alg-iv-vii-fr are %+v", got)
	}
	if len(m.HeadErrata("alg-i-iii-fr")) != 0 {
		t.Error("a volume with no head errata got some")
	}
}

// The same failure mode again: written down, never applied.
func TestAHeadErratumThatWouldGoUnreadIsRefused(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"no book", `heads:
    - errata:
        - {pdf_page: 1, says: a, read: b, why: c}
`, "no book"},
		{"nothing to correct", `heads:
    - book: alg-x-fr
      errata: []
`, "list no errata"},
		{"no page", `heads:
    - book: alg-x-fr
      errata:
        - {says: a, read: b, why: c}
`, "which pdf page"},
		{"page twice", `heads:
    - book: alg-x-fr
      errata:
        - {pdf_page: 7, says: a, read: b, why: c}
        - {pdf_page: 7, says: d, read: e, why: f}
`, "entered twice"},
		{"nothing said and nothing to read", `heads:
    - book: alg-x-fr
      errata:
        - {pdf_page: 7, says: '', read: '  ', why: c}
`, "what the head says"},
		{"reads the same", `heads:
    - book: alg-x-fr
      errata:
        - {pdf_page: 7, says: a, read: a, why: c}
`, "what the head says"},
		{"no reason", `heads:
    - book: alg-x-fr
      errata:
        - {pdf_page: 7, says: a, read: b}
`, "no reason"},
		{"book twice", `heads:
    - book: alg-x-fr
      errata:
        - {pdf_page: 7, says: a, read: b, why: c}
    - book: alg-x-fr
      errata:
        - {pdf_page: 8, says: d, read: e, why: f}
`, "entered twice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadErrata(writeErrata(t, c.body))
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the error does not say %q: %v", c.want, err)
			}
		})
	}
}

// An erratum with no says supplies a head rather than correcting one, which is
// how the page a chapter opens on says what it is printed as. Every house in
// the series suppresses the running head there and numbers the page all the
// same, so there is nothing on the page to quote.
func TestAHeadErratumMaySupplyAHeadThePrintingSuppresses(t *testing.T) {
	root := writeErrata(t, `heads:
    - book: ac-x-fr
      errata:
        - pdf_page: 1
          says: ''
          read: AC X.1
          why: the opener of the only chapter, printed AC X.1 and carrying no head
`)
	m, err := LoadErrata(root)
	if err != nil {
		t.Fatal(err)
	}
	got := m.HeadErrata("ac-x-fr")
	if len(got) != 1 || got[0].Says != "" || got[0].Read != "AC X.1" {
		t.Fatalf("the manifest gave %v, want one erratum supplying AC X.1", got)
	}
}
