package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/quality"
	"github.com/tamnd/bourbaki-solver/roundtrip"
	"github.com/tamnd/bourbaki-solver/translate"
)

// The sampled round trip, wired to a checkout and to the fleet.
//
// The rule is package roundtrip and it touches no disk. This is what reads the
// corpus, draws the sample, asks the hosts and writes the verdicts down.
//
// Two calls a file, and the first of them must not see the English. That is the
// only thing here that is easy to get wrong and fatal: a back translation made
// by a model that was shown the original is a copy of the original, every
// verdict comes back same, and the report says the translations are sound when
// nothing was measured. So the English is not loaded into the back translation
// question at all rather than merely left out of the prompt text.
//
// Verdicts are written after every file rather than at the end. A run over a
// full corpus is hours of fleet time and the fleet is a set of browsers on three
// machines, so a run that dies in the fourth hour is an ordinary event and not
// an exception. Nothing here is production, so there is no queue and no lease:
// a file that was in flight when the run stopped is simply not judged yet, which
// the next run sees as stale, and the cost of the crash is the one file.

const translateRoundTripUsage = `usage: bourbaki translate roundtrip [flags]

Takes a sample of the translations, has each one put back into English by a
model that has not seen the original, and asks a judge whether the two English
texts say the same mathematics.

Everything else that reads a translation proves it is the same text: the same
formulas in the same order, the same tags, the same headings, the same
citations, the same number of paragraphs, the right alphabet. All of that passes
on a fluent sentence that says the opposite of the book. This is the only thing
in the project that looks at what the sentences mean.

What comes back is a list of places to read, not a verdict on the corpus. A
difference could have been made on the way back as easily as on the way out, and
the judge is an unmeasured instrument, so the count is a floor on the problems
rather than a rate of them.

The sample is drawn by hashing the language and the path, so it is the same
sample for anybody with the same tree and cannot be drawn again until it reads
well. Five per cent by default, and never fewer than one file of a language that
has any, since a rate that rounds to nothing publishes a sampling rate beside a
sample that measured nothing.

  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -lang CODE     only this language
  -book ID       only this book
  -chapter ID    only this chapter, as VIII
  -file PATH     only this translation, relative to the corpus root
  -rate R        the share sampled, 0.05 by default
  -limit N       stop after this many files
  -force         judge again even where a verdict on this body is already in
  -list          print the sample and what is judged, and ask nothing
  -dry           print the first question and stop, without asking anything
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -ask N         the most text in one question to the judge, 20000 by default
  -deadline D    the longest one ask may take
  -keep          leave the questions on the boxes
  -report PATH   write the Markdown report here, reports/roundtrip.md by default
  -no-report     do not write the Markdown report
`

func runTranslateRoundTrip(args []string) error {
	fs := flag.NewFlagSet("translate roundtrip", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, translateRoundTripUsage) }
	dir := fs.String("corpus", "", "the checkout")
	lang := fs.String("lang", "", "only this language")
	book := fs.String("book", "", "only this book")
	chapter := fs.String("chapter", "", "only this chapter")
	file := fs.String("file", "", "only this translation")
	rate := fs.Float64("rate", roundtrip.Rate, "the share sampled")
	limit := fs.Int("limit", 0, "stop after this many files")
	force := fs.Bool("force", false, "judge again even where a verdict is in")
	list := fs.Bool("list", false, "print the sample and ask nothing")
	dry := fs.Bool("dry", false, "print the first question and stop")
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	ask := fs.Int("ask", roundtrip.JudgeChars, "the most text in one question to the judge")
	deadline := fs.Duration("deadline", chunkDeadline, "the longest one ask may take")
	keep := fs.Bool("keep", false, "leave the questions on the boxes")
	report := fs.String("report", "", "write the Markdown report here")
	noReport := fs.Bool("no-report", false, "do not write the Markdown report")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root := *dir
	if root == "" {
		var err error
		if root, err = corpus.Root(); err != nil {
			return err
		}
	}
	c, err := quality.Load(quality.Options{Root: root})
	if err != nil {
		return err
	}
	items, notEnglish := roundTripItems(c, *lang, *book, *chapter, *file)
	if notEnglish > 0 {
		// Said and not swallowed. A file left out of a measurement in silence
		// is a file the report implies was measured.
		fmt.Printf("translate roundtrip: %s made from something other than English, left out, since this compares two English texts\n",
			plural(notEnglish, "translation was", "translations were"))
	}
	if len(items) == 0 {
		fmt.Println("translate roundtrip: the corpus holds no translation to sample")
		return nil
	}
	sample := roundtrip.Draw(items, *rate)
	res, err := roundtrip.LoadResults(root)
	if err != nil {
		return err
	}
	res.Rate = *rate
	if *list {
		return listRoundTrip(sample, len(items), res)
	}

	pending := make([]roundtrip.Item, 0, len(sample))
	for _, it := range sample {
		if *force || res.Stale(it) {
			pending = append(pending, it)
		}
	}
	fmt.Printf("translate roundtrip: %s, %d in the sample at %.0f%%, %d to judge\n",
		plural(len(items), "translation", "translations"), len(sample), *rate*100, len(pending))
	if len(pending) == 0 {
		printRoundTripTally(sample, res)
		return writeRoundTripReport(root, *report, *noReport, sample, len(items), res)
	}
	if *limit > 0 && len(pending) > *limit {
		// Said out loud, for the reason the translate run says it: a run that
		// quietly stopped at the limit reads like a run that covered the sample.
		fmt.Printf("translate roundtrip: doing the first %d of them\n", *limit)
		pending = pending[:*limit]
	}
	bodies, err := roundTripBodies(c, pending)
	if err != nil {
		return err
	}
	if *dry {
		chunks := translate.Chunks(bodies[pending[0].Path].translation)
		fmt.Printf("\n%s: chunk 1 of %d, back into English\n\n%s\n",
			pending[0].Path, len(chunks), prompt.RoundTripBack(pending[0].Lang, chunks[0].Body))
		return nil
	}
	hosts, err := askHosts(*routeFile, *hostList)
	if err != nil {
		return err
	}
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// One worker to a host, pulling files off a channel. The work is uneven,
	// since a file is one call for every six thousand characters of it plus one
	// for every twenty thousand of the comparison, and handing every host a
	// fixed share of the list would leave two hosts idle while the third works
	// through the long sections of Theory of Sets.
	work := make(chan roundtrip.Item)
	var mu sync.Mutex
	var wg sync.WaitGroup
	judged, failed := 0, 0
	for _, h := range hosts {
		wg.Add(1)
		go func(h ocr.Host) {
			defer wg.Done()
			for it := range work {
				b := bodies[it.Path]
				v, err := roundTripOne(ctx, root, h, it, b, *ask, *keep, *deadline, logf)
				mu.Lock()
				if err != nil {
					failed++
					logf("%s: not judged, %v", it.Path, err)
				} else {
					judged++
					res.Put(v)
					// After every file. See the comment at the top.
					if err := res.Save(root); err != nil {
						logf("%s: the verdict could not be written, %v", it.Path, err)
					}
					if v.Same {
						logf("%s: came back saying the same mathematics", it.Path)
					} else {
						logf("%s: %d differences", it.Path, len(v.Differences))
					}
				}
				mu.Unlock()
				if ctx.Err() != nil {
					return
				}
			}
		}(h)
	}
	for _, it := range pending {
		if ctx.Err() != nil {
			break
		}
		work <- it
	}
	close(work)
	wg.Wait()

	fmt.Printf("translate roundtrip: %d judged, %d not\n", judged, failed)
	printRoundTripTally(sample, res)
	if err := writeRoundTripReport(root, *report, *noReport, sample, len(items), res); err != nil {
		return err
	}
	return ctx.Err()
}

// bodyPair is the two texts a file needs: the translation that goes out and the
// English it is held against when it comes back.
type bodyPair struct {
	translation string
	english     string
}

// roundTripItems is every translated file the corpus can pair with an English
// source, narrowed by the flags.
//
// It uses quality.Pairs, which is the same pairing the audit rules make and the
// translation report makes. A third way of deciding which English a file came
// from would be a third answer, and a sample drawn over a set no rule looks at
// is a measurement of something other than the corpus.
//
// A translation made from something other than English is left out, and the
// caller is told how many. content/en-mt is the case: it is the French volumes
// read into English, so its source is French, and this loop would ask for it to
// be put back into English and then compare two English texts one of which is a
// paraphrase of the other. Every verdict would come back the same and none of
// them would mean anything. Holding a French reading against its French is a
// real job and a different one, with a different prompt and a judge working in
// French, and calling it done here because the command did not crash would be
// the worse outcome of the two.
func roundTripItems(c *quality.Corpus, lang, book, chapter, file string) ([]roundtrip.Item, int) {
	var out []roundtrip.Item
	notEnglish := 0
	for _, p := range c.Pairs() {
		d := p.Translation
		if p.English.Lang != "en" {
			notEnglish++
			continue
		}
		if lang != "" && d.Lang != lang {
			continue
		}
		if file != "" && d.Path != file {
			continue
		}
		if book != "" && !roundTripInBook(d.Path, book) {
			continue
		}
		if chapter != "" && !roundTripInChapter(d, chapter) {
			continue
		}
		out = append(out, roundtrip.Item{
			Path: d.Path, English: p.English.Path, Lang: d.Lang,
			Digest: corpus.ContentSHA256(d.Body),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, notEnglish
}

// roundTripInBook matches content/<lang>/<book>/... on the middle element.
func roundTripInBook(path, book string) bool {
	parts := strings.Split(path, "/")
	return len(parts) > 2 && parts[2] == book
}

func roundTripInChapter(d quality.Doc, chapter string) bool {
	switch {
	case d.Section != nil:
		return d.Section.Chapter == chapter
	case d.Exercise != nil:
		return d.Exercise.Chapter == chapter
	}
	return false
}

// roundTripBodies reads both halves of every file that is going to be judged,
// before a host is asked for anything.
//
// All of it up front, the way solve eval reads its answers up front. A sample
// that names an English file the corpus does not hold measures a smaller thing
// than it says it does, and finding that out an hour in, one file at a time, is
// finding it out too late to do anything about.
func roundTripBodies(c *quality.Corpus, items []roundtrip.Item) (map[string]bodyPair, error) {
	byPath := map[string]string{}
	for _, d := range c.Sources {
		byPath[d.Path] = d.Body
	}
	for _, d := range c.Docs {
		byPath[d.Path] = d.Body
	}
	out := map[string]bodyPair{}
	for _, it := range items {
		tr, ok := byPath[it.Path]
		if !ok {
			return nil, fmt.Errorf("%s is in the sample and the corpus does not hold it", it.Path)
		}
		en, ok := byPath[it.English]
		if !ok {
			return nil, fmt.Errorf("%s was translated from %s and there is no such file", it.Path, it.English)
		}
		out[it.Path] = bodyPair{translation: tr, english: en}
	}
	return out, nil
}

// roundTripOne is one file's trip: back into English in chunks, then judged
// against the English it was made from, segment by segment.
func roundTripOne(ctx context.Context, root string, host ocr.Host, it roundtrip.Item, b bodyPair,
	ask int, keep bool, deadline time.Duration, logf func(string, ...any)) (roundtrip.Verdict, error) {

	chunks := translate.Chunks(b.translation)
	if len(chunks) == 0 {
		return roundtrip.Verdict{}, fmt.Errorf("the translation is empty")
	}
	answers := make([]string, 0, len(chunks))
	backModel := ""
	for _, ch := range chunks {
		q := prompt.RoundTripBack(it.Lang, ch.Body)
		a, err := roundTripAsk(ctx, host, q,
			fmt.Sprintf("roundtrip-back-%s-%d", roundTripID(it.Path), ch.Index),
			"roundtrip back "+it.Lang, fmt.Sprintf("%s chunk %d of %d", it.Path, ch.Index, ch.Of),
			root, keep, deadline, logf)
		if err != nil {
			return roundtrip.Verdict{}, err
		}
		answers = append(answers, a.Text)
		backModel = a.Model
	}
	back := translate.Join(answers)
	segs, err := roundtrip.Segments(b.english, back, ask)
	if err != nil {
		return roundtrip.Verdict{}, err
	}
	v := roundtrip.Verdict{
		Path: it.Path, English: it.English, Lang: it.Lang, Digest: it.Digest,
		BackModel: backModel, On: time.Now().UTC().Format("2006-01-02"),
		Same: true, Back: back,
	}
	for _, s := range segs {
		q := prompt.RoundTripJudge(s.English, s.Back)
		a, err := roundTripAsk(ctx, host, q,
			fmt.Sprintf("roundtrip-judge-%s-%d", roundTripID(it.Path), s.Index),
			"roundtrip judge "+it.Lang, fmt.Sprintf("%s part %d of %d", it.Path, s.Index, s.Of),
			root, keep, deadline, logf)
		if err != nil {
			return roundtrip.Verdict{}, err
		}
		same, diffs, err := roundtrip.ParseJudgement(a.Text)
		if err != nil {
			return roundtrip.Verdict{}, fmt.Errorf("part %d of %d: %w", s.Index, s.Of, err)
		}
		v.JudgeModel = a.Model
		v.Differences = append(v.Differences, diffs...)
		if !same {
			v.Same = false
		}
	}
	return v, nil
}

func roundTripAsk(ctx context.Context, host ocr.Host, question, id, stage, target, root string,
	keep bool, deadline time.Duration, logf func(string, ...any)) (ocr.Answer, error) {

	call := ocr.NewAskWithin(host, fleet.SSH{Timeout: 2 * time.Minute}, ocr.Rsync{Timeout: 5 * time.Minute},
		question, id, keep, deadline)
	return ocr.Recorded{Asker: call, Stage: stage, Host: host.Name, Target: target,
		Chars: len(question), Note: noteAsks(root, logf)}.Do(ctx)
}

// roundTripID turns a path into something that can be a directory name on a
// host, since that is what the ask id becomes.
func roundTripID(path string) string {
	s := strings.TrimSuffix(path, ".md")
	s = strings.ReplaceAll(s, "/", "-")
	return strings.ReplaceAll(s, " ", "_")
}

func listRoundTrip(sample []roundtrip.Item, total int, res *roundtrip.Results) error {
	fmt.Printf("%s, %d in the sample\n\n", plural(total, "translation", "translations"), len(sample))
	for _, it := range sample {
		state := "waiting"
		if v := res.Find(it.Lang, it.Path); v != nil {
			switch {
			case res.Stale(it):
				state = "translated again since it was judged"
			case v.Same:
				state = "same mathematics"
			default:
				state = fmt.Sprintf("%d differences", len(v.Differences))
			}
		}
		fmt.Printf("%-64s %s\n", it.Path, state)
	}
	fmt.Println()
	printRoundTripTally(sample, res)
	return nil
}

func printRoundTripTally(sample []roundtrip.Item, res *roundtrip.Results) {
	for _, c := range roundtrip.Tally(sample, res) {
		fmt.Println(c.Line())
	}
}

// writeRoundTripReport writes the Markdown a person reads.
//
// The differences are the report and the counts are the heading over them. A
// report that gave the rate and left the findings in a JSON file would be a
// report nobody acts on, and the whole reason for the loop is that somebody
// goes and reads the two passages.
func writeRoundTripReport(root, path string, skip bool, sample []roundtrip.Item, total int, res *roundtrip.Results) error {
	if skip {
		return nil
	}
	if path == "" {
		path = filepath.Join(root, "reports", "roundtrip.md")
	}
	var b strings.Builder
	b.WriteString("# Round trip\n\n")
	b.WriteString("A sample of the translations, put back into English by a model that had not seen the original, and judged against the English they were made from. What is listed here is places to read and not faults that are proved: a difference can be made on the way back as easily as on the way out, and the judge is itself unmeasured. The sample is drawn by hashing the language and the path, so it is the same sample for anybody with this tree.\n\n")
	fmt.Fprintf(&b, "%s, %d sampled at %.0f per cent.\n\n",
		plural(total, "translation", "translations"), len(sample), res.Rate*100)
	for _, c := range roundtrip.Tally(sample, res) {
		fmt.Fprintf(&b, "- %s\n", c.Line())
	}
	b.WriteString("\n")
	n := 0
	for _, it := range sample {
		v := res.Find(it.Lang, it.Path)
		if v == nil || res.Stale(it) || v.Same {
			continue
		}
		n++
		fmt.Fprintf(&b, "## %s\n\n", it.Path)
		fmt.Fprintf(&b, "Made from `%s`, back on %s.\n\n", v.English, v.BackModel)
		for _, d := range v.Differences {
			fmt.Fprintf(&b, "**%s.** %s\n\n", d.Kind, d.Why)
			fmt.Fprintf(&b, "> %s\n\n", oneLine(d.English))
			fmt.Fprintf(&b, "came back as\n\n")
			fmt.Fprintf(&b, "> %s\n\n", oneLine(d.Back))
		}
	}
	if n == 0 {
		b.WriteString("Nothing in the sample came back saying different mathematics. That is worth reading as what it is: the judge found nothing, on the files the draw picked, on the day it ran.\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// oneLine folds a quotation onto one line, since a block quote broken across
// lines renders as several quotes.
func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// plural is the count with the word after it in the right number.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
