package textguard

import "testing"

func TestStrandedBaseTakesTheLetterTheProseWasHolding(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"sends S to cl(S$^*$) is a bijection", "sends S to cl($S^*$) is a bijection"},
		{"then we have Tr(A$^*$) $=$ Tr(A).", "then we have Tr($A^*$) $=$ Tr(A)."},
		{"conjugate under Ad(G$^*$).", "conjugate under Ad($G^*$)."},
		{"to a subset of E (resp. E$^*$): II, § 2", "to a subset of E (resp. $E^*$): II, § 2"},
		{"Soit $u\\in$ dom(D$^*$) tel que", "Soit $u\\in$ dom($D^*$) tel que"},
	} {
		got, n := StrandedBase(c.in)
		if got != c.want || n != 1 {
			t.Errorf("StrandedBase(%q)\n got %q, %d\nwant %q, 1", c.in, got, n, c.want)
		}
	}
}

func TestStrandedBaseLeavesABaseItCannotSeeAllOf(t *testing.T) {
	// Two letters: the dual is of QQ and of BA, and taking the nearer letter
	// would write a dual of Q and of A, which is a different thing.
	for _, in := range []string{
		"Les germes $Q^*(z)$ et (QQ$^*$)$(z)$ sont inversibles",
		"=d(u$)Tr(BA$^*$), (1)",
		// A bracket, a brace and a closing delimiter are all a base this rule
		// declines to guess the extent of.
		"dom(($u_{(\\mathbf{C})}$)$^*$) et que",
		"INTO S($\\mathfrak{h}$)$^*$",
		"sur son dual (End(V))*$^*$. Identifiant",
	} {
		got, n := StrandedBase(in)
		if n != 0 || got != in {
			t.Errorf("StrandedBase(%q) = %q, %d; want it untouched", in, got, n)
		}
	}
}

func TestStrandedBaseLeavesTheMarkAndTheMathematicsAlone(t *testing.T) {
	for _, in := range []string{
		// The forward-reference mark, which is ScriptStars's and comes out as \*.
		"\\*The set $\\mathbf{R}$ is ordered.\\*",
		// A dual already written whole.
		"the mapping $S\\mapsto \\mathrm{cl}(S^*)$ is a bijection",
		// A pushforward, which is a script on a base and not this at all.
		"the map $(p_1)_*$ is surjective",
		// A display is left to whoever set it as a display.
		"$$\\mathrm{Tr}(A^*) = \\mathrm{Tr}(A)$$",
	} {
		got, n := StrandedBase(in)
		if n != 0 || got != in {
			t.Errorf("StrandedBase(%q) = %q, %d; want it untouched", in, got, n)
		}
	}
}

func TestStrandedBaseTakesEveryOneOnALine(t *testing.T) {
	in := "cl(S$^*$) and Tr(A$^*$) and ch(E$^*$)"
	want := "cl($S^*$) and Tr($A^*$) and ch($E^*$)"
	got, n := StrandedBase(in)
	if got != want || n != 3 {
		t.Errorf("StrandedBase(%q)\n got %q, %d\nwant %q, 3", in, got, n, want)
	}
}

func TestStrandedBaseRunTwiceChangesNothingTheSecondTime(t *testing.T) {
	once, n := StrandedBase("the mapping that sends S to cl(S$^*$) is a bijection")
	if n != 1 {
		t.Fatalf("first pass moved %d bases, want 1", n)
	}
	twice, n := StrandedBase(once)
	if n != 0 || twice != once {
		t.Errorf("second pass = %q, %d; want it untouched", twice, n)
	}
}
