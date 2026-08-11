package glossary

import (
	"strings"
	"testing"
)

// The Vietnamese below is real Vietnamese mathematics and the terms are real
// Bourbaki terms, because an audit of renderings cannot be tested with invented
// ones: the whole question is whether a rendering is in the right script and is
// one phrase rather than two.
//
// Every case here is a way a browser answer has actually gone wrong, or a way
// one plainly can. The comments say which.

func batch() Batch {
	return Batch{Lang: "vi", Terms: []string{"ring", "left ideal", "free module"}}
}

func TestAGoodAnswerIsAccepted(t *testing.T) {
	reply := batch().Audit("1 | ring | vành\n2 | left ideal | iđêan trái\n3 | free module | môđun tự do\n")
	if len(reply.Rejects) != 0 {
		t.Fatalf("a good answer was rejected: %+v", reply.Rejects)
	}
	if len(reply.Rows) != 3 {
		t.Fatalf("got %d rows: %+v", len(reply.Rows), reply.Rows)
	}
	if reply.Rows[1].EN != "left ideal" || reply.Rows[1].TR != "iđêan trái" {
		t.Errorf("row 2 is %+v", reply.Rows[1])
	}
}

// The failure that makes every other check worth having. A model that drops one
// term and shifts the rest up by one produces lines that are each plausible and
// each attached to the wrong words, and only the English column catches it.
func TestARenumberedAnswerIsCaught(t *testing.T) {
	reply := batch().Audit("1 | ring | vành\n2 | free module | môđun tự do\n3 | ideal | iđêan\n")
	if len(reply.Rows) != 1 || reply.Rows[0].EN != "ring" {
		t.Fatalf("a shifted answer was accepted: %+v", reply.Rows)
	}
	var found bool
	for _, r := range reply.Rejects {
		if strings.Contains(r.Reason, `answers "free module", not "left ideal"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("the shift was not reported as one: %+v", reply.Rejects)
	}
}

// A term that was not answered is a term to ask about again, and the caller can
// only do that if it is told which one.
func TestAMissingTermIsReported(t *testing.T) {
	reply := batch().Audit("1 | ring | vành\n3 | free module | môđun tự do\n")
	if len(reply.Rejects) != 1 || reply.Rejects[0].EN != "left ideal" {
		t.Fatalf("the missing term was not named: %+v", reply.Rejects)
	}
	if !strings.Contains(reply.Rejects[0].Reason, "not answered") {
		t.Errorf("the reason is %q", reply.Rejects[0].Reason)
	}
}

// The escape hatch. A model told it may say it does not know says so instead of
// inventing, and a term nobody can render is a term for a person to write by
// hand rather than a failure to retry.
func TestUnknownIsNotARejection(t *testing.T) {
	reply := batch().Audit("1 | ring | vành\n2 | left ideal | UNKNOWN\n3 | free module | môđun tự do\n")
	if len(reply.Unknown) != 1 || reply.Unknown[0] != "left ideal" {
		t.Fatalf("unknown was not recorded: %+v", reply.Unknown)
	}
	for _, r := range reply.Rejects {
		if r.EN == "left ideal" {
			t.Errorf("saying it does not know was counted against it: %+v", r)
		}
	}
}

// English that came back as English is the failure L07 exists for, one term at
// a time. In Vietnamese the test is the diacritics, which the language uses in
// almost every phrase and English uses in none.
func TestTheEnglishComingBackIsRefused(t *testing.T) {
	reply := batch().Audit("1 | ring | ring\n2 | left ideal | left ideal\n3 | free module | môđun tự do\n")
	if len(reply.Rows) != 1 {
		t.Fatalf("untranslated English was accepted: %+v", reply.Rows)
	}
	if len(reply.Rejects) != 2 {
		t.Fatalf("got %+v", reply.Rejects)
	}
}

// Measured, and the first thing the first real run taught this. Forty terms
// into Vietnamese on 11 August 2026 produced one rendering with no diacritic in
// it: "generated" as "sinh", which is the correct word. A rule that threw that
// away would have been throwing away correct terminology at one term in forty,
// so a diacritic-free Vietnamese rendering is kept and flagged.
func TestADiacriticFreeVietnameseRenderingIsKeptAndFlagged(t *testing.T) {
	b := Batch{Lang: "vi", Terms: []string{"generated"}}
	reply := b.Audit("1 | generated | sinh\n")
	if len(reply.Rows) != 1 || reply.Rows[0].TR != "sinh" {
		t.Fatalf("a correct Vietnamese word was thrown away: %+v", reply.Rejects)
	}
	if len(reply.Suspect) != 1 || reply.Suspect[0].EN != "generated" {
		t.Errorf("it was not flagged for a person: %+v", reply.Suspect)
	}
}

// The same leniency is wrong for Chinese and Japanese, where a rendering with
// no Han and no kana in it is English that came back untouched.
func TestAChineseAnswerWithNoHanIsRefused(t *testing.T) {
	b := Batch{Lang: "zh", Terms: []string{"ring"}}
	reply := b.Audit("1 | ring | vanh\n")
	if len(reply.Rows) != 0 {
		t.Fatalf("a rendering with nothing Chinese in it was accepted: %+v", reply.Rows)
	}
	if !strings.Contains(reply.Rejects[0].Reason, "nothing of zh") {
		t.Errorf("the reason is %q", reply.Rejects[0].Reason)
	}
}

// One rendering was asked for. Picking one of two here would be this program
// guessing at terminology, which is the one thing it must not do.
func TestTwoRenderingsAreRefused(t *testing.T) {
	reply := batch().Audit("1 | ring | vành / nhẫn\n2 | left ideal | iđêan trái (bên trái)\n3 | free module | môđun tự do\n")
	if len(reply.Rows) != 1 || reply.Rows[0].EN != "free module" {
		t.Fatalf("an answer that hedged was accepted: %+v", reply.Rows)
	}
}

// A model that decided to explain the notion instead of naming it. Six words is
// past anything a noun phrase needs in these three languages and well short of
// a sentence.
func TestAnExplanationIsRefused(t *testing.T) {
	b := Batch{Lang: "vi", Terms: []string{"ring"}}
	reply := b.Audit("1 | ring | một tập hợp có hai phép toán cộng và nhân thoả mãn\n")
	if len(reply.Rows) != 0 {
		t.Fatalf("a sentence was accepted as a term: %+v", reply.Rows)
	}
	if !strings.Contains(reply.Rejects[0].Reason, "explanation") {
		t.Errorf("the reason is %q", reply.Rejects[0].Reason)
	}
}

// Mathematics is not translated and not dropped. "$A$-module" has to come back
// with the $A$ still in it, whatever happened to the word.
func TestTheMathematicsHasToSurvive(t *testing.T) {
	b := Batch{Lang: "vi", Terms: []string{"$A$-module"}}
	if reply := b.Audit("1 | $A$-module | môđun trên vành\n"); len(reply.Rows) != 0 {
		t.Fatalf("a rendering that lost the formula was accepted: %+v", reply.Rows)
	}
	if reply := b.Audit("1 | $A$-module | $A$-môđun\n"); len(reply.Rows) != 1 {
		t.Fatalf("a rendering that kept the formula was refused: %+v", reply.Rejects)
	}
}

// What actually comes back from a browser: a markdown table, bold on a column,
// "1." for the number. That is the same answer and throwing a term away over it
// would be a poor reason.
func TestTheTidyingAModelDoesIsRead(t *testing.T) {
	reply := batch().Audit("| 1. | **ring** | `vành` |\n| 2) | left ideal | \"iđêan trái\" |\n|3| free module | môđun tự do |\n")
	if len(reply.Rows) != 3 {
		t.Fatalf("got %d rows, rejects %+v", len(reply.Rows), reply.Rejects)
	}
	if reply.Rows[0].TR != "vành" {
		t.Errorf("the decoration was left on: %q", reply.Rows[0].TR)
	}
}

// The closing paragraph. It is reported once rather than once per line, because
// what a caller needs to know is that the model wrote prose, not how much.
func TestCommentaryIsReportedOnce(t *testing.T) {
	reply := batch().Audit("1 | ring | vành\n2 | left ideal | iđêan trái\n3 | free module | môđun tự do\n\n" +
		"I have translated all three terms using standard\nVietnamese mathematical terminology.\n")
	if len(reply.Rows) != 3 {
		t.Fatalf("the rows were lost: %+v", reply.Rejects)
	}
	var n int
	for _, r := range reply.Rejects {
		if r.Reason == "not a row" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the commentary was reported %d times, want once: %+v", n, reply.Rejects)
	}
}

// Two notions really can share a word, so this is a flag and not a refusal. It
// is the one check that needs the whole batch rather than one line.
func TestARepeatedRenderingIsFlaggedAndKept(t *testing.T) {
	b := Batch{Lang: "vi", Terms: []string{"ring", "annulus"}}
	reply := b.Audit("1 | ring | vành\n2 | annulus | vành\n")
	if len(reply.Rows) != 2 {
		t.Fatalf("a collision threw a row away: %+v", reply.Rejects)
	}
	if len(reply.Collisions) != 1 || len(reply.Collisions[0].EN) != 2 {
		t.Fatalf("the collision was not flagged: %+v", reply.Collisions)
	}
}

func TestATermAnsweredTwiceIsTakenOnce(t *testing.T) {
	b := Batch{Lang: "vi", Terms: []string{"ring"}}
	reply := b.Audit("1 | ring | vành\n1 | ring | nhẫn\n")
	if len(reply.Rows) != 1 || reply.Rows[0].TR != "vành" {
		t.Fatalf("got %+v", reply.Rows)
	}
	if len(reply.Rejects) != 1 || reply.Rejects[0].Reason != "answered twice" {
		t.Errorf("the second answer was not reported: %+v", reply.Rejects)
	}
}

// A line numbered past the end of the batch is a model answering a question it
// was not asked, and the terms it would land on belong to another batch.
func TestALineOutsideTheBatchIsRefused(t *testing.T) {
	b := Batch{Lang: "vi", Terms: []string{"ring"}}
	reply := b.Audit("1 | ring | vành\n2 | left ideal | iđêan trái\n")
	if len(reply.Rows) != 1 {
		t.Fatalf("got %+v", reply.Rows)
	}
	if !strings.Contains(reply.Rejects[0].Reason, "this batch has 1 terms") {
		t.Errorf("the reason is %q", reply.Rejects[0].Reason)
	}
}

func TestThePromptCarriesEveryTermNumbered(t *testing.T) {
	p := batch().Prompt()
	for _, want := range []string{"1 | ring", "2 | left ideal", "3 | free module", "Vietnamese", "UNKNOWN"} {
		if !strings.Contains(p, want) {
			t.Errorf("the prompt does not contain %q", want)
		}
	}
	// The language is spelled out. "vi" is a thing a model has to guess at.
	if strings.Contains(p, "{{") {
		t.Error("a placeholder was left in the prompt")
	}
}

func TestBatchesCoverEveryTermExactlyOnce(t *testing.T) {
	terms := make([]string, 0, 95)
	for i := range 95 {
		terms = append(terms, string(rune('a'+i%26))+strings.Repeat("x", i/26))
	}
	bs := Batches("vi", terms, 40)
	if len(bs) != 3 {
		t.Fatalf("got %d batches", len(bs))
	}
	var n int
	for _, b := range bs {
		n += len(b.Terms)
	}
	if n != len(terms) {
		t.Errorf("the batches hold %d of %d terms", n, len(terms))
	}
	if len(bs[2].Terms) != 15 {
		t.Errorf("the last batch holds %d", len(bs[2].Terms))
	}
}

// A rendering already in the file is left alone. Whatever is there was either
// curated by a person or accepted by an earlier run, and a later run
// overwriting it would make the file depend on the order batches finished in.
func TestMergeDoesNotOverwriteWhatIsThere(t *testing.T) {
	g := &Glossary{Version: 1, Terms: []Term{{EN: "ring", VI: "vành"}}}
	added, kept := g.Merge("vi", []Row{{EN: "Ring", TR: "nhẫn"}, {EN: "left ideal", TR: "iđêan trái"}})
	if added != 1 || kept != 1 {
		t.Fatalf("added %d, kept %d", added, kept)
	}
	if g.Terms[0].VI != "vành" {
		t.Errorf("the curated rendering was overwritten with %q", g.Terms[0].VI)
	}
	if len(g.Terms) != 2 || g.Terms[1].VI != "iđêan trái" {
		t.Errorf("the new term is %+v", g.Terms)
	}
	if err := g.Validate(); err != nil {
		t.Errorf("the merge left the glossary invalid: %v", err)
	}
}

// A second language fills the same row rather than adding one.
func TestMergeFillsTheOtherLanguageInPlace(t *testing.T) {
	g := &Glossary{Version: 1, Terms: []Term{{EN: "ring", VI: "vành"}}}
	if added, _ := g.Merge("ja", []Row{{EN: "ring", TR: "環"}}); added != 1 {
		t.Fatalf("added %d", added)
	}
	if len(g.Terms) != 1 || g.Terms[0].JA != "環" || g.Terms[0].VI != "vành" {
		t.Errorf("got %+v", g.Terms)
	}
}
