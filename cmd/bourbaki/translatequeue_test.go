package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/queue"
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

	have, queued, stuck, err := plan(q, root, "vi", "prompt-v1", j, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(have) != 0 || queued != 3 || len(stuck) != 0 {
		t.Fatalf("first pass: have %d, queued %d, stuck %d", len(have), queued, len(stuck))
	}

	// The first chunk comes back and is written down, the way a worker writes it.
	first := j.chunks[0]
	answer := accepted{Source: j.source, Chunk: first.Index, Of: first.Of,
		Input: chunkInput(first.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6", Text: "Đoạn thứ nhất."}
	if err := writeAccepted(root, "vi", answer); err != nil {
		t.Fatal(err)
	}

	have, queued, stuck, err = plan(q, root, "vi", "prompt-v1", j, false)
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

// The terminology is part of the question, so a glossary row that reaches this
// section makes the answer on disk an answer to something else.
func TestAChangeOfTerminologyIsANewChunk(t *testing.T) {
	q, root := openQueue(t)
	j := section()
	first := j.chunks[0]
	if err := writeAccepted(root, "vi", accepted{Source: j.source, Chunk: first.Index, Of: first.Of,
		Input: chunkInput(first.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6", Text: "Đoạn thứ nhất."}); err != nil {
		t.Fatal(err)
	}
	moved := j
	moved.terms = "terms-v2"

	have, queued, _, err := plan(q, root, "vi", "prompt-v1", moved, false)
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
		Input: chunkInput(first.Body, j.terms), Prompt: "prompt-v1", Model: "gpt-5-6", Text: "Đoạn thứ nhất."}); err != nil {
		t.Fatal(err)
	}
	have, queued, _, err := plan(q, root, "vi", "prompt-v2", j, false)
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
		Input: chunkInput(first.Body, j.terms), Model: "gpt-5-6", Text: "Đoạn thứ nhất."}); err != nil {
		t.Fatal(err)
	}
	have, queued, _, err := plan(q, root, "vi", "prompt-v1", j, false)
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
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", j, false); err != nil {
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

	have, queued, stuck, err := plan(q, root, "vi", "prompt-v1", j, false)
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
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", j, false); err != nil {
		t.Fatal(err)
	}
	item, err := q.Lease(queue.StageTranslate, "server3", translateGroup("vi", j.source), chunkLease)
	if err != nil {
		t.Fatal(err)
	}
	if state, err := q.Fail(item, "the formula came back reflowed"); err != nil || state != queue.Dead {
		t.Fatalf("Fail = %s %v", state, err)
	}

	have, queued, stuck, err := plan(q, root, "vi", "prompt-v1", j, false)
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
	if _, queued, stuck, err = plan(q, root, "vi", "prompt-v1", j, true); err != nil {
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
	have, queued, _, err := plan(q, root, "vi", "prompt-v1", j, true)
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
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", first, false); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := plan(q, root, "vi", "prompt-v1", second, false); err != nil {
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
		g, "vi", j, j.chunks[0], false, func(format string, args ...any) { asks++ })
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
