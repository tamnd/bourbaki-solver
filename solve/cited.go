package solve

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/textguard"
)

// A solution ends with a USES line naming the tags it leaned on, and nothing
// until now checked that those tags were ever in front of the model.
//
// Exercise 2 of § 1 is what taught this. It is about a nilpotent endomorphism of
// a module of finite length, and two of the three candidates ended with
// "USES: 00QM, 00QN". Both tags are real. They are Theorem 1 and Corollary 2 of
// Appendix 3, which is Hilbert's Nullstellensatz, and neither was in the
// question, in the § or anywhere near the subject. The selector took one of the
// two, the truth judge read the solution, found the mathematics sound, and
// failed the whole thing on the last line: "these identifiers are not resolved
// to named results in the section, so the citation is not honest or
// verifiable". It was right to. The cost was a correction call, a second pair of
// judge calls and an exercise that did not verify, all of it spent on four
// characters of bookkeeping that this package printed and can therefore check.
//
// The check is the question and not the corpus. A tag that exists somewhere in
// the Éléments but was not carried into this question is a tag the model did not
// read, and a citation to a thing not read is a guess whichever way it lands.
// Both judge prompts already say what to do instead: name it as standard, in
// words.

// shownTag finds the tags a question carries, in the two forms a question
// writes them: "tag=00QV" in the heading of a statement that is printed, and
// "tag 00QV" in the line that stands where one was left out for length or for
// the cap.
//
// Both count. A statement the question named and did not print is a statement
// the model did not read, but the question is what handed it the four
// characters, and refusing a citation to a name we ourselves gave would be
// punishing the model for using it. What is refused is a tag the question does
// not mention at all, which the model can only have got from somewhere else.
//
// This reads the question that was sent rather than the context it was built
// from, so what the length limit cut out is cut out here too.
var shownTag = regexp.MustCompile(`tag[= ]([0-9A-Z]{4})`)

// shown is the tags a question carries.
func shown(question string) map[string]bool {
	out := map[string]bool{}
	for _, m := range shownTag.FindAllStringSubmatch(question, -1) {
		out[m[1]] = true
	}
	return out
}

// wantSolutionFrom is wantSolution, plus the citations being ones the question
// carried.
//
// It is a guard and not a repair. Quietly dropping the tags would leave a
// solution whose sentences still lean on results it never saw, said in words
// where the line is not being checked, and the model is the only thing that
// knows which of the two it did.
func wantSolutionFrom(question string) func(string) error {
	have := shown(question)
	return func(text string) error {
		if err := wantSolution(text); err != nil {
			return err
		}
		uses, _ := textguard.Uses(text)
		var bad []string
		for _, u := range uses {
			if !have[u] {
				bad = append(bad, u)
			}
		}
		if len(bad) == 0 {
			return nil
		}
		return fmt.Errorf("its USES line names %s, which %s not printed in the material "+
			"you were given, so cite on that line only tags that are printed there and "+
			"name anything else in words", strings.Join(bad, ", "), were(len(bad)))
	}
}

func were(n int) string {
	if n == 1 {
		return "was"
	}
	return "were"
}
