package glossary

import "testing"

// The French column is looked up by the French and not by the English.
//
// Every other column is a target: the English is the headword and the row says
// what a translation of it has to say. The French runs the other way, because
// what is in hand when a French passage is read is the French phrase, and the
// row it belongs to is what has to be found. A lookup keyed by the English
// would never match anything a French page prints.
func TestAFrenchTermFindsItsRowByTheFrench(t *testing.T) {
	g := Glossary{Terms: []Term{
		{EN: "Artinian ring", FR: "anneau artinien", VI: "vanh Artin"},
		{EN: "semisimple ring", FR: "anneau semi-simple"},
		{EN: "ring", FR: "anneau"},
		{EN: "left ideal"}, // no French yet, and that is not an error
	}}
	byFrench := g.Keyed("fr")
	if len(byFrench) != 3 {
		t.Fatalf("Keyed(fr) has %d rows, want 3", len(byFrench))
	}
	row, ok := byFrench["anneau artinien"]
	if !ok {
		t.Fatalf("anneau artinien is not in %v", byFrench)
	}
	if row.EN != "Artinian ring" {
		t.Errorf("anneau artinien reads to %q, want Artinian ring", row.EN)
	}
	if _, ok := byFrench["left ideal"]; ok {
		t.Error("a row with no French is in the French lookup")
	}
}

// The French is matched the way every other term is, so the phrase at the head
// of a sentence and the phrase inside one are one term.
func TestTheFrenchIsMatchedWithoutRegardToCase(t *testing.T) {
	g := Glossary{Terms: []Term{{EN: "Artinian ring", FR: "Anneau  artinien"}}}
	if _, ok := g.Keyed("fr")["anneau artinien"]; !ok {
		t.Errorf("the French did not fold to its key: %v", g.Keyed("fr"))
	}
}

// English is the headword and nothing may overwrite it.
func TestTheEnglishHeadwordIsNotSettable(t *testing.T) {
	term := Term{EN: "Artinian ring"}
	term.Set("en", "something else")
	if term.EN != "Artinian ring" {
		t.Errorf("the headword was rewritten to %q", term.EN)
	}
	term.Set("fr", "anneau artinien")
	if term.FR != "anneau artinien" {
		t.Errorf("the French was not written, it reads %q", term.FR)
	}
}
