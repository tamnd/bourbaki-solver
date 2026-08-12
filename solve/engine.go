package solve

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// The engine of spec 07 §2: one exercise in, one solution file out, over seven
// calls at most and as few as one.
//
// It does not know what a fleet is. Everything that touches ssh, a rented box or
// a browser profile is behind Asker, which is one method, so the pipeline can be
// run end to end in a test against answers written by hand. That is not a
// convenience. The judging is the part of this that has to be right, and a
// judging harness that can only be exercised by spending a fleet is a harness
// nobody exercises.

// Answer is what one call came back with.
type Answer struct {
	Text string
	// Model is what actually answered, which is not what was asked for. An
	// account moved down between two calls answers the rest of the pipeline on
	// a cut down model, and a solution that names only the first is a solution
	// that hides it.
	Model        string
	Conversation string
	Elapsed      time.Duration
}

// Asker is one question put to a model in a fresh conversation.
//
// Fresh is the whole design. Six of the seven calls must not see what the others
// saw: a candidate that has read the reference is a paraphrase of it, and an
// audit judge sharing a thread with the truth judge is the truth judge agreeing
// with itself.
type Asker interface {
	Ask(ctx context.Context, id, question string) (Answer, error)
}

// Engine runs the pipeline.
type Engine struct {
	Ask Asker

	// Candidates is how many attempts are written before one is chosen, 3 by
	// default, one to an angle. More than the angles wraps round and asks the
	// same angle twice.
	Candidates int
	// Corrections is how many times a failed solution goes back with the
	// complaints, 2 by default. It is bounded because the third turn of a
	// correction loop is nearly always the model rewording the same gap: the
	// budget is not a limit on how good the answer can get, it is a limit on how
	// long a wrong answer can cost.
	Corrections int
	// Attempts is how many times one call is asked before it is given up on, 2
	// by default. The second ask carries a note saying what was missing from the
	// first, and it is for an answer that did not carry its decision lines
	// rather than for one that decided something unwelcome.
	Attempts int
	// Retries is how many times a call that did not come back at all is tried
	// again, 3 by default, with a pause between.
	//
	// This is not Attempts, and the two are counted apart because they are two
	// different things gone wrong. An answer that did not carry its decision has
	// been thought about, and asking again is asking the same model the same
	// question. A host that dropped the connection, or a service that answered
	// with its own error page, has not answered at all, and the exercise behind
	// it may have five calls already spent on it.
	Retries int
	// Pause is how the retry waits. A service that has just said something went
	// wrong is likelier than not to say it again a second later.
	Pause func(ctx context.Context, d time.Duration) error

	// Archive is given every question and every answer, before anything is
	// judged. A solution goes into a book and the run that produced it is the
	// only evidence of how, so this is called even for the calls that are thrown
	// away.
	Archive func(id, question, answer string) error
	Logf    func(string, ...any)
	Now     func() time.Time
}

// Result is one exercise solved, or one exercise honestly not solved.
type Result struct {
	Solution Solution
	// Reference is the whole of the reference call, which is not written to the
	// corpus. It is the obligations and the falsification checks, and it is what
	// a person reads when they want to know why a judge said no.
	Reference string
	// Selected is the candidate the selector chose, 1 based, and 0 where there
	// was nothing to choose between.
	Selected int
	Calls    []Call
}

// Call is one question asked, for the run log.
type Call struct {
	Stage        string
	Attempt      int
	Model        string
	Conversation string
	Question     int // characters
	Answer       int // characters
	Elapsed      time.Duration
}

func (e Engine) candidates() int {
	if e.Candidates > 0 {
		return e.Candidates
	}
	return 3
}

func (e Engine) corrections() int {
	if e.Corrections > 0 {
		return e.Corrections
	}
	return 2
}

func (e Engine) attempts() int {
	if e.Attempts > 0 {
		return e.Attempts
	}
	return 2
}

func (e Engine) retries() int {
	if e.Retries > 0 {
		return e.Retries
	}
	return 3
}

// pause waits between retries. The wait grows, because a service that is having
// a bad minute is having a bad minute and not a bad second.
func (e Engine) pause(ctx context.Context, n int) error {
	d := time.Duration(n) * 30 * time.Second
	if e.Pause != nil {
		return e.Pause(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (e Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e Engine) logf(format string, args ...any) {
	if e.Logf != nil {
		e.Logf(format, args...)
	}
}

// Solve runs the pipeline over one assembled context.
//
// An error here is the pipeline failing, not the exercise: a host that will not
// answer, a call that came back with no decision twice running. An exercise that
// cannot be done comes back as a Result with a status saying so, because that is
// a fact about the corpus and belongs in it.
func (e Engine) Solve(ctx context.Context, c *Context) (Result, error) {
	parts := c.PartsOf()
	run := &state{engine: e, ctx: c, parts: parts}

	reference, err := run.ask(ctx, "reference", prompt.SolveReference(c.Render()), wantReference)
	if err != nil {
		return run.result(), err
	}
	run.reference = reference

	// The two ways an exercise is not the model's to fail. Both are read off the
	// reference call, which is the one call that has read the exercise and has
	// not read an answer, and both stop the pipeline there: three candidates on
	// an exercise that turns on a volume nobody holds is three calls spent to
	// find out what one call already said.
	if reach, ok := textguard.Reach(reference); ok && reach == "OUT_OF_CORPUS" {
		return run.verdict(corpus.StatusBlocked, blockedNote, reference), nil
	}
	if nature, ok := textguard.Nature(reference); ok && nature == "EXPLORATION" {
		return run.verdict(corpus.StatusOpen, openNote, reference), nil
	}

	written, err := run.write(ctx)
	if err != nil {
		return run.result(), err
	}
	if len(written) == 0 {
		return run.result(), fmt.Errorf("%s: no candidate came back", c.Label)
	}

	chosen, err := run.choose(ctx, written)
	if err != nil {
		return run.result(), err
	}

	return run.judge(ctx, chosen)
}

// Review is a solution judged and not rewritten.
type Review struct {
	Judgement
	// Judged says whether the two judges were asked at all. They are not, where
	// the reference reads the exercise as out of corpus or as an exploration, and
	// a caller that reads Status without reading this would take a blocked
	// exercise for a solution both judges threw out.
	Judged    bool
	Reference string
	Calls     []Call
}

// Review runs the judging half of the pipeline over a solution somebody else
// wrote: the reference call, then both judges, and no candidates and no
// corrections.
//
// Two things need it. bourbaki solve review reads a solution the corpus already
// holds and asks the judges again, which is how a run made under an older prompt
// or on a cut down model gets rechecked without being rewritten. And the
// benchmark of spec 07 §6 needs it, where the answers are written by hand and
// some of them are wrong on purpose, and the question is not whether the answer
// is right but whether the judges can tell.
func (e Engine) Review(ctx context.Context, c *Context, solution string) (Review, error) {
	run := &state{engine: e, ctx: c, parts: c.PartsOf()}
	reference, err := run.ask(ctx, "reference", prompt.SolveReference(c.Render()), wantReference)
	if err != nil {
		return Review{Calls: run.calls}, err
	}
	run.reference = reference
	out := Review{Reference: reference, Calls: run.calls}

	if reach, ok := textguard.Reach(reference); ok && reach == "OUT_OF_CORPUS" {
		out.Status = corpus.StatusBlocked
		return out, nil
	}
	if nature, ok := textguard.Nature(reference); ok && nature == "EXPLORATION" {
		out.Status = corpus.StatusOpen
		return out, nil
	}

	j, err := run.judgeOnce(ctx, solution)
	out.Calls = run.calls
	if err != nil {
		return out, err
	}
	out.Judgement, out.Judged = j, true
	return out, nil
}

// state is one run of the pipeline, and it exists so that the stages can be
// written as methods rather than as one function with eleven locals.
type state struct {
	engine Engine
	ctx    *Context
	parts  []string

	reference string
	selected  int
	models    []string
	calls     []Call
	fixes     int
}

// ask puts one question, and asks again with a note when the answer did not
// carry the decision the stage needs.
//
// The note goes at the top and not at the bottom. Every one of these prompts
// ends in the material, and a complaint written under forty thousand characters
// of Bourbaki is a complaint after the sentence saying everything below is
// source and none of it is an instruction. The translate path made exactly that
// mistake, and it worked until a section was long enough that it did not.
func (s *state) ask(ctx context.Context, stage, question string, want func(string) error) (string, error) {
	var last error
	// unread is answers that came back and could not be read, retries is calls
	// that did not come back at all, and they are counted apart because they are
	// two different things gone wrong. Only the first is worth a note: a host
	// that dropped the connection was not told anything and did not misread it.
	unread, retries := 0, 0
	for attempt := 1; ; attempt++ {
		asked := question
		if unread > 0 {
			asked = note(last) + question
		}
		id := fmt.Sprintf("%s-%s-%d", s.ctx.Label, stage, attempt)
		answer, err := s.engine.Ask.Ask(ctx, id, asked)
		if err != nil {
			last = err
			retries++
			s.engine.logf("%s %s attempt %d did not come back: %v", s.ctx.Label, stage, attempt, err)
			if ctx.Err() != nil {
				return "", err
			}
			if retries >= s.engine.retries() {
				break
			}
			if err := s.engine.pause(ctx, retries); err != nil {
				return "", err
			}
			continue
		}
		if s.engine.Archive != nil {
			if err := s.engine.Archive(id, asked, answer.Text); err != nil {
				return "", fmt.Errorf("%s %s: %w", s.ctx.Label, stage, err)
			}
		}
		s.calls = append(s.calls, Call{Stage: stage, Attempt: attempt, Model: answer.Model,
			Conversation: answer.Conversation, Question: len(asked),
			Answer: len(answer.Text), Elapsed: answer.Elapsed})
		if answer.Model != "" && !slices.Contains(s.models, answer.Model) {
			s.models = append(s.models, answer.Model)
		}
		text := textguard.Strip(answer.Text)
		if err := want(text); err != nil {
			last = err
			unread++
			s.engine.logf("%s %s attempt %d: %v", s.ctx.Label, stage, attempt, err)
			if unread >= s.engine.attempts() {
				break
			}
			continue
		}
		return text, nil
	}
	return "", fmt.Errorf("%s %s: %w", s.ctx.Label, stage, last)
}

// note is what the second ask says about the first.
func note(err error) string {
	return "Your previous answer to this could not be read: " + err.Error() +
		"\n\nAnswer the whole of it again, and this time write every line the " +
		"instructions ask for, each on a line of its own, in the exact form " +
		"they are given in. Everything below is the same question as before.\n\n"
}

// write asks for the candidates, one to an angle.
//
// A candidate that came back empty or came back as the model talking about
// itself is dropped here rather than sent to the selector. The selector is asked
// which of these is most nearly a solution, and an apology is not a wrong answer
// to that question, it is not an answer.
func (s *state) write(ctx context.Context) ([]string, error) {
	angles := prompt.Angles()
	var out []string
	for i := 0; i < s.engine.candidates(); i++ {
		angle := angles[i%len(angles)]
		stage := "candidate-" + angle.Name
		text, err := s.ask(ctx, stage,
			prompt.SolveCandidateFor(s.ctx.Render(), angle, s.parts), wantSolution)
		if err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			s.engine.logf("%s: the %s candidate came to nothing: %v", s.ctx.Label, angle.Name, err)
			continue
		}
		out = append(out, text)
	}
	return out, nil
}

// choose runs the selector, or does not.
//
// One candidate is not a choice. Asking a model to select the best of one costs
// a call to be told 1, and on a run of three hundred exercises where two
// candidates in ten come back unusable that is real money for a foregone
// conclusion.
func (s *state) choose(ctx context.Context, written []string) (string, error) {
	if len(written) == 1 {
		s.selected = 1
		return written[0], nil
	}
	answer, err := s.ask(ctx, "select",
		prompt.SolveSelect(s.ctx.Exercise(), obligations(s.reference), written), wantSelection)
	if err != nil {
		return "", err
	}
	n := textguard.Selected(answer)
	if n < 1 || n > len(written) {
		// It named a candidate that is not there. The first is taken rather than
		// the run thrown away: the candidates are in angle order and the first
		// angle is the direct one, which is the order a person with no
		// information would try them in.
		s.engine.logf("%s: the selector chose candidate %d of %d", s.ctx.Label, n, len(written))
		n = 1
	}
	s.selected = n
	return written[n-1], nil
}

// Judgement is one solution read by both judges, once.
type Judgement struct {
	Status string
	Parts  []corpus.Part
	// Truth and Audit are what the two judges wrote, whole and unsummarised.
	// They are what goes to the correction call and what a person reads when a
	// verdict is not obvious.
	Truth, Audit             string
	TruthPassed, AuditPassed bool
	// WhyTruth and WhyAudit are the one line each judge's verdict comes down to,
	// which is what a log can carry and the whole judgement cannot.
	WhyTruth, WhyAudit string
}

// judgeOnce runs both judges over one solution and reads the two verdicts.
//
// Both have to pass. They are not two opinions averaged: the truth judge reads
// the solution beside a worked reference and asks whether it is right, and the
// audit judge reads it with the reference taken away and tries to break it.
// Passing one and failing the other is a solution that is either persuasive
// about a step it did not take or correct in a way that cannot be seen from
// what is written, and neither of those goes into a book as believed.
func (s *state) judgeOnce(ctx context.Context, solution string) (Judgement, error) {
	truth, err := s.ask(ctx, s.stage("truth"),
		prompt.SolveTruth(s.ctx.Render(), s.reference, solution, s.parts), wantTruth)
	if err != nil {
		return Judgement{}, err
	}
	audit, err := s.ask(ctx, s.stage("audit"),
		prompt.SolveAudit(s.ctx.Render(), solution, s.parts), wantAudit)
	if err != nil {
		return Judgement{}, err
	}
	td, ad := textguard.Read(truth), textguard.Read(audit)
	okTruth, whyTruth := td.Passed()
	okAudit, whyAudit := ad.Audited()
	status, parts := outcome(td, ad, s.parts, okTruth, okAudit)
	return Judgement{Status: status, Parts: parts, Truth: truth, Audit: audit,
		TruthPassed: okTruth, AuditPassed: okAudit,
		WhyTruth: whyTruth, WhyAudit: whyAudit}, nil
}

// judge runs the judges and the correction loop.
func (s *state) judge(ctx context.Context, solution string) (Result, error) {
	for {
		j, err := s.judgeOnce(ctx, solution)
		if err != nil {
			return s.result(), err
		}
		if j.Status == corpus.StatusVerified || s.fixes >= s.engine.corrections() {
			if j.Status != corpus.StatusVerified {
				s.engine.logf("%s: %s after %d corrections, truth %v, audit %v",
					s.ctx.Label, j.Status, s.fixes, j.WhyTruth, j.WhyAudit)
			}
			return s.finish(j, solution), nil
		}

		s.fixes++
		fixed, err := s.ask(ctx, fmt.Sprintf("correct-%d", s.fixes),
			prompt.SolveCorrect(s.ctx.Render(), solution,
				complaints(j.Truth, j.Audit, j.TruthPassed, j.AuditPassed), s.parts), wantSolution)
		if err != nil {
			// The correction call failed and the solution as it stands has been
			// judged. Marking it on what the judges said beats losing the whole
			// exercise over a host that would not answer the eighth call.
			s.engine.logf("%s: the correction call failed, keeping what was judged: %v", s.ctx.Label, err)
			return s.finish(j, solution), nil
		}
		solution = fixed
	}
}

// stage names a judging call, so that the second time round the loop is not
// filed over the first.
func (s *state) stage(name string) string {
	if s.fixes == 0 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, s.fixes)
}

// finish turns a judged solution into a file.
func (s *state) finish(j Judgement, solution string) Result {
	uses, body := textguard.Uses(solution)
	m := s.meta(j.Status)
	m.Parts = j.Parts
	m.Uses = uses
	m.TruthJudge, m.AuditJudge = verdict(j.TruthPassed), verdict(j.AuditPassed)
	m.Candidates = s.engine.candidates()
	m.Corrections = s.fixes
	out := s.result()
	out.Solution = Solution{Meta: m, Body: strings.TrimSpace(body) + "\n"}
	return out
}

// verdict is what goes in the front matter for a judge.
func verdict(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

// blockedNote and openNote are the body of a solution that was never attempted
// for a reason. They are a sentence and then the reference call, because the
// reference is the evidence that the verdict was reached rather than assumed,
// and a file saying an exercise cannot be done with nothing under it is a file
// nobody can disagree with.
const (
	blockedNote = "This exercise was not attempted. The reference reading found " +
		"that it turns on a numbered result of a volume this corpus does not " +
		"hold, and cannot be discharged from what is here. What the reading found:"
	openNote = "This exercise was not attempted as a proof. The reference reading " +
		"found that it asks the reader to investigate rather than to establish a " +
		"statement, so there is nothing here for a judge to pass. What the " +
		"reading found:"
)

// verdict writes the file for an exercise the pipeline stopped at.
func (s *state) verdict(status, why, reference string) Result {
	out := s.result()
	out.Solution = Solution{Meta: s.meta(status),
		Body: why + "\n\n" + strings.TrimSpace(reference) + "\n"}
	return out
}

func (s *state) meta(status string) corpus.SolutionFrontMatter {
	return corpus.SolutionFrontMatter{
		Label: s.ctx.Label, Tag: s.ctx.Tag, Lang: s.ctx.Lang, Status: status,
		Model:        strings.Join(s.models, ", "),
		Generated:    s.engine.now().UTC().Format(time.RFC3339),
		PromptSHA256: prompt.SolveSHA256(),
	}
}

func (s *state) result() Result {
	return Result{Reference: s.reference, Selected: s.selected, Calls: s.calls}
}

// outcome is the status and the per-part verdicts the two judges add up to.
//
// A part is verified when both judges passed it. A judge that wrote no line for
// a part did not pass it: the prompts ask for a line on every part including the
// ones that passed, and a missing line is a part nobody looked at rather than a
// part nobody objected to.
func outcome(truth, audit textguard.Decision, parts []string, okTruth, okAudit bool) (string, []corpus.Part) {
	if len(parts) == 0 {
		if okTruth && okAudit {
			return corpus.StatusVerified, nil
		}
		return corpus.StatusUnverified, nil
	}
	byID := func(d textguard.Decision) map[string]textguard.PartDecision {
		m := map[string]textguard.PartDecision{}
		for _, p := range d.Parts {
			m[p.ID] = p
		}
		return m
	}
	t, a := byID(truth), byID(audit)
	var out []corpus.Part
	good := 0
	for _, id := range parts {
		tp, hasT := t[id]
		ap, hasA := a[id]
		switch {
		case hasT && tp.Pass && hasA && ap.Pass:
			out = append(out, corpus.Part{ID: id, Status: corpus.StatusVerified})
			good++
		case hasT && !tp.Pass:
			out = append(out, corpus.Part{ID: id, Status: corpus.StatusUnverified,
				Reason: reason(tp.Reason, "the truth judge failed this part and gave no reason")})
		case hasA && !ap.Pass:
			out = append(out, corpus.Part{ID: id, Status: corpus.StatusUnverified,
				Reason: reason(ap.Reason, "the audit judge failed this part and gave no reason")})
		default:
			out = append(out, corpus.Part{ID: id, Status: corpus.StatusUnverified,
				Reason: "no judge wrote a line for this part"})
		}
	}
	switch {
	// The whole has to pass as well as every part. A judge can pass each part
	// and still fail the solution, and where it does it has usually found
	// something about how the parts fit together, which is the thing an exercise
	// in this many parts is often about.
	case good == len(parts) && okTruth && okAudit:
		return corpus.StatusVerified, out
	case good == 0:
		return corpus.StatusUnverified, out
	case good == len(parts):
		// Every part passed and the solution did not. Nothing is partial about
		// the parts, so the parts are dropped and the file says what happened:
		// carrying them would make the audit read it as a partial with no
		// failing part, which is the shape it refuses.
		return corpus.StatusUnverified, nil
	}
	return corpus.StatusPartial, out
}

func reason(given, fallback string) string {
	if strings.TrimSpace(given) == "" {
		return fallback
	}
	return given
}

// complaints is what goes to the correction call.
//
// Both judgements go over whole and unsummarised, and only the ones that failed.
// A summary of a complaint is a second reading of the solution by something that
// has not read the solution, and sending the judgement that passed would invite
// the model to fix what nobody objected to.
func complaints(truth, audit string, okTruth, okAudit bool) string {
	var b strings.Builder
	if !okTruth {
		b.WriteString("The first judge read your solution beside a worked " +
			"reference. What it said:\n\n" + strings.TrimSpace(truth) + "\n\n")
	}
	if !okAudit {
		b.WriteString("The second judge read your solution on its own and tried " +
			"to break it. What it said:\n\n" + strings.TrimSpace(audit) + "\n\n")
	}
	return strings.TrimSpace(b.String())
}

// obligations is the obligations section of the reference and nothing else.
//
// The selector is given this and not the reference's own derivation, spec 07
// §2.3. Handed the derivation it picks the candidate that reads most like it,
// which is a vote for one route rather than a judgement about which answer is
// right.
func obligations(reference string) string {
	const heading = "## Obligations"
	i := strings.Index(reference, heading)
	if i < 0 {
		return strings.TrimSpace(reference)
	}
	rest := reference[i+len(heading):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// The four things a stage will not accept as an answer. Each is the reason a
// call is asked a second time, and each is about the answer being unreadable
// rather than about it being unwelcome. A judge that says FAIL has answered.
func wantReference(text string) error {
	if _, ok := textguard.Nature(text); !ok {
		return fmt.Errorf("there is no NATURE line in it")
	}
	if _, ok := textguard.Reach(text); !ok {
		return fmt.Errorf("there is no REACH line in it")
	}
	if !strings.Contains(text, "## Obligations") {
		return fmt.Errorf("there is no Obligations section in it")
	}
	return nil
}

func wantSolution(text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("it is empty")
	}
	if leaks := textguard.Check(text); len(leaks) > 0 {
		return fmt.Errorf("it is the model talking rather than the mathematics: %s",
			leaks[0].Detail)
	}
	return nil
}

func wantSelection(text string) error {
	if textguard.Selected(text) == 0 {
		return fmt.Errorf("there is no SELECTED line in it")
	}
	return nil
}

func wantTruth(text string) error {
	d := textguard.Read(text)
	switch {
	case d.Verdict == "UNKNOWN":
		return fmt.Errorf("there is no VERDICT line in it")
	case !d.HasTruth:
		return fmt.Errorf("there is no TRUTH line in it")
	case !d.HasQuality:
		return fmt.Errorf("it does not answer all four of COMPLETE, SELF_CONTAINED, " +
			"HUMAN_READABLE and VERIFIABLE")
	case d.Score < 0:
		return fmt.Errorf("there is no SCORE line in it")
	}
	return nil
}

func wantAudit(text string) error {
	d := textguard.Read(text)
	switch {
	case d.Verdict == "UNKNOWN":
		return fmt.Errorf("there is no VERDICT line in it")
	case !d.HasTruth:
		return fmt.Errorf("there is no TRUTH line in it")
	}
	return nil
}
