package ocr

import "testing"

// The four real cases, all of them from the three rounds over the flagged pages
// of Lie 7 to 9. Each is the smallest piece of a page that carries the defect.
func TestTheFacesThatMovedOnTheFlaggedPagesOfLieSevenToNine(t *testing.T) {
	cases := []struct {
		name string
		was  string
		now  string
		want FaceChange
	}{{
		// Page 340. The fraktur survived standing alone and not in a subscript,
		// so \lambda_{\mathfrak{g}} said something else.
		name: "a fraktur letter flattened inside a subscript",
		was:  `$\lambda_{\mathfrak{g}}(x) = \delta_{\mathfrak{g}}(x)$`,
		now:  `$\lambda_g(x) = \delta_g(x)$`,
		want: FaceChange{Face: "mathfrak", Letter: "g", Was: 2, Now: 0},
	}, {
		// Page 303. A classical group the book sets in bold, written upright.
		name: "a classical group written upright",
		was:  `$\mathbf{SU}(2,\mathbf{C})$`,
		now:  `$\mathrm{SU}(2,\mathbf C)$`,
		want: FaceChange{Face: "mathbf", Letter: "U", Was: 1, Now: 0},
	}, {
		// Page 139. Twelve italic capitals bolded, which turned the module
		// Z(\lambda-\rho) into the ring of integers.
		name: "an italic capital bolded into a number set",
		was:  `$Z(\lambda-\rho)$ and $Z(\mu-\rho)$`,
		now:  `$\mathbf{Z}(\lambda-\rho)$ and $\mathbf{Z}(\mu-\rho)$`,
		want: FaceChange{Face: "mathbf", Letter: "Z", Was: 0, Now: 2},
	}, {
		// Page 145. The symmetric algebra lost its bold.
		name: "the symmetric algebra lost its bold",
		was:  `$\mathbf{S}(\mathfrak{h})^W$`,
		now:  `$S(\mathfrak h)^W$`,
		want: FaceChange{Face: "mathbf", Letter: "S", Was: 1, Now: 0},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var found bool
			for _, change := range faceChanges(0, c.was, c.now) {
				if change.Face == c.want.Face && change.Letter == c.want.Letter {
					found = true
					if change.Was != c.want.Was || change.Now != c.want.Now {
						t.Errorf("%s was %d now %d, want was %d now %d",
							faceName(change.Face, change.Letter), change.Was, change.Now, c.want.Was, c.want.Now)
					}
				}
			}
			if !found {
				t.Errorf("nothing said about %s in %v",
					faceName(c.want.Face, c.want.Letter), faceChanges(0, c.was, c.now))
			}
		})
	}
}

// A face command takes a group or a bare letter and the two spell the same
// page. Counting groups rather than letters was the first way this was written
// and it reported three losses on lie-vii-ix that were not there, all of them
// the model joining a pair the extractor had split.
func TestJoiningTwoFrakturLettersIntoOneGroupIsNotAChange(t *testing.T) {
	was := `$\mathfrak{s}\mathfrak{u}(2,\mathbf{C})$`
	now := `$\mathfrak{su}(2,\mathbf C)$`
	if changes := faceChanges(310, was, now); len(changes) > 0 {
		t.Errorf("faceChanges = %v, want nothing", changes)
	}
}

// An operator name is a run of letters and not a variable that lost its face,
// and prose is not mathematics. Both would otherwise flood the report: a page
// carries a few hundred plain letters of English and a couple of dozen of
// \operatorname.
func TestProseAndOperatorNamesAreNotVariables(t *testing.T) {
	was := "The group Aut is a group. $\\operatorname{Aut}(\\mathfrak{g})$"
	now := "The group Aut is a group, said twice. $\\operatorname{Aut}(\\mathfrak{g})$"
	if changes := faceChanges(119, was, now); len(changes) > 0 {
		t.Errorf("faceChanges = %v, want nothing", changes)
	}
}

// Page 119, and the reason the report counts a letter only where a face moved.
// The pass exists to put back mathematics the text layer dropped, and the exact
// sequence below is one of the three rows of a commutative diagram that was not
// on the page before. Every letter in it is new, and the plain ones are not a
// typeface question: T, Q, f and A are new letters and not letters that lost a
// face. Counting them put fifty rows of drift into the eighteen real ones over
// the eleven pages of lie-vii-ix that shipped first.
//
// The bolded R is a question, and a real one. The book sets a root system in
// italic and says so in the prompt, and the model bolded it anyway inside the
// diagram it had just restored.
func TestTheLettersOfARestoredDiagramAreNotFaceChanges(t *testing.T) {
	was := `The exact sequences give the result.`
	now := was + "\n" + `$$1\longrightarrow T_Q\xrightarrow{\ f\ }` +
		`\operatorname{Aut}(\mathfrak g,\mathfrak h)\xrightarrow{\ \varepsilon\ }` +
		`A(\mathbf R)\longrightarrow 1$$`
	changes := faceChanges(119, was, now)
	for _, change := range changes {
		if change.Face == plainFace {
			t.Errorf("%s, and a letter a diagram brought with it did not lose a face", change)
		}
	}
	if len(changes) != 3 || changes[0].Face != "mathbf" || changes[0].Letter != "R" {
		t.Errorf("faceChanges = %v, want the bold R and the fraktur pair the diagram carries", changes)
	}
}

// A page read from a scan has no text layer behind it, so there is no second
// reading to compare against and the whole question is moot. That is decided in
// write, by the method of the page being replaced, but the shape of a run over
// a page with nothing before it is worth pinning here.
func TestAPageWithNoReadingBeforeItReportsNothing(t *testing.T) {
	if changes := faceChanges(12, "", `$\mathbf{Z}$ and $\mathfrak{g}$`); len(changes) != 2 {
		t.Errorf("faceChanges = %v, want the two faces of the new reading", changes)
	}
}

// Page 76 of Algebra VIII is the shape of it: thirty script capitals in the
// native reading, thirty calligraphic in the reading that replaced it, and
// nothing else about the mathematics changed.
func TestAScriptCapitalIsPutBackWhenAPictureSpelledItCalligraphic(t *testing.T) {
	was := `Relations between $\mathscr{T}$ and $\mathscr{H}$, where $\mathscr{T}$ is a topology.`
	now := `Relations between $ \mathcal{T} $ and $ \mathcal{H} $, where $ \mathcal{T} $ is a topology.`
	got := restoreScript(was, now)
	want := `Relations between $ \mathscr{T} $ and $ \mathscr{H} $, where $ \mathscr{T} $ is a topology.`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// Only that direction, that pair, and only for a letter the native reading
// settled. Everything else is left for the report to raise with a human.
func TestRestoreScriptIsNarrow(t *testing.T) {
	for _, c := range []struct {
		why      string
		was, now string
		want     string
	}{
		{"a letter the native reading never set in script",
			`$\mathscr{T}$`, `$\mathcal{F}$`, `$\mathcal{F}$`},
		{"the other direction is never taken",
			`$\mathcal{T}$`, `$\mathscr{T}$`, `$\mathscr{T}$`},
		{"a letter the native reading set both ways is a real distinction",
			`$\mathscr{C}$ and $\mathcal{C}$`, `$\mathcal{C}$`, `$\mathcal{C}$`},
		{"another face is not this rule's business",
			`$\mathscr{T}$ and $\mathbf{R}$`, `$\mathcal{T}$ and $\mathrm{R}$`,
			`$\mathscr{T}$ and $\mathrm{R}$`},
		{"a group of two letters is not a script capital",
			`$\mathscr{T}$`, `$\mathcal{Tr}$`, `$\mathcal{Tr}$`},
		{"the unbraced spelling is reached and normalised",
			`$\mathscr{T}$`, `$\mathcal T$`, `$\mathscr{T}$`},
		{"a native reading with no script capital changes nothing",
			`$\mathbf{R}$`, `$\mathcal{T}$`, `$\mathcal{T}$`},
	} {
		t.Run(c.why, func(t *testing.T) {
			if got := restoreScript(c.was, c.now); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

// Without mathcal among the faces, a letter moving between the two script
// spellings reported as a loss with nothing gaining it.
func TestASwapBetweenTheTwoScriptFacesIsReportedAsBoth(t *testing.T) {
	changes := faceChanges(76, `$\mathscr{T}$ $\mathscr{T}$`, `$\mathcal{T}$ $\mathcal{T}$`)
	got := map[string][2]int{}
	for _, c := range changes {
		got[c.Face] = [2]int{c.Was, c.Now}
	}
	if got["mathscr"] != [2]int{2, 0} {
		t.Errorf("mathscr = %v, want the two it lost", got["mathscr"])
	}
	if got["mathcal"] != [2]int{0, 2} {
		t.Errorf("mathcal = %v, want the two it gained", got["mathcal"])
	}
}
