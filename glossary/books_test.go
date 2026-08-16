package glossary

import "testing"

// "order" is what forced a row to be able to name its volumes.
//
// Bare "order" is used 66 times in content/en/alg and 112 times in
// content/en/lie, where it is the order of a group or of an element and the
// word is cấp. It is used 133 times in content/en/ens, where it is an ordering
// and the word is thứ tự: 73 of those are inside "order relation", 20 inside
// "order-type". One row across the whole corpus has to be wrong for one of the
// two, and until this it was wrong for the volume whose chapter III is called
// Ordered sets.
func order() Glossary {
	return Glossary{Version: 27, Terms: []Term{
		{EN: "ring", VI: "vành", ZH: "环"},
		{EN: "order", VI: "cấp", ZH: "阶"},
		{EN: "order", VI: "thứ tự", ZH: "序", Books: []string{"ens"}},
	}}
}

func TestForGivesAVolumeTheRowScopedToIt(t *testing.T) {
	g := order().For("ens")
	if len(g.Terms) != 2 {
		t.Fatalf("Theory of Sets was given %d rows, want 2, one of them order", len(g.Terms))
	}
	if got := g.Terms[1].VI; got != "thứ tự" {
		t.Errorf("order is %q for Theory of Sets, want thứ tự", got)
	}
}

func TestForGivesEveryOtherVolumeTheDefaultRow(t *testing.T) {
	g := order().For("alg")
	if len(g.Terms) != 2 {
		t.Fatalf("Algebra was given %d rows, want 2", len(g.Terms))
	}
	if got := g.Terms[1].VI; got != "cấp" {
		t.Errorf("order is %q for Algebra, want cấp", got)
	}
}

// A scoped row wins wherever it is written, and the term keeps the place its
// first row had, so adding a scoped row at the end of the file does not shuffle
// the block that goes into a prompt.
func TestForDoesNotDependOnTheOrderTheRowsAreWrittenIn(t *testing.T) {
	g := Glossary{Terms: []Term{
		{EN: "order", VI: "thứ tự", Books: []string{"ens"}},
		{EN: "ring", VI: "vành"},
		{EN: "order", VI: "cấp"},
	}}
	out := g.For("ens")
	if len(out.Terms) != 2 || out.Terms[0].VI != "thứ tự" || out.Terms[1].VI != "vành" {
		t.Errorf("Theory of Sets was given %+v", out.Terms)
	}
}

func TestForCarriesTheVersion(t *testing.T) {
	if got := order().For("ens").Version; got != 27 {
		t.Errorf("version = %d, want 27, since it is what makes a translation stale", got)
	}
}

// Validate has to let the two order rows through and still catch the duplicate
// it was written for, which is the row nobody can see is dead.
func TestValidateAcceptsADefaultRowBesideAScopedOne(t *testing.T) {
	if err := order().Validate(); err != nil {
		t.Errorf("the two order rows were rejected: %v", err)
	}
}

func TestValidateRejectsTwoRowsThatAnswerForTheSameVolume(t *testing.T) {
	g := Glossary{Terms: []Term{
		{EN: "order", VI: "thứ tự", Books: []string{"ens", "ta"}},
		{EN: "order", VI: "cấp", Books: []string{"alg", "ens"}},
	}}
	if err := g.Validate(); err == nil {
		t.Error("two rows both answering for Theory of Sets were accepted, so one of them is dead")
	}
}

func TestValidateStillRejectsAPlainDuplicate(t *testing.T) {
	g := Glossary{Terms: []Term{{EN: "ring", VI: "vành"}, {EN: "Ring", VI: "nhẫn"}}}
	if err := g.Validate(); err == nil {
		t.Error("a duplicated term was accepted")
	}
}

// SameTerms is what moves the version, and it compares rows by English. Once
// two rows can share an English term it has to tell them apart by their books,
// or scoping a term would look like no change at all and every file translated
// against the old row would stay marked fresh.
func TestSameTermsSeesAScopedRowArrive(t *testing.T) {
	was := []Term{{EN: "order", VI: "cấp"}}
	now := []Term{{EN: "order", VI: "cấp"}, {EN: "order", VI: "thứ tự", Books: []string{"ens"}}}
	if SameTerms(was, now) {
		t.Error("adding the Theory of Sets row for order counted as no change")
	}
}

func TestSameTermsIgnoresTheBookOrderNothingElse(t *testing.T) {
	was := []Term{{EN: "order", VI: "thứ tự", Books: []string{"ens"}}}
	now := []Term{{EN: "order", VI: "thứ tự", Books: []string{"ens"}, Note: "an ordering, not the order of a group"}}
	if !SameTerms(was, now) {
		t.Error("writing down why the row is there counted as a change")
	}
	now[0].Books = []string{"ta"}
	if SameTerms(was, now) {
		t.Error("moving the row to another volume counted as no change")
	}
}
