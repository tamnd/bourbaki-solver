package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/translate"
)

// Where a section is cut is not fixed. translate.ChunkChars and ChunkSpans
// decide it, and both are numbers somebody will want to move: the fleet answers
// a chunk in forty to seventy seconds on a good day and in twelve minutes on a
// bad one, and how much goes over in one question is the only handle there is on
// that. The archive is filed under the section and the chunk number, so the day
// either number moves, chunk seven of a section is a different piece of English
// than the chunk seven whose answer is on disk.
//
// Every other place a chunk is recognised already knows this and none of them
// had to be told. The queue id is built over chunkInput, the body and the
// terminology hashed, and readAccepted compares that hash before it will hand
// an answer back, so a re-split misses the cache and asks again. This path had
// nothing to compare, and what it feeds is the note telling a model what it got
// wrong last time.
func TestAnArchivedAnswerIsOnlyReadBackForThePassageItAnswered(t *testing.T) {
	root := t.TempDir()
	const source = "content/en/alg/I/03_s3_actions.md"
	const was = "An action of $\\Omega$ on $E$ is a map from $\\Omega$ to $E^E$."
	const now = "A left action of a monoid is a map."

	if err := archiveChunk(root, "vi", source,
		translate.Chunk{Index: 7, Of: 25, Body: was}, 1,
		asked(t, was), "Tác động của $\\Omega$ trên $E$ là ánh xạ.", ""); err != nil {
		t.Fatal(err)
	}
	if got := archivedAnswers(root, "vi", source, 7, was); len(got) != 1 {
		t.Fatalf("%d answers came back for the passage that was asked, want the one", len(got))
	}
	// The same file, the same chunk number, a different passage. It is the split
	// that moved and not the book.
	if got := archivedAnswers(root, "vi", source, 7, now); len(got) != 0 {
		t.Errorf("chunk 7 of an older split was read back as an answer to this one: %v", got)
	}
}

// The whole cost of getting this wrong is paid in the retry note, so it is worth
// showing what the note would have been. The answer of the older chunk 7 is a
// perfectly good translation of the text it was written for, and audited against
// the text standing there today it is missing formulas, missing sentences and
// missing most of what it is supposed to carry. That is a page of complaints
// about a passage nobody is looking at, put in front of a model as the first
// thing it reads.
func TestTheNoteFromAnOlderSplitIsNotPutInFrontOfTheModel(t *testing.T) {
	root := t.TempDir()
	const source = "content/en/alg/I/03_s3_actions.md"
	const was = "An action of $\\Omega$ on $E$ is a map from $\\Omega$ to $E^E$."
	const now = "A left action of a monoid $M$ on $E$ is a map $M \\times E \\to E$."

	if err := archiveChunk(root, "vi", source,
		translate.Chunk{Index: 7, Of: 25, Body: was}, 1,
		asked(t, was), "Tác động của $\\Omega$ trên $E$ là ánh xạ từ $\\Omega$ tới $E^E$.", ""); err != nil {
		t.Fatal(err)
	}
	g := &glossary.Glossary{Version: 1}
	j := job{source: source}
	c := translate.Chunk{Index: 7, Of: 25, Body: now}
	if prior := refusedBefore(root, "vi", g, j, c, now); len(prior) != 0 {
		t.Errorf("the ask opens with %d complaints about text it is not being shown: %v", len(prior), prior)
	}
	// And the passage it was written for still reads back, since nothing about
	// this is a rule against old answers.
	if prior := refusedBefore(root, "vi", g, j,
		translate.Chunk{Index: 7, Of: 25, Body: was}, was); len(prior) != 0 {
		t.Errorf("an answer that passes today carried %v", prior)
	}
}

// An answer with no question beside it cannot say what it was asked. archiveChunk
// writes the question first and returns if it cannot, so this is not a case that
// arises from a run, but work/ is scratch and a person cleaning it half way is
// what it is there for. The run goes on without the note rather than guessing.
func TestAnAnswerWithNoQuestionBesideItIsNotUsed(t *testing.T) {
	root := t.TempDir()
	const source = "content/en/alg/I/03_s3_actions.md"
	const body = "A monoid is a set with an associative law and an identity."

	if err := archiveChunk(root, "vi", source,
		translate.Chunk{Index: 2, Of: 25, Body: body}, 1,
		asked(t, body), "Vị nhóm là một tập hợp.", ""); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "work", "translate", "vi", strings.ReplaceAll(source, "/", "_"))
	if err := os.Remove(filepath.Join(dir, "002-1.ask.md")); err != nil {
		t.Fatal(err)
	}
	if got := archivedAnswers(root, "vi", source, 2, body); len(got) != 0 {
		t.Errorf("an answer nothing can date was read back: %v", got)
	}
}
