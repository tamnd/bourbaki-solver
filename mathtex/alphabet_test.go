package mathtex

import "testing"

func TestAlphabet(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want string
		n    int
	}{
		{"a letter on its own",
			`Il existe alors un idéal 𝔅 ≠ 0 de A`,
			`Il existe alors un idéal $\mathfrak{B}$ ≠ 0 de A`, 1},
		{"the argument comes inside the span",
			`considered as a subset of 𝒫(E), is called`,
			`considered as a subset of $\mathscr{P}(E)$, is called`, 1},
		{"a comma and a space inside the argument",
			`of W into 𝓛(E_0, M) and 𝓛(M, E_0) are`,
			`of W into $\mathscr{L}(E_0, M)$ and $\mathscr{L}(M, E_0)$ are`, 2},
		{"the accent over the letter",
			"The partition 𝔖̃ which it defines",
			`The partition $\mathfrak{S}` + "̃" + `$ which it defines`, 1},
		{"a subscript and a superscript",
			`the space 𝒞^r_b of functions`,
			`the space $\mathscr{C}^r_b$ of functions`, 1},
		{"the script small l has a command of its own",
			`une forme linéaire sur ℓ(G)`,
			`une forme linéaire sur $\ell(G)$`, 1},
		{"double-struck is bold, which is M02's answer",
			`de l’intervalle [0, f(b)] de ℝ est dense`,
			`de l’intervalle [0, f(b)] de $\mathbf{R}$ est dense`, 1},
		{"bold script is script",
			`the hyperplanes H ∈ 𝓗 are locally finite`,
			`the hyperplanes H ∈ $\mathscr{H}$ are locally finite`, 1},
		{"inside the mathematics it is only spelled",
			`soit $x \in ℓ_\infty^\infty(G)$ une forme`,
			`soit $x \in \ell_\infty^\infty(G)$ une forme`, 1},
		{"a command must not run into the letter after it",
			`on a $ℓn = 0$ et $ℤn = 0$ pour tout`,
			`on a $\ell n = 0$ et $\mathbf{Z}n = 0$ pour tout`, 2},
		{"a display too",
			"$$\nE = (ℓ_1^1(G))^k\n$$",
			"$$\nE = (\\ell_1^1(G))^k\n$$", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, n, _ := Alphabet(c.in)
			if got != c.want {
				t.Errorf("Alphabet(%q)\n got %q\nwant %q", c.in, got, c.want)
			}
			if n != c.n {
				t.Errorf("Alphabet(%q) counted %d, want %d", c.in, n, c.n)
			}
		})
	}
}

// What is already mathematics is already right, and what the repair cannot
// vouch for it leaves where a reader can find it.
func TestAlphabetLeavesAlone(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
	}{
		{"inside an inline span", `the set $\mathscr{P}(E)$ of subsets`},
		{"what is already spelled", `the set $x \in \mathscr{P}$ of`},
		{"nothing of the block at all", `the ordinary letters P and B and g`},
		{"past a delimiter that never closes", `a $ b 𝒫(E) c`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, n, _ := Alphabet(c.in)
			if got != c.in || n != 0 {
				t.Errorf("Alphabet(%q) = %q, %d; want it untouched", c.in, got, n)
			}
		})
	}
}

// The argument is taken only when it is plainly the letter's. Anything else
// stays outside, because pulling a bare Greek or a bare infinity inside a span
// hands M03 a character stranded in the mathematics.
func TestAlphabetKeepsAnUnvouchedArgumentOutside(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`sur ℓ_∞^∞(G) une forme`, `sur $\ell$_∞^∞(G) une forme`},
		{`la tribu 𝒞(α) de T`, `la tribu $\mathscr{C}$(α) de T`},
		{"the set 𝒫(E\nand F) here", "the set $\\mathscr{P}$(E\nand F) here"},
	} {
		got, n, _ := Alphabet(c.in)
		if got != c.want || n != 1 {
			t.Errorf("Alphabet(%q) = %q, %d; want %q, 1", c.in, got, n, c.want)
		}
	}
}

// The two letters that are the wrong letter rather than the wrong font are
// refused by name and reported, the way Repair refuses a stranded sigma.
func TestAlphabetRefusesTheWrongLetter(t *testing.T) {
	in := `toute fonction définie sur la tribu borélienne 𝔽(T) de T`
	got, n, refused := Alphabet(in)
	if got != in || n != 0 {
		t.Fatalf("Alphabet(%q) = %q, %d; want it untouched", in, got, n)
	}
	if len(refused) != 1 {
		t.Fatalf("refused %d, want 1", len(refused))
	}
	if refused[0].Rune != '𝔽' {
		t.Errorf("refused %q, want %q", refused[0].Rune, '𝔽')
	}
	if refused[0].Line != 1 {
		t.Errorf("refused on line %d, want 1", refused[0].Line)
	}
}

func TestAlphabetCountsTheLine(t *testing.T) {
	_, _, refused := Alphabet("one\ntwo\nla tribu 𝔽(T) de T\n")
	if len(refused) != 1 || refused[0].Line != 3 {
		t.Fatalf("refused %+v, want one on line 3", refused)
	}
}
