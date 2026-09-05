package book

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The floor is the one thing in this package that refuses. Everything else the
// audit finds is a book with something wrong with it; this is the case where
// there is no book. The last full sweep produced 21 four page PDFs, a cover and
// a title page and a contents of nothing, each passing 18 of 21 checks because
// there was no text in them to be wrong about.

func TestATypesetterThatStoppedIsAFailedCheckAndNotASilence(t *testing.T) {
	// The shape a stopped run used to have: no pages, an error, and an audit
	// that said nothing at all about the typesetter, because the command nilled
	// the build out and this function returns on a nil build. That reads exactly
	// like a volume nobody asked to typeset, which is why a regression presented
	// as nothing happening.
	a := &Audit{}
	a.typeset(&Volume{Meta: corpus.Book{Pages: 144}},
		&Build{PDF: "book.pdf", Failed: "tectonic: exit status 1\nand forty lines of log"},
		DefaultAuditOptions())

	want := map[string]bool{
		"the typesetter ran to the end": false,
		"the typesetter wrote a PDF":    false,
	}
	for _, c := range a.Checks {
		if _, ok := want[c.Name]; ok {
			want[c.Name] = true
			if c.OK {
				t.Errorf("%q passed on a run that stopped with no PDF", c.Name)
			}
			if strings.Contains(c.Detail, "\n") {
				t.Errorf("%q detail runs to several lines: %q", c.Name, c.Detail)
			}
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("no check named %q, so a stopped run is still a silence", name)
		}
	}
}

func TestARunThatFinishedPassesBothOfThem(t *testing.T) {
	a := &Audit{}
	a.typeset(&Volume{Meta: corpus.Book{Pages: 144}},
		&Build{PDF: "book.pdf", Pages: 126},
		DefaultAuditOptions())
	for _, c := range a.Checks {
		switch c.Name {
		case "the typesetter ran to the end", "the typesetter wrote a PDF":
			if !c.OK {
				t.Errorf("%q failed on a good build: %s", c.Name, c.Detail)
			}
		}
	}
}

func TestTheFloorIsOffWhenItIsZero(t *testing.T) {
	// Zero has to mean no floor rather than a floor of nothing, since a build
	// that wants every volume however thin is a real thing to want and -floor 0
	// is how it is asked for. It returns before it reads the manifest, which is
	// also what lets the unit tests in here run with no corpus under them.
	if err := BelowFloor("/nonexistent", &Volume{Lang: "vi"}, 0); err != nil {
		t.Errorf("BelowFloor with the floor off = %v, want nil", err)
	}
}

func TestAVolumeTheSectionsManifestDoesNotKnowIsNotRefused(t *testing.T) {
	// A measurement nobody could take is not a reason to stop a build. The audit
	// reports the missing manifest as its own failed check, which is the right
	// place for it: the fault is in manifests/sections.yaml and not in the
	// volume, and refusing here would hide that behind a coverage number that
	// was never computed.
	v := &Volume{Lang: "vi", Meta: corpus.Book{ID: "no-such-volume", Lang: "fr"}}
	if err := BelowFloor("/nonexistent", v, 0.10); err != nil {
		t.Errorf("BelowFloor on a volume with no sections recorded = %v, want nil", err)
	}
}
