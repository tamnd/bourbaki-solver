package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/quality"
	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/translate"
)

// The durable half of translating.
//
// A section is fifteen chunks and eleven minutes, and the largest section in
// chapter VIII is a hundred thousand characters, which is over two hours. A run
// that keeps its answers in memory and writes the file at the end throws all of
// that away when a tunnel drops, and the chapter is twenty six sections. So each
// chunk is a job on the queue and each accepted answer is a file, and a run that
// is killed halfway costs the chunk it was holding and nothing else.
//
// The unit is the chunk and not the section, because the section is what cannot
// be partly written. Half a translated § is worse than none: the audit counts it
// as translated and a reader has no way to see where the English stopped being
// carried over. A chunk has no such problem, since it is only ever joined back
// with the others and audited whole before anything reaches content/.
//
// What the queue adds over the answers on disk is the bound. A chunk the model
// cannot get right is asked three times across three runs and is then dead, and
// dead is a state somebody reads, rather than a section that quietly refuses
// itself every night for a week.

// translateGroup names the queue group a section's chunks belong to.
//
// Lease takes a group so that a worker cannot pick up work from somewhere else,
// and here somewhere else is another section: this run asks for the chunks of
// one section at a time and joins them, so a chunk of the next section leased
// into this one would be a chunk nobody puts anywhere. The name is the language
// and the source, which is what chunkID already names a box's scratch directory
// with, so the two line up when somebody is reading both.
func translateGroup(lang, source string) string {
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("%s-%s", lang, hex.EncodeToString(sum[:])[:6])
}

func translateTarget(lang, source string, index int) string {
	return fmt.Sprintf("%s/%03d", translateGroup(lang, source), index)
}

// chunkInput is the content address of one chunk of work.
//
// The terminology is in it as well as the text. A glossary row that reaches this
// section changes the question, so it has to change the job: without it, the
// chunk is done, the queue says so, and the next run reads back an answer made
// under terminology nobody uses any more. This is the same fact the front matter
// records as glossary_terms_sha256, one level down.
func chunkInput(body, terms string) string {
	sum := sha256.Sum256([]byte(body + "\x00" + terms))
	return hex.EncodeToString(sum[:])
}

// copiedModel is what the front matter records for a chunk no model was shown,
// because there was nothing in it to translate. It is a name no route has and no
// rule about cut down or free models matches, which is the point: the text came
// off the English page and not off a model at all.
const copiedModel = "copied"

// accepted is a chunk's answer, kept because the run that asked for it may not
// be the run that writes the section.
type accepted struct {
	Source string `json:"source"`
	Chunk  int    `json:"chunk"`
	Of     int    `json:"of"`
	Input  string `json:"input_sha256"`
	Prompt string `json:"prompt_sha256"`
	Model  string `json:"model"`
	Text   string `json:"text"`
}

// acceptedPath is beside the questions and the answers of every attempt, under
// work, which is not in the repository.
func acceptedPath(root, lang, source string, index int) string {
	dir := filepath.Join(root, "work", "translate", lang, strings.ReplaceAll(source, "/", "_"))
	return filepath.Join(dir, fmt.Sprintf("%03d.accepted.json", index))
}

// writeAccepted puts the answer down by temp file and rename, so a reader sees
// the whole of one answer or none of it.
func writeAccepted(root, lang string, a accepted) error {
	path := acceptedPath(root, lang, a.Source, a.Chunk)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "accepted.*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.Write(append(raw, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// readAccepted is the answer already on disk for this exact chunk, if there is
// one.
//
// The input hash is compared rather than trusted. work/ is not in the repository
// and nothing stops a person from re-extracting the English underneath it, and
// an answer to a question nobody would ask today is not an answer.
//
// The prompt hash is compared for the same reason, and it is the half that was
// missing. The queue keys a job on the instructions as well as the text, so a
// rewritten prompt puts every chunk back on the list, but the answer beside it
// was read back anyway and the run wrote a file that claimed the new
// instructions and held the old translation. That is how the rule about words
// inside a formula would have shipped without a single line of the book
// actually being asked again. An answer with no prompt recorded is from before
// this field and cannot say what it was asked, so it is asked again.
func readAccepted(root, lang, source string, index int, input, promptHash string) (accepted, bool) {
	raw, err := os.ReadFile(acceptedPath(root, lang, source, index))
	if err != nil {
		return accepted{}, false
	}
	var a accepted
	if err := json.Unmarshal(raw, &a); err != nil {
		return accepted{}, false
	}
	if a.Input != input || a.Prompt != promptHash || strings.TrimSpace(a.Text) == "" {
		return accepted{}, false
	}
	return a, true
}

// chunkDeadline is the longest one ask of one chunk may take, on a fleet
// answering at the speed the fleet answered at when this was measured.
//
// A chunk of six thousand characters has measured between forty and seventy
// seconds, and five minutes is what the gateway routes already allow, so a box
// is held to the same. Without it a box question takes the page default, which
// is forty five minutes, and that is a number for a photograph of a page and
// not for this. server3 sat on three of these for nine minutes with nothing
// coming back, which is three of its four lanes doing nothing while the queue
// counted them as busy.
//
// It is the default of -deadline rather than the law, because the number
// measures the fleet and the fleet is not always the same. On a slow afternoon
// bourbaki fleet ask -host server3 took three minutes four seconds to answer
// the word ok, and under a five minute cap the eight footnote chunks of the
// historical note of chapter IV timed out on every pass on both boxes while
// chapter III was going through the same routes at one to two minutes a chunk.
// Nothing there is the question failing. Raising the cap for that run is what
// the flag is for, and it stays a cap: a box that answers nothing still loses
// the lane rather than the section.
const chunkDeadline = 5 * time.Minute

// maxChunkDeadline is as far as -deadline may be pushed.
//
// The reason the cap exists at all is that a lane holding a question is a lane
// not asking anything else, so a deadline long enough to be a page timeout puts
// the run back where it was before there was one. Twenty minutes is the
// slowest a browser route says it will ever be, in route.Default, so no honest
// chunk needs more than that.
const maxChunkDeadline = 20 * time.Minute

// chunkLeaseFor is how long a worker says it will be, for a given deadline.
//
// It has to outlast the work, or the queue hands the chunk to somebody else
// while the first lane is still legitimately on it and the book gets asked for
// twice. A chunk that does not pass is asked a second time inside the same
// lease, so the lease is two deadlines plus the ssh and rsync either side of
// each, and the queue adds its own slack on top of that. It was three minutes,
// which was under one deadline let alone two.
func chunkLeaseFor(deadline time.Duration) time.Duration { return 2*deadline + 2*time.Minute }

// chunkLease is chunkLeaseFor the default deadline.
const chunkLease = 2*chunkDeadline + 2*time.Minute

// chunkOf finds the chunk a leased job is for.
//
// By the number in the target rather than by the order jobs come back in, since
// two lanes are leasing at once and the answer has to go back where it came
// from.
func chunkOf(j job, item queue.Job) (int, translate.Chunk, bool) {
	_, number, ok := strings.Cut(item.Target, "/")
	if !ok {
		return 0, translate.Chunk{}, false
	}
	index, err := strconv.Atoi(number)
	if err != nil {
		return 0, translate.Chunk{}, false
	}
	for _, c := range j.chunks {
		if c.Index == index {
			return index, c, true
		}
	}
	return 0, translate.Chunk{}, false
}

// chunksOutstanding is how many chunks of one file a run would still have to ask
// for, which is the file's chunks less the ones with an answer already in hand.
//
// It asks the same question plan asks and asks it the same way, through
// readAccepted and the audit, so the count here is the count a run would meet.
// What it does not do is touch the queue: plan supersedes ids and resets jobs as
// it goes, which is right when a run is about to start and wrong when the only
// thing being done is counting. So this reads the archive and nothing else, and
// a chunk that is leased or dead counts as outstanding, which is what it is.
func chunksOutstanding(root, lang, promptHash string, j job, redoSmall bool) int {
	left := 0
	for _, c := range j.chunks {
		a, ok := readAccepted(root, lang, j.source, c.Index, chunkInput(c.Body, j.terms), promptHash)
		if ok && !(redoSmall && quality.SmallModel(a.Model)) && len(translate.Audit(lang, c.Body, a.Text)) == 0 {
			continue
		}
		left++
	}
	return left
}

// plan decides, for every chunk of one section, whether the answer is in hand,
// whether the chunk goes on the queue, and whether it is out of attempts.
//
// The four cases are worth naming because three of them are quiet failures if
// they are left out. A chunk whose answer is on disk must not be asked again, or
// the queue buys nothing. A chunk the queue calls done whose answer is not on
// disk must be asked again, or the section can never be written: work/ is
// scratch and gets cleaned, and a record that says done when the answer is gone
// is a record that has to follow the answer. A chunk that is dead must not be
// quietly retried, because three attempts that fail the same way are what dead
// means and a person is meant to read it. And -force must reach a chunk that is
// done, which is the whole of what -force is for: the answer is there and it was
// written on a cut down model.
func plan(q *queue.Queue, root, lang, promptHash string, j job, force, redoSmall bool) (have map[int]accepted, queued int, stuck map[int][]translate.Problem, err error) {
	have, stuck = map[int]accepted{}, map[int][]translate.Problem{}

	// What this section's chunks are asked under today, so that the versions of
	// them asked under yesterday's instructions can go. See Queue.Supersede: a
	// worker leases by group and reads the chunk number out of the target, so a
	// superseded job is indistinguishable from the job that replaced it once it
	// is in a lane, and the section is asked for twice.
	keep := map[string]string{}
	for _, c := range j.chunks {
		target := translateTarget(lang, j.source, c.Index)
		keep[target] = queue.NewID(queue.StageTranslate, target, chunkInput(c.Body, j.terms), promptHash)
	}
	if _, err := q.Supersede(queue.StageTranslate, keep); err != nil {
		return nil, 0, nil, err
	}

	for _, c := range j.chunks {
		input := chunkInput(c.Body, j.terms)
		target := translateTarget(lang, j.source, c.Index)
		id := queue.NewID(queue.StageTranslate, target, input, promptHash)

		if !force {
			// An answer a cut down model gave is passed over under -redo-small,
			// and only that answer. -force would ask for all forty two chunks
			// of chapter I, § 1 again to replace the four of them that came
			// back on gpt-5-6-mini, which is thirty eight questions nobody
			// needs to put and, on the free gateway, most of a day.
			//
			// An answer today's rules refuse is passed over as well, and this
			// is § 7 of chapter III. Two of its chunks were accepted weeks ago
			// with \textit{whenever} copied through, because copying it was
			// what the rules asked for then; the word counts as prose now, the
			// file is put together out of every accepted chunk, and the audit
			// of the whole file refused it over two spans no chunk was ever
			// going to be asked about again. A cached answer that the rule
			// standing today will not accept is not an answer, for the same
			// reason readAccepted compares the hashes rather than trusting
			// them. The terminology is not asked here because it is inside the
			// input hash already, so a glossary that moved has re-asked the
			// chunk before this line is reached.
			if a, ok := readAccepted(root, lang, j.source, c.Index, input, promptHash); ok &&
				!(redoSmall && quality.SmallModel(a.Model)) &&
				len(translate.Audit(lang, c.Body, a.Text)) == 0 {
				have[c.Index] = a
				continue
			}
		}
		_, state, findErr := q.Find(queue.StageTranslate, id)
		switch {
		case findErr != nil && !os.IsNotExist(findErr):
			return nil, 0, nil, findErr
		case state == queue.Dead && !force:
			stuck[c.Index] = []translate.Problem{{Rule: "queue", Msg: "it is dead after every attempt, " +
				"read it with bourbaki queue list -stage translate -state dead and put it back with bourbaki queue retry -stage translate"}}
			continue
		case state == queue.Leased:
			stuck[c.Index] = []translate.Problem{{Rule: "queue",
				Msg: "it is leased by another run, so this one is not asking for it"}}
			continue
		case state == queue.Done || state == queue.Failed || force:
			// Done with no answer beside it, or a person asking again. Either
			// way the work is the same work, so it keeps its id and its history
			// rather than arriving as something new.
			if _, err := q.Reset(queue.StageTranslate, id); err != nil {
				return nil, 0, nil, err
			}
		}
		item := queue.New(queue.StageTranslate, target, input, promptHash)
		item.Meta = map[string]string{
			"lang": lang, "source": j.source,
			"chunk": strconv.Itoa(c.Index), "of": strconv.Itoa(c.Of),
		}
		if _, err := q.Add(item); err != nil {
			return nil, 0, nil, err
		}
		queued++
	}
	return have, queued, stuck, nil
}
