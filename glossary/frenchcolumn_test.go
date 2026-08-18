package glossary

import "strings"

import "testing"

// The French column is the one place where the English coming back unchanged is
// often the truth rather than a model failing to answer. Refusing those would
// empty the column of the terms both printings agree about.
func TestAFrenchTermSpelledAsTheEnglishIsKeptAndFlagged(t *testing.T) {
	b := Batch{Lang: "fr", Terms: []string{"module", "ring"}}
	reply := b.Audit("1 | module | module\n2 | ring | anneau\n")
	if len(reply.Rows) != 2 {
		t.Fatalf("kept %d rows, want 2: %+v", len(reply.Rows), reply.Rejects)
	}
	if len(reply.Suspect) != 1 || reply.Suspect[0].EN != "module" {
		t.Fatalf("flagged %+v, want module alone", reply.Suspect)
	}
}

// The same answer in a language somebody is being asked to write is a model
// that did not answer, and it is still refused there.
func TestTheEnglishComingBackIsStillRefusedInVietnamese(t *testing.T) {
	b := Batch{Lang: "vi", Terms: []string{"module"}}
	reply := b.Audit("1 | module | module\n")
	if len(reply.Rows) != 0 || len(reply.Rejects) != 1 {
		t.Fatalf("kept %+v, refused %+v, want the row refused", reply.Rows, reply.Rejects)
	}
}

// A French question has to say that the word is a fact of the printing and not
// a rendering to be chosen, or the model answers the wrong question well.
func TestTheFrenchQuestionAsksWhatBourbakiPrinted(t *testing.T) {
	fr := Batch{Lang: "fr", Terms: []string{"ring"}}.Prompt()
	if !strings.Contains(fr, "the word Bourbaki printed") {
		t.Fatalf("the french question does not say what it wants:\n%s", fr)
	}
	if vi := (Batch{Lang: "vi", Terms: []string{"ring"}}).Prompt(); strings.Contains(vi, "Bourbaki printed") {
		t.Fatalf("the vietnamese question carries the french note:\n%s", vi)
	}
}

// fr is a column a pass may write and en is not, whatever else changes.
func TestTheHeadwordIsNotFillableAndTheFrenchIs(t *testing.T) {
	for _, l := range []string{"vi", "zh", "ja", "fr"} {
		if !Fillable(l) {
			t.Errorf("%s is not fillable, and every column but the headword is", l)
		}
	}
	if Fillable("en") {
		t.Error("english is fillable, and it is the headword the row hangs off")
	}
}
