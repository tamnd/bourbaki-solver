package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The contents is read off the Acrobat text layer the PDF carries, and that
// layer is the reason Theory of Sets is being read again: it turns "Methods"
// into "Met?ods" and "element" into "dement". The correction is applied to the
// page before the contents is parsed, so the generated manifest stays
// generated.
func TestCorrectContentsMendsTheLineBeforeItIsRead(t *testing.T) {
	pages := []string{
		"CONTENTS\n\n   3. Met?ods .of proof. ......  30\n",
		"the body of the volume, which also says dement here and there\n",
	}
	got, err := correctContents(pages, []corpus.Erratum{
		{Says: "3. Met?ods .of proof.", Read: "3. Methods of proof.", Why: "the text layer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got[0], "3. Methods of proof.") {
		t.Errorf("the contents page was not corrected:\n%s", got[0])
	}
	if !strings.Contains(got[1], "dement") {
		t.Error("the correction ran past the line it was written for")
	}
}

// An erratum nobody applied is a person having written down a correction in the
// belief that it was in force. A says that is not on any page, or is on two, is
// an error and not a warning, because the check it exempts is there to catch a
// page misread or a page missing.
func TestCorrectContentsRefusesAnErratumItCannotPlace(t *testing.T) {
	for _, c := range []struct{ name, says string }{
		{"on no page", "4. Nowhere in the volume"},
		{"on two pages", "1. Twice over"},
	} {
		t.Run(c.name, func(t *testing.T) {
			pages := []string{"1. Twice over 12\n", "1. Twice over 13\n"}
			_, err := correctContents(pages, []corpus.Erratum{{Says: c.says, Read: "x", Why: "y"}})
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), "want exactly one") {
				t.Errorf("the error does not say what it wanted: %v", err)
			}
		})
	}
}
