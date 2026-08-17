package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/route"
	"github.com/tamnd/bourbaki-solver/translate"
)

// A section of three chunks, which is enough for every case here: one answered,
// one to ask for, one stuck.
//
// Each paragraph carries a full chunk's worth of mathematics, since that is the
// budget that cuts first, and a section of three short prose paragraphs is one
// chunk and tests nothing.
func section() job {
	var body string
	for range 3 {
		body += "Let"
		for i := range translate.ChunkSpans {
			body += fmt.Sprintf(" $x_{%d}$", i)
		}
		body += " be elements of E.\n\n"
	}
	return job{source: "content/en/alg/VIII/03_s3_simple_modules.md", body: body,
		chunks: translate.Chunks(body), terms: "terms-v1"}
}

// viOf is a chunk of the fixture as a translation of itself: the mathematics
// where it stood and the words around it in Vietnamese. An answer has to be one
// the rules accept, since plan reads them over what is on disk before it takes
// an answer as answered.
func viOf(body string) string {
	body = strings.ReplaceAll(body, "Let", "Cho")
	return strings.ReplaceAll(body, " be elements of E.", " là các phần tử của E.")
}

func openQueue(t *testing.T) (*queue.Queue, string) {
	t.Helper()
	root := t.TempDir()
	q, err := queue.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return q, root
}

// The point of the whole thing: a run that is killed does not ask again for what
// it already has.
func TestAnAnsweredChunkIsNotAskedFor(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	if len(j.chunks) != 3 {
		t.Fatalf("the fixture came to %d chunks", len(j.chunks))
	}

	have, queued, stuck, err := plan(q, root, "vi", "prompt-v1", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || queued != 3 || len(stuck) != 0 {
		t.Fatalf("first pass: have %d, queued %d, stuck %d", len(have), queued, len(stuck))
	}

	// The first chunk comes back and is written down, the way a worker writes it.
	first := j.chunks[0]
	answer := accepted{Source: j.source, Chunk: first.Index, Of: first.Of,
		Input: chunkInput(first.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6", Text: viOf(first.Body)}
	if err := writeAccepted(root, "vi", answer); err != nil {
		t.Fatal(err)
	}

	have, queued, stuck, err = plan(q, root, "vi", "prompt-v1", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 1 || len(stuck) != 0 {
		t.Fatalf("second pass: have %d, stuck %d", len(have), len(stuck))
	}
	if got := have[first.Index].Text; got != answer.Text {
		t.Errorf("the answer came back as %q", got)
	}
	if got := have[first.Index].Model; got != "gpt-5-6" {
		t.Errorf("the model came back as %q, and the front matter is made of it", got)
	}
	if queued != 2 {
		t.Errorf("queued = %d, want the other two", queued)
	}
}

// Section 7 of chapter III is where this comes from. Two of its chunks were
// accepted weeks ago with a word inside a formula copied through, which is what
// the rules asked for then, the word counts as prose now, and the file put
// together out of every accepted chunk was refused over two spans that no chunk
// was ever going to be asked about again. A cached answer the rule standing
// today will not take is not an answer.
func TestAnAnswerTodaysRulesRefuseIsAskedForAgain(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	first := j.chunks[0]
	if err := writeAccepted(root, "vi", accepted{Source: j.source, Chunk: first.Index, Of: first.Of,
		Input: chunkInput(first.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6",
		Text: strings.ReplaceAll(viOf(first.Body), "$x_{0}$", "$y_{0}$")}); err != nil {
		t.Fatal(err)
	}

	have, queued, _, err := plan(q, root, "vi", "prompt-v1", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || queued != 3 {
		t.Fatalf("have %d and queued %d, want the renamed variable refused and all three asked for", len(have), queued)
	}
}

// The terminology is part of the question, so a glossary row that reaches this
// section makes the answer on disk an answer to something else.
func TestAChangeOfTerminologyIsANewChunk(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	first := j.chunks[0]
	if err := writeAccepted(root, "vi", accepted{Source: j.source, Chunk: first.Index, Of: first.Of,
		Input: chunkInput(first.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6", Text: viOf(first.Body)}); err != nil {
		t.Fatal(err)
	}
	moved := j
	moved.terms = "terms-v2"

	have, queued, _, err := plan(q, root, "vi", "prompt-v1", moved, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || queued != 3 {
		t.Fatalf("have %d and queued %d, want the answer refused and all three asked for", len(have), queued)
	}
}

// The instructions are part of the question too. The rule about words set
// inside a formula is a change to the prompt and nothing else, and an answer
// written before it was there is an answer to the old instructions.
func TestAChangeOfInstructionsIsANewChunk(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	first := j.chunks[0]
	if err := writeAccepted(root, "vi", accepted{Source: j.source, Chunk: first.Index, Of: first.Of,
		Input: chunkInput(first.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6", Text: viOf(first.Body)}); err != nil {
		t.Fatal(err)
	}
	have, queued, _, err := plan(q, root, "vi", "prompt-v2", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || queued != 3 {
		t.Fatalf("have %d and queued %d, want the answer refused and all three asked for", len(have), queued)
	}
}

// An answer written before the prompt was recorded cannot say what it was asked
// for, so it is asked again rather than trusted.
func TestAnAnswerWithNoInstructionsRecordedIsAskedAgain(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	first := j.chunks[0]
	if err := writeAccepted(root, "vi", accepted{Source: j.source, Chunk: first.Index, Of: first.Of,
		Input: chunkInput(first.Body, j.terms), Model: "gpt-5-6", Text: viOf(first.Body)}); err != nil {
		t.Fatal(err)
	}
	have, queued, _, err := plan(q, root, "vi", "prompt-v1", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || queued != 3 {
		t.Fatalf("have %d and queued %d, want the answer refused and all three asked for", len(have), queued)
	}
}

// work/ is scratch and gets cleaned. A queue that says done when the answer is
// gone is a section that can never be joined, so the record follows the answer.
func TestADoneChunkWithNoAnswerIsAskedForAgain(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", j, false, false); err != nil {
		t.Fatal(err)
	}
	// Whichever one comes out. Jobs are handed out in id order, which is hash
	// order, so the chunks of a section do not go over in reading order and
	// nothing here should assume they do: an answer is placed by the number in
	// its target and not by the order the lanes came back in.
	item, err := q.Lease(queue.StageTranslate, "server3", translateGroup("vi", j.source), chunkLease)
	if err != nil {
		t.Fatal(err)
	}
	index, chunk, ok := chunkOf(j, item)
	if !ok {
		t.Fatalf("chunkOf did not recognise %s", item.Target)
	}
	if want := queue.NewID(queue.StageTranslate,
		translateTarget("vi", j.source, index), chunkInput(chunk.Body, j.terms), "prompt-v1"); item.ID != want {
		t.Fatalf("leased %s, want %s: the id is not the content address of the work", item.ID, want)
	}
	if _, err := q.Finish(item, true, "gpt-5-6"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(acceptedPath(root, "vi", j.source, index)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	have, queued, stuck, err := plan(q, root, "vi", "prompt-v1", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || len(stuck) != 0 || queued != 3 {
		t.Fatalf("have %d, queued %d, stuck %d, want the done chunk asked for again", len(have), queued, len(stuck))
	}
	if _, err := q.Lease(queue.StageTranslate, "server3", translateGroup("vi", j.source), chunkLease); err != nil {
		t.Fatalf("the chunk did not come back: %v", err)
	}
}

// Three attempts that fail the same way are what dead means. The section is
// refused with the reason rather than asked for a fourth time.
func TestADeadChunkStopsTheSectionAndSaysSo(t *testing.T) {
	q, root := openQueue(t)
	q.MaxAttempts = 1
	j := section()
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", j, false, false); err != nil {
		t.Fatal(err)
	}
	item, err := q.Lease(queue.StageTranslate, "server3", translateGroup("vi", j.source), chunkLease)
	if err != nil {
		t.Fatal(err)
	}
	if state, err := q.Fail(item, "the formula came back reflowed"); err != nil || state != queue.Dead {
		t.Fatalf("Fail = %s %v", state, err)
	}

	have, queued, stuck, err := plan(q, root, "vi", "prompt-v1", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	index, _, ok := chunkOf(j, item)
	if !ok {
		t.Fatalf("chunkOf did not recognise %s", item.Target)
	}
	if len(stuck[index]) == 0 {
		t.Fatalf("the dead chunk was not reported: stuck = %v", stuck)
	}
	if len(have) != 0 || queued != 2 {
		t.Errorf("have %d and queued %d, want the dead one left alone and the other two asked for", len(have), queued)
	}

	// -force is the way past it, since a person typing it has read the reason.
	if _, queued, stuck, err = plan(q, root, "vi", "prompt-v1", j, true, false); err != nil {
		t.Fatal(err)
	}
	if len(stuck) != 0 || queued != 3 {
		t.Errorf("with -force: queued %d, stuck %d", queued, len(stuck))
	}
}

// -force is for a section that is already there and was written on a cut down
// model, so it has to reach the answers and not only the file.
func TestForceAsksAgainForAnAnswerThatIsOnDisk(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	for _, c := range j.chunks {
		if err := writeAccepted(root, "vi", accepted{Source: j.source, Chunk: c.Index, Of: c.Of,
			Input: chunkInput(c.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6-mini", Text: "một đoạn"}); err != nil {
			t.Fatal(err)
		}
	}
	have, queued, _, err := plan(q, root, "vi", "prompt-v1", j, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || queued != 3 {
		t.Fatalf("have %d and queued %d, want all three asked again", len(have), queued)
	}
}

// Two sections translated in one run must not lease each other's chunks: the
// run joins one section at a time and a chunk of the next one is a chunk nobody
// puts anywhere.
func TestASectionOnlyLeasesItsOwnChunks(t *testing.T) {
	q, root := openQueue(t)
	first := section()
	second := section()
	second.source = "content/en/alg/VIII/04_s4_semisimple_modules.md"
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", first, false, false); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", second, false, false); err != nil {
		t.Fatal(err)
	}
	group := translateGroup("vi", first.source)
	for range 3 {
		item, err := q.Lease(queue.StageTranslate, "server3", group, chunkLease)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, ok := chunkOf(first, item); !ok {
			t.Fatalf("%s is not a chunk of %s", item.Target, first.source)
		}
	}
	if _, err := q.Lease(queue.StageTranslate, "server3", group, chunkLease); err == nil {
		t.Error("a fourth chunk was leased into a section of three")
	}
}

// Ctrl-C during a chunk is not the model getting it wrong. The second ask is
// down a context that is already cancelled, so it cannot answer and only costs
// the two minutes an ssh timeout takes, after the person has already stopped it.
func TestAnInterruptedChunkIsNotAskedTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	j := section()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "element", VI: "phần tử"}}}
	asks := 0
	_, _, bad := askChunk(ctx, t.TempDir(), ocr.Host{Name: "nowhere.invalid", Tool: "/usr/bin/false"},
		g, "vi", j, j.chunks[0], false, chunkDeadline, func(format string, args ...any) { asks++ })
	if asks != 1 {
		t.Errorf("the chunk was asked %d times, want the one that was interrupted", asks)
	}
	if len(bad) != 1 || bad[0].Rule != "transport" {
		t.Errorf("it came back as %v, want the transport failure kept for the caller to release on", bad)
	}
}

// The same language and the same section is the same group, and a different
// language is a different one, since the two are asked for separately and one
// must not lease the other's work.
func TestGroupsSeparateTheLanguages(t *testing.T) {
	source := "content/en/alg/VIII/03_s3_simple_modules.md"
	if translateGroup("vi", source) == translateGroup("zh", source) {
		t.Error("Vietnamese and Chinese share a group")
	}
	// A second run is a second process, so the group has to come out of the name
	// alone and not out of anything the run happens to be holding.
	again := strings.Join([]string{"content", "en", "alg", "VIII", "03_s3_simple_modules.md"}, "/")
	if translateGroup("vi", source) != translateGroup("vi", again) {
		t.Error("the group is not a function of the section")
	}
}

// The other half of a change of instructions. The new job goes on the queue, and
// the old one has to come off it, or the section is pending twice over: two jobs
// with different ids and the same target, in the same group, and a worker reads
// the chunk number out of the target and cannot tell them apart.
//
// This is what the fleet was doing after the prompt was rewritten to name
// Bourbaki rather than Algebra. The log has chapter I's front matter accepted at
// 12 seconds on zen-laguna and again at 1 minute 6 on server3, and the second
// answer overwrote the first, on a cut down model.
func TestPlanningUnderNewInstructionsTakesTheOldChunksOffTheQueue(t *testing.T) {
	q, root := openQueue(t)
	j := section()

	if _, queued, _, err := plan(q, root, "vi", "prompt-v1", j, false, false); err != nil {
		t.Fatal(err)
	} else if queued != 3 {
		t.Fatalf("queued %d under the first instructions, want 3", queued)
	}
	if _, queued, _, err := plan(q, root, "vi", "prompt-v2", j, false, false); err != nil {
		t.Fatal(err)
	} else if queued != 3 {
		t.Fatalf("queued %d under the second, want 3", queued)
	}

	// Three chunks, so three leases and then the group is empty. Six would be
	// every chunk of this section asked for twice.
	group := translateGroup("vi", j.source)
	seen := map[string]int{}
	for {
		item, err := q.Lease(queue.StageTranslate, "host", group, time.Minute)
		if errors.Is(err, queue.ErrEmpty) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[item.Target]++
	}
	if len(seen) != 3 {
		t.Fatalf("leased %d distinct chunks, want 3: %v", len(seen), seen)
	}
	for target, count := range seen {
		if count != 1 {
			t.Errorf("%s was handed out %d times", target, count)
		}
	}
}

// A provider that will not take the question is not the chunk getting it wrong.
//
// Three attempts are for three bad answers. Counting a 429 as one of them means
// a gateway that has spent its allowance for the day kills every chunk of the
// section it touches, and dead is a state somebody has to come and clear by
// hand. It is not hypothetical: forty two chunks of chapter I, § 1 went from
// pending to dead in one minute fourteen seconds, and not one model behind them
// had been asked anything.
func TestAProviderThatWillNotAnswerDoesNotKillTheChunks(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "element", VI: "phần tử"}}}
	host := ocr.Host{Name: "nowhere.invalid", Tool: "/usr/bin/false", Lanes: 1}

	// Four runs, which is one more than a chunk has attempts, so a run that
	// spends an attempt on a transport failure would have killed all three by
	// the end of this loop.
	for run := 1; run <= 4; run++ {
		_, _, problems := translateFile(context.Background(), root, q, []ocr.Host{host}, g,
			"vi", "prompt-v1", j, false, false, false, chunkDeadline, func(string, ...any) {})
		if len(problems) == 0 {
			t.Fatalf("run %d: the section was written by a host that answers nothing", run)
		}
	}

	dead, err := q.List(queue.StageTranslate, queue.Dead)
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 0 {
		t.Errorf("%d chunks are dead and no model ever saw them", len(dead))
	}
	pending, err := q.List(queue.StageTranslate, queue.Pending)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != len(j.chunks) {
		t.Fatalf("%d of %d chunks are still on the list", len(pending), len(j.chunks))
	}
	for _, item := range pending {
		if item.Attempts != 0 {
			t.Errorf("%s has spent %d attempts on a provider that never answered", item.Target, item.Attempts)
		}
	}
}

// And the two are told apart by what askChunk called them, so an answer that
// came back and was wrong still costs the chunk an attempt.
func TestOnlyATransportFailureIsGivenBack(t *testing.T) {
	cases := []struct {
		name string
		bad  []translate.Problem
		want bool
	}{
		{"nothing wrong", nil, false},
		{"the gateway refused", []translate.Problem{{Rule: "transport", Msg: "429"}}, true},
		{"the answer was wrong", []translate.Problem{{Rule: "math", Msg: "math span 6"}}, false},
		{"both", []translate.Problem{{Rule: "transport", Msg: "429"}, {Rule: "math", Msg: "math span 6"}}, false},
	}
	for _, c := range cases {
		if got := transportOnly(c.bad); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// The lease has to outlast the work it is a lease on.
//
// A worker takes the lease, then asks, then asks again with the complaint if
// the first answer did not pass. If the lease runs out in the middle of that,
// the queue is entitled to give the chunk to another lane, and then two lanes
// hold the same chunk and the book is asked for twice. This is the same fault
// Supersede fixed from the other end, and it was live: the lease was three
// minutes and one ask of one chunk on a box was allowed forty five, so a box
// that stopped answering expired three chunks out from under itself in the
// first nine minutes of a run.
func TestALeaseOutlastsBothAttemptsAtAChunk(t *testing.T) {
	if chunkDeadline <= 0 {
		t.Fatalf("a chunk names no deadline, so a box takes the page default of %s", 45*time.Minute)
	}
	if chunkLease < 2*chunkDeadline {
		t.Errorf("the lease is %s and the two asks it covers are %s, so the chunk expires while a lane is still on it",
			chunkLease, 2*chunkDeadline)
	}
	// And the deadline is a chunk's number rather than a page's. Six thousand
	// characters has measured between forty and seventy seconds and the
	// gateway routes allow five minutes, so anything near the page default is
	// somebody having copied the wrong constant.
	if chunkDeadline > 10*time.Minute {
		t.Errorf("one ask of one chunk is allowed %s, which is a number for a photograph of a page", chunkDeadline)
	}
}

// And it still outlasts them when a run raises the deadline.
//
// -deadline is for a day the fleet is slow. A lease that stayed at the default
// while the asks got longer is the same fault read from the other end: the
// queue would hand the chunk on while the lane that raised the deadline is
// still legitimately waiting on it, and the book gets asked for twice.
func TestARaisedDeadlineTakesALeaseThatOutlastsIt(t *testing.T) {
	for _, d := range []time.Duration{time.Minute, chunkDeadline, maxChunkDeadline} {
		if got := chunkLeaseFor(d); got < 2*d {
			t.Errorf("a %s deadline takes a %s lease, and the two asks it covers are %s", d, got, 2*d)
		}
	}
	if chunkLeaseFor(chunkDeadline) != chunkLease {
		t.Errorf("the default deadline takes a %s lease and the constant says %s",
			chunkLeaseFor(chunkDeadline), chunkLease)
	}
	// The ceiling is what a browser route says of itself, so that -deadline
	// cannot be pushed past the point the transport under it gives up anyway.
	var browser time.Duration
	for _, r := range route.Default().Routes {
		if r.Host != "" && time.Duration(r.Timeout) > browser {
			browser = time.Duration(r.Timeout)
		}
	}
	if maxChunkDeadline > browser {
		t.Errorf("-deadline goes up to %s and the slowest route says it will be %s", maxChunkDeadline, browser)
	}
}

// -redo-small asks again for the chunks a cut down model answered, and leaves
// the rest of the section alone.
//
// Nobody chooses the model. An account gets moved down between two runs of the
// same section and half of it comes back on gpt-5-6-mini, which L08 says so
// about afterwards. The only answer to that was -force, which throws away the
// whole section: chapter I, § 1 is forty two chunks and four of them were on
// the small model, so -force is thirty eight questions nobody needs to put and,
// on the free gateway, most of a day.
func TestRedoSmallAsksAgainOnlyForWhatTheSmallModelAnswered(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	for i, model := range []string{"gpt-5-6", "gpt-5-6-mini", "nemotron-3-ultra-free"} {
		c := j.chunks[i]
		if err := writeAccepted(root, "vi", accepted{Source: j.source, Chunk: c.Index, Of: c.Of,
			Input: chunkInput(c.Body, j.terms), Prompt: "prompt-v1", Model: model,
			Text: viOf(c.Body)}); err != nil {
			t.Fatal(err)
		}
	}

	// Without the flag the section is finished, small model and all, which is
	// what makes L08 soft: the text may well be fine.
	have, queued, _, err := plan(q, root, "vi", "prompt-v1", j, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 3 || queued != 0 {
		t.Fatalf("without the flag: have %d, queued %d, want all three in hand", len(have), queued)
	}

	have, queued, stuck, err := plan(q, root, "vi", "prompt-v1", j, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if queued != 1 {
		t.Errorf("queued %d chunks, want the one gpt-5-6-mini answered", queued)
	}
	if len(stuck) != 0 {
		t.Errorf("%d chunks are stuck, and none of them was asked for yet", len(stuck))
	}
	if len(have) != 2 {
		t.Fatalf("have %d answers, want the two a full model gave", len(have))
	}
	if _, ok := have[j.chunks[1].Index]; ok {
		t.Error("the answer gpt-5-6-mini gave was read back, so nothing was asked again")
	}
	for _, i := range []int{0, 2} {
		if _, ok := have[j.chunks[i].Index]; !ok {
			t.Errorf("chunk %d was asked again, and a full model had already answered it", j.chunks[i].Index)
		}
	}
}
