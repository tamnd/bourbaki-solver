package translate

import "testing"

// The shape exercise 11 of Commutative Algebra III § 3 came back in, twice, and
// the shapes around it. A letter is written into the prose in the right place,
// spelled right, without the four characters that say it is mathematics.
func TestRedollarPutsTheDollarsBack(t *testing.T) {
	for _, c := range []struct{ name, en, tr, want string }{
		{
			"one letter lost between two spans that are still there",
			`the subgroup $H$ of $E$ is closed`,
			`nhóm con H của $E$ là đóng`,
			`nhóm con $H$ của $E$ là đóng`,
		},
		{
			"a run of losses in one gap",
			`$A$ and $H$ and $p$ and $L$ here`,
			`$A$ và H và p và L đây`,
			`$A$ và $H$ và $p$ và $L$ đây`,
		},
		{
			"a loss before the first span that survives",
			`$H$ is a subgroup of $E$`,
			`H là nhóm con của $E$`,
			`$H$ là nhóm con của $E$`,
		},
		{
			"a loss after the last span that survives",
			`$E$ contains $H$`,
			`$E$ chứa H`,
			`$E$ chứa $H$`,
		},
		{
			"a span that survived but was laid out differently is not a loss",
			`$M\cap N$ and $H$ and $E$`,
			`$M \cap N$ và H và $E$`,
			`$M \cap N$ và $H$ và $E$`,
		},
		{
			"a letter standing against the span it neighbours",
			`$E$, $H$ and $L$`,
			`$E$,H và $L$`,
			`$E$,$H$ và $L$`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Redollar(c.en, c.tr); got != c.want {
				t.Errorf("Redollar gave\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}

// Everything the repair must not touch. It is led by the English throughout, and
// where anything does not line up it does nothing at all and leaves RuleMath to
// report what it was always going to. A repair that guessed would write the
// wrong mathematics into the corpus, which is worse than the refusal it saves.
func TestRedollarLeavesAnythingElseAlone(t *testing.T) {
	for _, c := range []struct{ name, en, tr string }{
		{
			"nothing was lost",
			`the subgroup $H$ of $E$`,
			`nhóm con $H$ của $E$`,
		},
		{
			"the answer has more spans than the English",
			`the subgroup $H$ of E`,
			`nhóm con $H$ của $E$`,
		},
		{
			"a delimiter left open in the answer",
			`$H$ and $E$`,
			`$H và $E$`,
		},
		{
			"the letter is not written in the prose at all",
			`the subgroup $H$ of $E$`,
			`nhóm con của $E$`,
		},
		{
			"the letter stands twice in the one gap",
			`$A$ and $p$ and $B$`,
			`$A$ và p và p và $B$`,
		},
		{
			"the losses are in two gaps with a surviving span between them",
			`$p$ divides $a$ and $p$ divides $b$`,
			`p chia $a$ và p chia $b$`,
		},
		{
			"a surviving span sits between two losses, so which is which is a guess",
			`$H$ and $p$ and $E$ and $L$`,
			`H và $p$ và E và L`,
		},
		{
			"the letter is only inside a Vietnamese word",
			`the set $a$ is open`,
			`tập và là mở`,
		},
		{
			"the letter stands only inside an attribute block",
			`the set $a$ is open`,
			`tập mở {#a}`,
		},
		{
			"a display the answer dropped, with no bare copy to find",
			"Ta có\n\n$$A \\subset B.$$\n",
			"Ta có\n\n",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Redollar(c.en, c.tr); got != c.tr {
				t.Errorf("Redollar changed\n%q\nto\n%q", c.tr, got)
			}
		})
	}
}
