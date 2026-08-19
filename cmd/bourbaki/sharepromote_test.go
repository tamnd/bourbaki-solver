package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/share"
)

func promoted(t *testing.T) (string, corpus.SectionFrontMatter) {
	t.Helper()
	root := t.TempDir()
	m := corpus.SectionFrontMatter{
		Book: "ens", BookTitle: "Theory of Sets",
		Chapter: "I", ChapterTitle: "DESCRIPTION OF FORMAL MATHEMATICS",
		Section: 1, SectionTitle: "Terms and relations",
		Lang: "en", Source: "ens-i-iv", Extraction: "share", Statements: 13,
	}
	if _, err := writePromoted(root, m, "# 1. SIGNS AND ASSEMBLIES\n\nbody\n"); err != nil {
		t.Fatal(err)
	}
	return root, m
}

// A promoted file has to be readable as an ordinary section, because the whole
// point of moving it is that the audit rules and everything else that walks
// content/ can now see it.
func TestPromotedFileIsAnOrdinarySection(t *testing.T) {
	root, m := promoted(t)
	path := corpus.SectionPath(root, "en", m)
	if want := filepath.Join(root, "content", "en", "ens", "I", "01_s1_terms_and_relations.md"); path != want {
		t.Fatalf("want %s, got %s", want, path)
	}
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatalf("a promoted file should read back as a section: %v", err)
	}
	if f.Meta.Extraction != "share" {
		t.Errorf("want extraction share so the reading can be told from a page one, got %q", f.Meta.Extraction)
	}
	if f.Meta.ContentSHA256 != corpus.ContentSHA256(f.Body) {
		t.Errorf("a promoted file should carry the digest of its own body")
	}
	if !strings.Contains(f.Body, "SIGNS AND ASSEMBLIES") {
		t.Errorf("the body should be the import's, got %q", f.Body)
	}
}

// occupantOf is what stands between an import and a reading made from the
// pages, so it has to report the extraction it finds and it has to say nothing
// is there when nothing is.
func TestOccupantOfReadsWhatIsAlreadyThere(t *testing.T) {
	root, m := promoted(t)
	o := occupantOf(root, "en", m)
	if o == nil {
		t.Fatal("want the file just written found as an occupant")
	}
	if o.Extraction != "share" || o.FromPages() {
		t.Errorf("an earlier promotion is not a reading from the pages, got %+v", o)
	}

	m.Section, m.SectionTitle = 2, "Theorems"
	if o := occupantOf(root, "en", m); o != nil {
		t.Errorf("want nothing at an empty path, got %+v", o)
	}
}

// A head this cannot parse is a reason to stop and look, not a reason to write
// over the file, so an unreadable occupant counts as coming from the pages.
func TestOccupantOfTreatsAnUnreadableFileAsOccupied(t *testing.T) {
	root, m := promoted(t)
	if err := os.WriteFile(corpus.SectionPath(root, "en", m), []byte("---\nbook: [\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := occupantOf(root, "en", m)
	if o == nil || !o.FromPages() {
		t.Fatalf("want an unreadable occupant to block the promotion, got %+v", o)
	}
	if d := share.Decide("sets", share.Candidate{
		Target: share.Target{Book: "sets", Chapter: 1, Section: 1},
		Audit:  &share.Result{}, Occupant: o,
	}); d.Promote {
		t.Errorf("want the promotion refused over an unreadable file, got %s", d)
	}
}
