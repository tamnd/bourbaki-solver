package prompt

import (
	_ "embed"
	"fmt"
	"strings"
)

// The six prompts of the solve pipeline, spec 07 §2.
//
// The shape of the pipeline is in the prompts more than it is in the code that
// runs them. A reference is worked out without seeing any answer, several
// answers are written without seeing each other, one is chosen against the
// reference's obligations, and the choice is judged twice: once beside the
// reference and once with the reference taken away. Each of those is a prompt
// here, and what makes them a pipeline rather than six asks is what each is
// shown and what each is kept from.
//
// Every prompt that ends in a decision ends in the same fixed lines, and
// textguard.Read is the only thing that reads them. The two are one contract in
// two files, and solve_test.go asserts it: a prompt that stops asking for a line
// the parser reads, or asks for a word the parser does not know, fails the test
// rather than failing three hundred solves.

//go:embed solve_reference.md
var solveReference string

//go:embed solve_candidate.md
var solveCandidate string

//go:embed solve_select.md
var solveSelect string

//go:embed solve_truth.md
var solveTruth string

//go:embed solve_audit.md
var solveAudit string

//go:embed solve_correct.md
var solveCorrect string

// Angle is one route through an exercise.
//
// The candidates are asked for different routes rather than for the same route
// several times, because a model asked the same question twice at the same
// temperature gives two drafts of one answer, and a selector handed three drafts
// of one wrong answer has nothing to select. Three routes disagree where the
// exercise is hard, and where they disagree is where the judging is worth doing.
type Angle struct {
	// Name is what the run records, so a solution can say which route produced
	// it and a report can count which route wins.
	Name string
	// Instruction is the paragraph the model reads. It says what to try first
	// and not what to conclude: an angle that told the model the answer would
	// make three candidates that agree by construction.
	Instruction string
}

// Angles are the three routes spec 07 §2.2 asks for.
func Angles() []Angle {
	return []Angle{
		{Name: "direct", Instruction: "Work forwards. Start from the definitions " +
			"and the hypotheses as they are given, construct what has to be " +
			"constructed, and argue to the conclusion in the order the steps are " +
			"used. Where a result of the section does the work, use it and say so."},
		{Name: "contrapositive", Instruction: "Work from the negation. Suppose the " +
			"conclusion fails, or take the contrapositive of the statement, and " +
			"derive something that contradicts a hypothesis or a result you have " +
			"been given. Where the statement is an equivalence, prove the direction " +
			"that is easier this way and be explicit about which direction you are " +
			"proving. If the exercise genuinely will not take this treatment, say so " +
			"in one sentence and solve it directly instead."},
		{Name: "elementary", Instruction: "Work from first principles. Use as little " +
			"machinery as you can: unfold the definitions, and prefer an argument " +
			"about elements, submodules or ideals to an appeal to a general theorem. " +
			"The point is a solution a reader of this section could have found, so " +
			"where you do reach for a heavier result, say what it saves you."},
	}
}

// SolveReference is the candidate-blind reference call.
//
// It is candidate-blind because the obligations are what everything downstream
// is measured against, and obligations written after reading an answer are
// obligations that answer happens to meet. A reference that has seen a candidate
// is a rubric fitted to it.
func SolveReference(context string) string {
	return fill(solveReference, "{{CONTEXT}}", context)
}

// SolveCandidateFor is one attempt at the exercise, from one angle.
//
// The candidate is never shown the reference. Two calls that have read the same
// derivation are not two attempts, they are one attempt and a paraphrase, and
// the selector's whole job is to compare things that were arrived at
// separately.
func SolveCandidateFor(context string, angle Angle, parts []string) string {
	return fill(solveCandidate,
		"{{ANGLE}}", strings.TrimSpace(angle.Instruction),
		"{{PARTS}}", candidateParts(parts),
		"{{CONTEXT}}", context)
}

// SolveSelect chooses between the candidates against the reference's
// obligations.
//
// It is given the obligations and not the reference's own derivation, which is
// spec 07 §2.3. Handed the derivation the selector picks the candidate that
// reads most like it, and that is a vote for one route rather than a judgement
// about which answer is right.
//
// exercise is the exercise alone rather than the whole context. The selector is
// not checking the mathematics, it is checking coverage of a list, and forty
// thousand characters of section would be paid for on a call that does not read
// it.
func SolveSelect(exercise, obligations string, candidates []string) string {
	var b strings.Builder
	for i, c := range candidates {
		fmt.Fprintf(&b, "### Candidate %d\n\n%s\n\n", i+1, strings.TrimSpace(c))
	}
	return fill(solveSelect,
		"{{OBLIGATIONS}}", strings.TrimSpace(obligations),
		"{{EXERCISE}}", strings.TrimSpace(exercise),
		"{{CANDIDATES}}", strings.TrimSpace(b.String()))
}

// SolveTruth is the judge that reads the solution beside the reference.
//
// It is the half of the pair that decides the false reject rate, and the run of
// 2026-09-04 over the twenty-case set put that at 5 of the 10 right answers,
// against the 30 per cent spec 07 section 6 tolerates. The other half is fine:
// the reference-blind audit judge agreed with the person on 9 of the 10, and
// neither judge accepted a single one of the 10 wrong answers.
//
// The reviews say plainly what went wrong, and it is not calibration. Reading
// the four the truth judge turned down, three of them worked the whole
// checklist through and cleared it. Exercise 1 of section 1 of chapter II wrote
// DISCHARGED against all three obligations, DOES NOT FALL against all four
// failure modes, PASSED against all three falsification checks and found both
// citations honest, and then wrote COMPLETE: NO, SELF_CONTAINED: NO,
// VERIFIABLE: NO, SCORE 5/7 and a verdict of FAIL, naming nothing at all.
// Exercise 2 of section 5 of chapter I discharged 5 obligations of 5, passed 4
// checks of 4, and scored it 4 out of 7. The summary block was not being read
// off the review; it was being answered fresh, from an impression, and the
// review above it was doing no work.
//
// "Your default is to fail" is what produced that. It is the right instruction
// for a step the judge has not checked and the wrong one for a step it has
// checked and found sound, and the prompt did not distinguish the two. It now
// scopes that sentence to unchecked steps and requires every NO to name what it
// comes from: which obligation is not discharged, which step is left to a
// reader, which result is used unstated, which step cannot be checked.
//
// Requiring a reason for a rejection cannot raise the false accept rate, which
// is the number that decides whether a verdict in this corpus is worth
// anything and is the one currently met. Nothing about PASS is loosened: the
// verdict still wants TRUTH true, all four fields YES, a score of 6 or 7 and
// every part passed, and an unsettled step is still a fail.
func SolveTruth(context, reference, solution string, parts []string) string {
	return fill(solveTruth,
		"{{REFERENCE}}", strings.TrimSpace(reference),
		"{{SOLUTION}}", strings.TrimSpace(solution),
		"{{PARTS}}", judgeParts(parts),
		"{{CONTEXT}}", context)
}

// SolveAudit is the judge that never sees the reference.
//
// Two judges that had both read the reference would be one judge asked twice.
// The value of this one is that it can only read what is written, so a solution
// that is persuasive about a step it did not take has to persuade a reader who
// has no idea what the intended step was.
func SolveAudit(context, solution string, parts []string) string {
	return fill(solveAudit,
		"{{SOLUTION}}", strings.TrimSpace(solution),
		"{{PARTS}}", judgeParts(parts),
		"{{CONTEXT}}", context)
}

// SolveCorrect is one turn of the bounded correction loop.
//
// complaints is what the judges wrote, both of them where both failed it. They
// go over as they were written rather than summarised, because a summary of a
// complaint is a second reading of the solution by something that has not read
// the solution.
func SolveCorrect(context, solution, complaints string, parts []string) string {
	return fill(solveCorrect,
		"{{COMPLAINTS}}", strings.TrimSpace(complaints),
		"{{SOLUTION}}", strings.TrimSpace(solution),
		"{{PARTS}}", candidateParts(parts),
		"{{CONTEXT}}", context)
}

// candidateParts tells a writer to answer every lettered part.
//
// An exercise with no parts gets nothing at all rather than a sentence saying it
// has none. A model told there are no parts starts wondering what a part would
// have been, and most of the 317 exercises of chapter VIII have none.
func candidateParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("This exercise has lettered parts: %s. Answer every one of "+
		"them, in the order the book sets them, and say which part you are "+
		"answering as you begin it, in the book's own form: **(a)**. Do not merge "+
		"two parts into one argument. Where a part follows from an earlier one, say "+
		"that it does and say from what, which is an answer; leaving it out is not.",
		list(parts))
}

// judgeParts asks for a verdict on each part separately.
//
// Separately is the point. A judge asked for one verdict on a five part exercise
// gives the verdict the worst part deserves, and the corpus then prints nothing
// where four parts were right. The partial status exists to carry exactly that,
// and it can only be filled from lines like these.
func judgeParts(parts []string) string {
	if len(parts) == 0 {
		return "This exercise has no lettered parts, so do not write a PART line."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "This exercise has lettered parts: %s. Judge each one on its "+
		"own and write one line for each, before the lines below, in this form.\n\n",
		list(parts))
	fmt.Fprintf(&b, "PART %s: PASS\n", parts[0])
	if len(parts) > 1 {
		fmt.Fprintf(&b, "PART %s: FAIL, the induction has no base case\n", parts[1])
	}
	b.WriteString("\nA part is PASS only when it is discharged in full. A FAIL " +
		"carries the reason in a few words after the comma, and the reason is " +
		"printed beside the solution, so write it for a reader rather than for a " +
		"log. A part that the solution does not attempt is FAIL with that as its " +
		"reason. Write a line for every part, including the ones that passed.")
	return b.String()
}

// list writes a, b and c.
func list(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// fill substitutes the placeholders and leaves the prompt with one trailing
// newline.
//
// A placeholder whose value is empty takes the blank line above it with it.
// candidateParts returns nothing for most exercises, and a prompt with two blank
// lines in the middle of it is a prompt that looks to a model like something was
// meant to be there.
func fill(text string, pairs ...string) string {
	out := strings.TrimSpace(text)
	for i := 0; i+1 < len(pairs); i += 2 {
		value := pairs[i+1]
		if strings.TrimSpace(value) == "" {
			out = strings.ReplaceAll(out, "\n"+pairs[i]+"\n", "")
		}
		out = strings.ReplaceAll(out, pairs[i], strings.TrimSpace(value))
	}
	return out + "\n"
}

// SolveSHA256 is the hash of the six prompts together, with the context, the
// candidates and the judgements left standing.
//
// One hash and not six. A solution is the product of all six calls, so it is
// stale when any of them changed, and six fields in the front matter would be
// six ways to record the same fact. It does cover the angles, which are in this
// file rather than in a prompt file and are as much a part of what was asked as
// anything in them.
func SolveSHA256() string {
	var b strings.Builder
	for _, p := range []string{solveReference, solveCandidate, solveSelect,
		solveTruth, solveAudit, solveCorrect} {
		b.WriteString(strings.TrimSpace(p))
		b.WriteString("\n")
	}
	for _, a := range Angles() {
		fmt.Fprintf(&b, "%s: %s\n", a.Name, a.Instruction)
	}
	return SHA256(b.String())
}
