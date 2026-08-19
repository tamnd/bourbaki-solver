package share

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func yes() *bool { b := true; return &b }
func no() *bool  { b := false; return &b }

func passing() *Result { return &Result{Numbers: 4, Labels: 13, Pages: 9} }

func failing() *Result {
	return &Result{Numbers: 4, Labels: 13, Pages: 9,
		Findings: []Finding{{Rule: "no.", Hard: true, Text: "no. 3 is not in the import"}}}
}

func reviewed() *Review {
	return &Review{Import: "sets", Chapter: 1, Section: 1, By: "a reader", On: "2026-08-20",
		Edition: "1968 Hermann", Promote: yes(), Findings: "nothing"}
}

func sec11() Target { return Target{Book: "sets", Chapter: 1, Section: 1} }

func TestPromoteWantsAllFourThings(t *testing.T) {
	c := Candidate{Target: sec11(), Audit: passing(), Review: reviewed(),
		ContentPath: "content/en/ens/I/01_s1_terms_and_relations.md"}
	d := Decide("sets", c)
	if !d.Promote {
		t.Fatalf("a § that passes the audit, has no occupant and has a reader behind it should promote, got %s", d)
	}
	if !strings.Contains(d.Why, "a reader") || !strings.Contains(d.Why, "1968 Hermann") {
		t.Errorf("the reason should say who read it and against what, got %q", d.Why)
	}
}

// The introduction has no number and no place in the content layout, and the
// point of the test is that it comes out as a listed refusal rather than as a
// file nothing said anything about.
func TestPromoteRefusesTheIntroduction(t *testing.T) {
	d := Decide("sets", Candidate{Target: Target{Book: "sets", Intro: true}})
	if d.Promote || d.Refusal != RefuseIntro {
		t.Fatalf("want the introduction refused as intro, got %s", d)
	}
}

func TestPromoteRefusesAFailingAudit(t *testing.T) {
	c := Candidate{Target: sec11(), Audit: failing(), Review: reviewed()}
	d := Decide("sets", c)
	if d.Promote || d.Refusal != RefuseAudit {
		t.Fatalf("want a failing audit refused, got %s", d)
	}
	if !strings.Contains(d.Why, "no. 3") {
		t.Errorf("the reason should name what is missing, got %q", d.Why)
	}
}

// A reader's word is what the promotion rests on, so a section nobody has read
// does not move however clean the machine checks are.
func TestPromoteRefusesWhatNobodyHasRead(t *testing.T) {
	d := Decide("sets", Candidate{Target: sec11(), Audit: passing()})
	if d.Promote || d.Refusal != RefuseUnreviewed {
		t.Fatalf("want an unreviewed § refused, got %s", d)
	}
}

func TestPromoteRefusesWhatTheReaderRejected(t *testing.T) {
	v := reviewed()
	v.Promote, v.Findings = no(), "the criteria in no. 2 are transcribed out of order"
	d := Decide("sets", Candidate{Target: sec11(), Audit: passing(), Review: v})
	if d.Promote || d.Refusal != RefuseRejected {
		t.Fatalf("want a rejected § refused, got %s", d)
	}
	if !strings.Contains(d.Why, "out of order") {
		t.Errorf("the reason should carry what the reader found, got %q", d.Why)
	}
}

// This is the case that refuses everything in the corpus today, so it is the
// one worth being sure of: a reading made from the pages outranks an import
// even when the import is clean and somebody has signed off on it.
func TestPromoteWillNotWriteOverAReadingFromThePages(t *testing.T) {
	c := Candidate{Target: sec11(), Audit: passing(), Review: reviewed(),
		ContentPath: "content/en/ens/I/01_s1_terms_and_relations.md",
		Occupant:    &Occupant{Extraction: "ocr", PDFPages: "0022-0030"}}
	d := Decide("sets", c)
	if d.Promote || d.Refusal != RefuseOccupied {
		t.Fatalf("want a page-derived occupant to refuse the promotion, got %s", d)
	}
	if !strings.Contains(d.Why, "0022-0030") {
		t.Errorf("the reason should say what is there, got %q", d.Why)
	}
	// And the same guard has to hold for a native reading, not only an OCR one.
	c.Occupant = &Occupant{Extraction: "native", PDFPages: "0022-0030"}
	if d := Decide("sets", c); d.Promote {
		t.Errorf("a native reading should refuse the promotion too, got %s", d)
	}
}

// An earlier promotion is not a reading from the pages, so replacing it loses
// nothing that was not already an import.
func TestPromoteMayReplaceAnEarlierPromotion(t *testing.T) {
	c := Candidate{Target: sec11(), Audit: passing(), Review: reviewed(),
		Occupant: &Occupant{Extraction: "share"}}
	if d := Decide("sets", c); !d.Promote {
		t.Fatalf("want an earlier promotion replaceable, got %s", d)
	}
}

// The review is of a text and not of a filename, so editing the import after
// it was read has to break the sign-off rather than travel with it.
func TestPromoteNoticesTheImportChangedSinceItWasRead(t *testing.T) {
	v := reviewed()
	v.Body = "aaaaaaaaaaaaaaaa"
	c := Candidate{Target: sec11(), Audit: passing(), Review: v, SHA256: "bbbbbbbbbbbbbbbb"}
	d := Decide("sets", c)
	if d.Promote || d.Refusal != RefuseStale {
		t.Fatalf("want an edited import refused as stale, got %s", d)
	}
}

func TestPromoteAllowsAReviewWithNoDigestAndSaysSo(t *testing.T) {
	c := Candidate{Target: sec11(), Audit: passing(), Review: reviewed(), SHA256: "bbbbbbbbbbbbbbbb"}
	d := Decide("sets", c)
	if !d.Promote {
		t.Fatalf("a review with no digest is still a person's word, got %s", d)
	}
	if !strings.Contains(d.Why, "would not be noticed") {
		t.Errorf("the reason should say the digest is missing, got %q", d.Why)
	}
}

// Every import file has to come out either promoted or listed, because a file
// in neither list is how an import gets cited as the book.
func TestPromoteAccountsForEveryFile(t *testing.T) {
	rep := Report{Decisions: []Decision{
		Decide("sets", Candidate{Target: Target{Book: "sets", Intro: true}}),
		Decide("sets", Candidate{Target: sec11(), Audit: passing()}),
		Decide("sets", Candidate{Target: sec11(), Audit: passing(), Review: reviewed()}),
	}}
	if got := rep.Promoted(); got != 1 {
		t.Errorf("want 1 promoted, got %d", got)
	}
	reasons := strings.Join(rep.Reasons(), ", ")
	if reasons != "1 intro, 1 unreviewed" {
		t.Errorf("want the refusals counted and in a stable order, got %q", reasons)
	}
	for _, d := range rep.Decisions {
		if !d.Promote && d.Refusal == "" {
			t.Errorf("%s is neither promoted nor refused for a reason", d.Target)
		}
	}
}

func writeReviews(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ReviewsPath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// A checkout where nobody has reviewed anything is the ordinary state of the
// project, not an error, and the answer to every promotion there is unreviewed.
func TestLoadReviewsOfACheckoutWithNoManifest(t *testing.T) {
	r, err := LoadReviews(t.TempDir())
	if err != nil {
		t.Fatalf("a missing review manifest should not be an error: %v", err)
	}
	if r.Find("sets", 1, 1) != nil {
		t.Error("an empty manifest should find nothing")
	}
}

func TestLoadReviewsRefusesAnIncompleteRecord(t *testing.T) {
	for name, body := range map[string]string{
		"nobody read it": "reviews:\n  - import: sets\n    chapter: 1\n    section: 1\n    promote: true\n    findings: nothing\n",
		"no verdict":     "reviews:\n  - import: sets\n    chapter: 1\n    section: 1\n    reviewed_by: a reader\n    findings: nothing\n",
		"no findings":    "reviews:\n  - import: sets\n    chapter: 1\n    section: 1\n    reviewed_by: a reader\n    promote: true\n",
		"no section":     "reviews:\n  - import: sets\n    reviewed_by: a reader\n    promote: true\n    findings: nothing\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadReviews(writeReviews(t, body)); err == nil {
				t.Fatalf("want %s refused", name)
			}
		})
	}
}

func TestLoadReviewsFindsTheRecordForOneSection(t *testing.T) {
	root := writeReviews(t, "reviews:\n"+
		"  - import: sets\n    chapter: 1\n    section: 1\n    reviewed_by: a reader\n    promote: true\n    findings: nothing\n"+
		"  - import: sets\n    chapter: 1\n    section: 2\n    reviewed_by: another reader\n    promote: false\n    findings: no. 2 is missing a criterion\n")
	r, err := LoadReviews(root)
	if err != nil {
		t.Fatal(err)
	}
	if v := r.Find("sets", 1, 2); v == nil || v.By != "another reader" || *v.Promote {
		t.Fatalf("want the second record found and read, got %+v", v)
	}
	if r.Find("sets", 1, 3) != nil {
		t.Error("a section with no record should find nothing")
	}
}
