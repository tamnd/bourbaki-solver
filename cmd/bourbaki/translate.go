package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/quality"
	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/report"
	"github.com/tamnd/bourbaki-solver/translate"
)

// Translating the corpus.
//
// The shape is the glossary command's, one level up: chunks instead of batches,
// a section instead of a term, and the same fleet under it. What is different
// is what happens to an answer that does not pass. A refused rendering costs
// one row and the next run asks again; a refused chunk would cost the section,
// because a section is only written when all of its chunks are in, so a chunk
// that fails is asked again once inside this run with the audit's own complaint
// attached to the question.
//
// That second ask is the only place in this program where a model is told what
// it did wrong. It is worth it here and it was not worth it for the glossary,
// because the failures are different in kind: a bad rendering is a model that
// does not know the word, which telling it does not fix, and a failed chunk is
// almost always a model that reflowed a formula or dropped a blank line, which
// telling it does fix.
//
// Nothing here decides whether a translation is right. That is package
// translate, which proves the answer is the same section: the same mathematics
// in the same order, the same tags, the same headings with the same numbers,
// the same citations, the same number of blocks, and written in the language
// that was asked for. A fluent sentence that says the opposite of the English
// passes all of it. That is what the sampled round trip and a reader are for.

const translateUsage = `usage: bourbaki translate -lang CODE [flags]

Translates the English of the corpus into Vietnamese, Chinese or Japanese, and
refuses any answer that is not the same text.

Both kinds of file go over. A § is what a reader reads first and an exercise is
what a reader does next, and a volume that has its §§ in Vietnamese and its
exercises in English is not a volume anybody can work through. Theory of Sets is
28 §§ and 211 exercises, so the exercises are seven eighths of the files and
about a fifth of the words.

  -lang CODE     vi, zh or ja, required
  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -book ID       only this book, as books.yaml names it
  -chapter ID    only this chapter, as VIII
  -file PATH     only this English file, relative to the corpus root
  -limit N       stop after this many files
  -force         translate again even where a translation is already there and
                 not stale
  -redo-small    ask again for whatever a cut down model answered, and only for
                 that
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -dry           print the first question and stop, without asking anything
  -stale         list what needs translating and why, and ask nothing
  -check-glossary  hold the translations already on disk to the glossary, term
                 by term, and ask nothing
  -all           with -check-glossary, every term and not only the missed ones
  -keep          leave the questions on the boxes, for debugging
  -deadline DUR  the longest one ask of one chunk may take, default 5m, up to
                 20m, for a day the boxes are answering slowly
  -queue PATH    the work list, default $BOURBAKI_WORK/queue

A file is skipped when its translation is there, its source_content_sha256 is
the English it was made from, its glossary_terms_sha256 is the digest of the
glossary rows that English mentions today, and its prompt_sha256 is this
binary's. Any of those four out of date is what stale means, and a stale file is
translated again. An exercise records no content_sha256 of its own, so the
second of the four hashes the English body rather than reading a number out of
its head.

The terminology test is per file rather than on glossary_version, which moves
for any edit anywhere. Pinning "common zero", a phrase that occurs in one
appendix, moved the version and so marked all 27 sections of chapter VIII stale.
Measured on this corpus, that row reaches 1 section and "algebraic over" reaches
3, while "ring" reaches 26 and "module" 23, which is the difference the digest
keeps and the version threw away.

A section goes over in chunks, cut at blank lines, because the largest section
in chapter VIII is 97,520 characters and no measurement here says a browser
composer will take that. The chunks are put back together with a blank line
between them, which is exactly how they were cut, and the whole file is audited
again after the join.

-deadline is the cap on one ask. Five minutes fits a fleet that answers a chunk
in forty to seventy seconds, and it does not fit a fleet that takes three
minutes to answer the word ok, which is what bourbaki fleet ask measured on
server3 on the afternoon the historical note of chapter IV would not go
through: eight short chunks timed out on both boxes, pass after pass, while
chapter III went over the same routes. A run on a slow day is worth more time
per question. It is still a cap, and a lane that spends it has still spent it
doing nothing, so raise it for the run that needs it rather than in the file.

Every question and every answer is kept under work/translate, which is not in
the repository. What goes in the repository is the file, and the file is only
written when the audit found nothing.

Every chunk is a job on the queue and every accepted answer is a file beside the
questions, so a run that is killed costs the chunks in flight and nothing else:
the next run reads the answers that are there and asks only for the rest. That
matters at this size. A section of fifteen chunks is eleven minutes and the
largest section in chapter VIII is over two hours, and a run that held its
answers in memory threw all of it away when a tunnel dropped.

A chunk that fails is asked again on the next run, three times in all, and is
then dead. Dead is a state somebody reads: bourbaki queue list -stage translate
-state dead says which chunks and why, and bourbaki queue retry -stage translate
puts them back. Until then the section they belong to is refused, and refused
with the reason rather than in silence.

-force reaches the answers as well as the file. A section that is not stale is
translated again, and its chunks are asked again rather than read back off disk,
which is what -force is for: the section is there and it was written while the
account was being served a cut down model.

-redo-small is the same intention with a smaller bill. It takes the files L08
names, the ones whose translation_model holds a cut down model, and it asks
again for the chunks that model answered and for no others. Chapter I, § 1 is
forty two chunks and four of them came back on gpt-5-6-mini, so -force is
thirty eight questions nobody needs to put. Across the volume it was 22 answers
out of 769. Nothing else moves: a file that is stale is stale for its own
reason, and a file nobody has translated is still untranslated.
`

func runTranslate(args []string) error {
	fs := flag.NewFlagSet("translate", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, translateUsage) }
	dir := fs.String("corpus", "", "the checkout")
	lang := fs.String("lang", "", "vi, zh or ja")
	book := fs.String("book", "", "only this book")
	chapter := fs.String("chapter", "", "only this chapter")
	file := fs.String("file", "", "only this English file")
	limit := fs.Int("limit", 0, "stop after this many sections")
	force := fs.Bool("force", false, "translate again even where it is not stale")
	redoSmall := fs.Bool("redo-small", false, "ask again for whatever a cut down model answered")
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	dry := fs.Bool("dry", false, "print the first question and stop")
	stale := fs.Bool("stale", false, "list what needs translating and why, and ask nothing")
	checkGlossary := fs.Bool("check-glossary", false, "hold what is on disk to the glossary and ask nothing")
	all := fs.Bool("all", false, "with -check-glossary, every term and not only the missed ones")
	keep := fs.Bool("keep", false, "leave the questions on the boxes")
	deadline := fs.Duration("deadline", chunkDeadline, "the longest one ask of one chunk may take")
	queueRoot := fs.String("queue", defaultQueueRoot(), "queue directory")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	// French is a source and not a target. The corpus holds 28 French volumes
	// and they are the original, so content/fr is extracted like content/en and
	// translating into it would be translating a translation back into the
	// language it came from. It is worth its own sentence because -lang fr is
	// an easy thing to type once the manifest lists French volumes.
	if *lang == "fr" {
		return fmt.Errorf("french is a source language, not a target: content/fr is extracted from the French printing, not translated into")
	}
	if !known(*lang) {
		fs.Usage()
		os.Exit(2)
	}
	if *deadline <= 0 || *deadline > maxChunkDeadline {
		return fmt.Errorf("-deadline is %s, and one ask of one chunk is held to between nothing and %s", *deadline, maxChunkDeadline)
	}
	root, err := corpusRoot(*dir)
	if err != nil {
		return err
	}
	g, err := glossary.Load(root)
	if err != nil {
		return err
	}
	if len(g.In(*lang)) == 0 {
		return fmt.Errorf("the glossary has no %s in it, run bourbaki glossary translate -lang %s first", *lang, *lang)
	}
	// Before the run rather than after it. The pass that follows costs fleet
	// time per section, and a term the last pass rendered three ways is a term
	// this one will render three ways too unless the row is fixed first.
	if *checkGlossary {
		return checkGlossaryOnDisk(root, *lang, *book, *chapter, *file, *all)
	}
	promptHash, err := prompt.TranslateSHA256(*lang)
	if err != nil {
		return err
	}
	jobs, skipped, err := translateJobs(root, g, *lang, *book, *chapter, *file, promptHash, *force, *redoSmall)
	if err != nil {
		return err
	}
	if *stale {
		return reportStale(jobs, skipped, *lang)
	}
	if len(jobs) == 0 {
		fmt.Printf("translate: nothing to do, %d files are already translated and current\n", skipped)
		return nil
	}
	if *limit > 0 && len(jobs) > *limit {
		// Said out loud, because a run that quietly stopped at the limit reads
		// exactly like a run that covered everything.
		fmt.Printf("translate: %d files need %s, doing the first %d\n", len(jobs), *lang, *limit)
		jobs = jobs[:*limit]
	} else {
		fmt.Printf("translate: %d files need %s, %d are current\n", len(jobs), *lang, skipped)
	}
	if *dry {
		question, err := translateQuestion(g, *lang, jobs[0].chunks[0].Body)
		if err != nil {
			return err
		}
		fmt.Printf("\n%s: chunk 1 of %d\n\n%s\n", jobs[0].source, len(jobs[0].chunks), question)
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

	// Anything a worker was holding when it died comes back before this run
	// starts, or the chunks it had sit in leased until their deadline passes and
	// this run reports a section it never asked half of.
	q, err := queue.Open(*queueRoot)
	if err != nil {
		return err
	}
	reaped, err := q.Reap(queue.StageTranslate)
	if err != nil {
		return err
	}
	if len(reaped) > 0 {
		logf("%d chunks came back from a run that did not finish", len(reaped))
	}

	run := runID(*lang, promptHash, jobs)
	var written, refused int
	for _, job := range jobs {
		body, model, problems := translateFile(ctx, root, q, hosts, g, *lang, promptHash, job, *force, *redoSmall, *keep, *deadline, logf)
		if len(problems) > 0 {
			refused++
			logf("%s: refused, nothing written", job.source)
			for _, p := range problems {
				logf("\t%s", p)
			}
			continue
		}
		path, err := writeTranslation(root, *lang, run, promptHash, g.Version, job, body, model)
		if err != nil {
			return err
		}
		written++
		logf("%s: written, %d chunks", rel(root, path), len(job.chunks))
		if ctx.Err() != nil {
			break
		}
	}
	fmt.Printf("translate: %d files written, %d refused, %d were already current\n", written, refused, skipped)
	return ctx.Err()
}

// writeTranslation puts the finished file where the language wants it, as a
// section or as an exercise.
//
// The two carry the same five facts about how they were made, and they have to,
// because staleness is decided by reading them back. What differs is only where
// the file goes and which schema its head is written in, and an exercise takes
// the hash of the English body rather than a hash copied out of the English
// head, since an exercise records none.
func writeTranslation(root, lang, run, promptHash string, version int, j job, body, model string) (string, error) {
	var path string
	var file interface{ Write(string) error }
	if j.ex != nil {
		out := *j.ex
		out.Lang = lang
		out.TranslatedFrom = j.source
		out.SourceSHA256 = corpus.ContentSHA256(j.body)
		out.TranslationModel = model
		out.TranslationRun = run
		out.GlossaryVersion = version
		out.GlossaryTerms = j.terms
		out.PromptSHA256 = promptHash
		path = corpus.ExercisePath(root, lang, out)
		file = corpus.ExerciseFile{Meta: out, Body: body}
	} else {
		out := j.meta
		out.Lang = lang
		out.TranslatedFrom = j.source
		out.SourceSHA256 = j.meta.ContentSHA256
		out.ContentSHA256 = corpus.ContentSHA256(body)
		out.TranslationModel = model
		out.TranslationRun = run
		out.GlossaryVersion = version
		out.GlossaryTerms = j.terms
		out.PromptSHA256 = promptHash
		path = corpus.SectionPath(root, lang, out)
		file = corpus.SectionFile{Meta: out, Body: body}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := file.Write(path); err != nil {
		return "", err
	}
	return path, nil
}

// reportStale prints what a run would do, and asks nothing.
//
// The reason is printed beside every file, because "stale" on its own is not
// actionable: the English changing means a section was re-extracted, the
// instructions changing means every file is going again, and the terminology
// changing means one glossary row reached this section. Those are three
// different sizes of job and the count on its own hides which one this is.
// checkGlossaryOnDisk holds the translations that are already written to the
// terminology, term by term, and says nothing about the ones that are not
// written: this is a check on the corpus and not a plan for a run, which is
// what -stale is.
//
// It reads the committed files through the same loader the audit uses, so the
// answer here and L06's answer are the same answer counted two ways. A term
// that is missed is not by itself a defect. Vietnamese inflects nothing but it
// compounds, and a rendering looked for as it stands will miss a sentence that
// carries the idea in a pronoun. What the count is for is the shape: one file
// missing a term is a sentence, and thirty files missing the same term is a row
// that is wrong.
func checkGlossaryOnDisk(root, lang, book, chapter, file string, all bool) error {
	g, err := glossary.Load(root)
	if err != nil {
		return err
	}
	c, err := quality.Load(quality.Options{Root: root})
	if err != nil {
		return err
	}
	opt := report.TermOptions{Book: book, Chapter: chapter, File: file, All: all}
	fmt.Print(report.TermTable(lang, report.Terms(c, g, lang, opt)))
	for _, t := range report.Translations(c, g) {
		if t.Lang == lang {
			fmt.Println(t.Line())
		}
	}
	return nil
}

func reportStale(jobs []job, skipped int, lang string) error {
	for _, j := range jobs {
		fmt.Printf("%-64s %s\n", j.source, j.why)
	}
	fmt.Printf("translate: %d files need %s, %d are current\n", len(jobs), lang, skipped)
	return nil
}

// A job is one English file and the chunks it was cut into.
//
// Two schemas sit in the corpus under content/en and both of them are prose a
// reader reads, so both are translated. ex is set on an exercise and meta on a
// section, and nothing else in the run cares which it is: the chunking, the
// queue, the questions and the audit are the same work either way. The one
// place it matters is where the answer is written, which is writeTranslation.
type job struct {
	source string // relative to the corpus root
	meta   corpus.SectionFrontMatter
	ex     *corpus.ExerciseFrontMatter // set when this file is an exercise
	body   string
	chunks []translate.Chunk
	terms  string // digest of the glossary rows this file's English mentions
	why    string // why the translation on disk is not the one this run would make
}

// translateJobs is the English that needs this language.
//
// The walk is over content/en rather than over the sections manifest, because
// the manifest records what the assembler produced and this has to translate
// what is on disk. The two agree today and the file is the thing being read.
func translateJobs(root string, g *glossary.Glossary, lang, book, chapter, only, promptHash string, force, redoSmall bool) ([]job, int, error) {
	dir := filepath.Join(root, "content", "en")
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Strings(paths)

	var jobs []job
	skipped := 0
	for _, path := range paths {
		j, ok, err := readJob(root, path)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			continue
		}
		switch {
		case only != "" && j.source != only:
			continue
		case book != "" && j.meta.Book != book:
			continue
		case chapter != "" && !strings.EqualFold(j.meta.Chapter, chapter):
			continue
		}
		// The glossary a file is held to is the one its own volume is
		// translated against, since a row can be scoped to a book. See
		// glossary.Glossary.For.
		j.terms = translate.GlossaryDigest(g.For(j.meta.Book), lang, j.body)
		var fresh bool
		if j.ex != nil {
			fresh, j.why = currentExercise(root, lang, j.source, *j.ex, j.body, g.Version, promptHash, j.terms)
		} else {
			fresh, j.why = current(root, lang, j.source, j.meta, g.Version, promptHash, j.terms)
		}
		if fresh && redoSmall {
			// The file is the one this run would make, and it is still not the
			// file this book wants: some of it came back on a model that was
			// handed out because an account had been throttled. That is not
			// staleness, since nothing about the question changed, so the four
			// hashes are right to call it current and L08 is what notices. A
			// person who has read L08 and wants those answered again says so
			// here.
			if model := translatedBy(root, lang, j); quality.SmallModel(model) {
				fresh, j.why = false, "it was translated by "+model+", which is a cut down model"
			}
		}
		if !force && fresh {
			skipped++
			continue
		}
		j.chunks = translate.Chunks(j.body)
		if len(j.chunks) == 0 {
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs, skipped, nil
}

// readJob reads one file under content/en as the kind of thing it is, and says
// no to anything that is neither.
//
// Which schema to try is decided by the path and not by trying both, because
// the two heads share enough field names that a wrong guess would parse rather
// than fail, and a section read as an exercise has section 0 and exercise 0 and
// would be written over the exercise before it. The path is how the corpus
// already names the difference, in ExercisePath and in the audit's loader, so
// this is the third place that reads it and not a new rule.
//
// Book and Chapter are copied onto meta for an exercise as well, since the
// -book and -chapter flags and the per volume glossary are asked for by those
// two fields and both heads carry them.
func readJob(root, path string) (job, bool, error) {
	source := rel(root, path)
	if strings.Contains(filepath.ToSlash(source), "/exercises/") {
		f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](path)
		if err != nil || f.Meta.Lang != "en" || f.Meta.Book == "" {
			return job{}, false, nil
		}
		meta := corpus.SectionFrontMatter{Book: f.Meta.Book, Chapter: f.Meta.Chapter}
		return job{source: source, meta: meta, ex: &f.Meta, body: f.Body}, true, nil
	}
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil || f.Meta.Lang != "en" || f.Meta.Book == "" {
		return job{}, false, nil
	}
	return job{source: source, meta: f.Meta, body: f.Body}, true, nil
}

// current asks whether the translation on disk is the one this run would make,
// and says why not when it is not.
//
// Four things have to hold and any one of them failing is what stale means: the
// file is there, it was made from this English, it was made with the same
// terminology, and it was made with these instructions. Three of the four are
// the spec's staleness; the fourth is the prompt hash, which the spec asks to be
// recorded and does not say to check, and checking it is the only thing that
// makes recording it worth doing.
//
// The terminology test is per file and not the glossary version. The version
// moves for any edit anywhere, so pinning a phrase that occurs in one appendix
// marked all 27 sections stale. What is compared is the digest of the rows this
// section's English mentions, which is exactly what its prompt carried. A file
// written before that digest existed has nothing to compare, so it falls back to
// the version and is stale on any bump, which is where every file already was.
func current(root, lang, source string, en corpus.SectionFrontMatter, version int, promptHash, terms string) (bool, string) {
	meta := en
	meta.Lang = lang
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](corpus.SectionPath(root, lang, meta))
	switch {
	case err != nil:
		return false, "there is no translation"
	case f.Meta.TranslatedFrom != source:
		return false, "it was made from " + f.Meta.TranslatedFrom
	case f.Meta.SourceSHA256 != en.ContentSHA256:
		return false, "the English has changed since"
	case f.Meta.PromptSHA256 != promptHash:
		return false, "the instructions have changed since"
	case f.Meta.GlossaryTerms == "":
		if f.Meta.GlossaryVersion != version {
			return false, fmt.Sprintf("it records no terminology and was made with glossary %d, which is now %d", f.Meta.GlossaryVersion, version)
		}
	case f.Meta.GlossaryTerms != terms:
		return false, "the terminology it was shown has changed"
	}
	return true, ""
}

// translatedBy is what the translation on disk says answered it.
//
// One string for the whole file, and it is a list when the chunks did not all
// come back from the same place, which is the usual case: "laguna-s-2.1-free,
// hy3-free, gpt-5-6, gpt-5-6-mini" is one real section of chapter I. That is
// why the caller hands the whole string to quality.SmallModel rather than
// picking it apart. One cut down model anywhere in a file is a file worth
// asking about again, and it is the same test L08 prints.
//
// A file that is not there answers with nothing, and nothing is not a cut down
// model, so a missing translation stays a missing translation and is stale for
// the ordinary reason.
func translatedBy(root, lang string, j job) string {
	if j.ex != nil {
		meta := *j.ex
		meta.Lang = lang
		f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](corpus.ExercisePath(root, lang, meta))
		if err != nil {
			return ""
		}
		return f.Meta.TranslationModel
	}
	meta := j.meta
	meta.Lang = lang
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](corpus.SectionPath(root, lang, meta))
	if err != nil {
		return ""
	}
	return f.Meta.TranslationModel
}

// currentExercise is the same four questions asked of a translated exercise.
//
// The only difference is the second one. A section compares the hash the
// English file records with the hash the translation recorded when it was made;
// an exercise records no hash of its own, so the English body is hashed here and
// compared with what the translation kept. Both sides go through
// corpus.ContentSHA256, which normalises first, so an editor's trailing space
// does not restale an exercise.
func currentExercise(root, lang, source string, en corpus.ExerciseFrontMatter, body string, version int, promptHash, terms string) (bool, string) {
	meta := en
	meta.Lang = lang
	f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](corpus.ExercisePath(root, lang, meta))
	switch {
	case err != nil:
		return false, "there is no translation"
	case f.Meta.TranslatedFrom != source:
		return false, "it was made from " + f.Meta.TranslatedFrom
	case f.Meta.SourceSHA256 != corpus.ContentSHA256(body):
		return false, "the English has changed since"
	case f.Meta.PromptSHA256 != promptHash:
		return false, "the instructions have changed since"
	case f.Meta.GlossaryTerms == "":
		if f.Meta.GlossaryVersion != version {
			return false, fmt.Sprintf("it records no terminology and was made with glossary %d, which is now %d", f.Meta.GlossaryVersion, version)
		}
	case f.Meta.GlossaryTerms != terms:
		return false, "the terminology it was shown has changed"
	}
	return true, ""
}

// translateFile asks for every chunk and puts the answers together.
//
// A chunk that fails after its second ask fails the section. There is no
// partial write: half a translated § is worse than none, because the audit
// counts it as a translated file and a reader has no way to see where the
// English stopped being carried over.
//
// What is asked for is what the queue hands out, and what is written down is
// every accepted answer as it arrives. A run that is killed at chunk twelve
// costs chunk twelve, and the next run asks for that one and joins it to the
// eleven already on disk.
// freshOnly says which hosts may take only a chunk nobody has got wrong yet.
//
// The route table has the cheap model ranked ahead of the full one and says of
// the full one that it answers what the cheap one gets wrong. It did not: any
// lane could take any pending chunk, so a chunk the cheap model refused went
// back on the pile and the cheap lane, which is the one asking first, picked it
// straight back up. Chunk 2 of chapter IV, § 2, exercise 6, thirty six formulae
// in one paragraph, went round that three times in fifty minutes and each time
// lost the same two spans. The full model is sitting beside it.
//
// So a cut down model gets first ask and no second one. It is the cheap way
// round: nearly every chunk is straightforward and the cheap model answers it,
// and the hard ones stop costing the whole section its attempts.
//
// This only applies where there is somewhere to escalate to. A run given
// nothing but cut down routes filters nothing, because a rule that leaves work
// for a lane that does not exist is a rule that translates nothing at all.
func freshOnly(hosts []ocr.Host) map[string]func(queue.Job) bool {
	big := false
	for _, h := range hosts {
		if !quality.SmallModel(h.Model) {
			big = true
		}
	}
	want := map[string]func(queue.Job) bool{}
	if !big {
		return want
	}
	for _, h := range hosts {
		if quality.SmallModel(h.Model) {
			want[h.Name] = func(job queue.Job) bool { return job.Attempts == 0 }
		}
	}
	return want
}

func translateFile(ctx context.Context, root string, q *queue.Queue, hosts []ocr.Host, g *glossary.Glossary, lang, promptHash string, j job, force, redoSmall, keep bool, deadline time.Duration, logf func(string, ...any)) (string, string, []translate.Problem) {
	have, queued, stuck, err := plan(q, root, lang, promptHash, j, force, redoSmall)
	if err != nil {
		return "", "", []translate.Problem{{Rule: "queue", Msg: err.Error()}}
	}
	if len(have) > 0 {
		logf("%s: %d of %d chunks were answered by an earlier run", j.source, len(have), len(j.chunks))
	}
	if queued > 0 {
		logf("%s: %d chunks to ask for", j.source, queued)
	}

	group := translateGroup(lang, j.source)
	fresh := freshOnly(hosts)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, host := range hosts {
		for lane := 0; lane < host.Lanes; lane++ {
			wg.Add(1)
			go func(host ocr.Host) {
				defer wg.Done()
				for ctx.Err() == nil {
					item, err := q.LeaseWhere(queue.StageTranslate, host.Name, group, fresh[host.Name], chunkLeaseFor(deadline))
					if errors.Is(err, queue.ErrEmpty) {
						return
					}
					if err != nil {
						logf("%s: %v", host.Name, err)
						return
					}
					index, c, ok := chunkOf(j, item)
					if !ok {
						// The target is built from the section and the chunk
						// number, so this is a job from a corpus that has moved
						// under the queue. It is nobody's to run.
						if _, err := q.Fail(item, "no chunk of this section answers to "+item.Target); err != nil {
							logf("%s: %v", host.Name, err)
						}
						continue
					}
					text, model, bad := askChunk(ctx, root, host, g, lang, j, c, keep, deadline, logf)
					if len(bad) > 0 && ctx.Err() != nil {
						// Somebody pressed Ctrl-C while this chunk was out. The
						// model did not get it wrong, so the attempt is given
						// back: three attempts are for three bad answers, and a
						// chunk that loses one every time a run is interrupted is
						// a chunk that dies of being interrupted.
						if err := q.Release(item, "the run was interrupted"); err != nil {
							logf("%s: %v", host.Name, err)
						}
						return
					}
					if transportOnly(bad) {
						// The provider never read the question. That is not the
						// chunk failing, and it must not spend one of the
						// chunk's three attempts: a gateway that is out of turns
						// for the day otherwise kills the whole section in a
						// minute and a half, which is what it did. Forty two
						// chunks of chapter I, § 1 went from pending to dead in
						// one minute fourteen seconds, and no model behind them
						// had been asked anything at all.
						//
						// The lane goes with it. A route that has just answered
						// 429 with fifteen hours on it will answer the next
						// chunk the same way, and the queue is shared, so
						// retiring this lane puts the work in front of one that
						// is still answering. What is left at the end is the
						// section refused with its chunks pending, which is the
						// true state of it, and the next run asks again.
						if err := q.Release(item, bad[0].Msg); err != nil {
							logf("%s: %v", host.Name, err)
						}
						logf("%s: this lane is done for now, %s", host.Name, bad[0].Msg)
						return
					}
					if len(bad) > 0 {
						state, err := q.Fail(item, bad[0].String())
						if err != nil {
							logf("%s: %v", host.Name, err)
						}
						if state == queue.Dead {
							bad = append(bad, translate.Problem{Rule: "queue",
								Msg: "and that was its last attempt, so it is dead until bourbaki queue retry -stage translate puts it back"})
						}
						mu.Lock()
						stuck[index] = bad
						mu.Unlock()
						continue
					}
					a := accepted{Source: j.source, Chunk: index, Of: c.Of,
						Input: item.InputSHA256, Prompt: promptHash, Model: model, Text: text}
					if err := writeAccepted(root, lang, a); err != nil {
						// The answer is in hand and cannot be put down, so the
						// job goes back rather than counting as done: the
						// alternative is a queue that says done and a section
						// that can never be joined.
						if _, err := q.Fail(item, "the answer could not be written: "+err.Error()); err != nil {
							logf("%s: %v", host.Name, err)
						}
						continue
					}
					if _, err := q.Finish(item, true, model); err != nil {
						logf("%s: %v", host.Name, err)
					}
					mu.Lock()
					have[index] = a
					mu.Unlock()
				}
			}(host)
		}
	}
	wg.Wait()

	// The complaints are collected in chunk order and not in the order the lanes
	// came back in, so that two runs of the same broken section print the same
	// thing and a person can tell whether anything moved.
	answers := make([]string, len(j.chunks))
	models := make([]string, len(j.chunks))
	var problems []translate.Problem
	for i, c := range j.chunks {
		if a, ok := have[c.Index]; ok {
			answers[i], models[i] = a.Text, a.Model
			continue
		}
		bad, ok := stuck[c.Index]
		if !ok && ctx.Err() == nil {
			bad = []translate.Problem{{Rule: "queue", Msg: "it was never answered"}}
		}
		for _, p := range bad {
			p.Msg = fmt.Sprintf("chunk %d of %d: %s", c.Index, c.Of, p.Msg)
			problems = append(problems, p)
		}
	}
	if len(problems) > 0 {
		return "", "", problems
	}
	body := translate.Join(answers)
	// The join is audited against the whole English as well as chunk by chunk.
	// Every rule here is about order, and a chunk that is right on its own can
	// still land in the wrong place if a lane returned late, so this is what
	// says the section is a section.
	if ps := translate.Audit(lang, j.body, body); len(ps) > 0 {
		return "", "", ps
	}
	return body, modelsUsed(models), nil
}

// askChunk asks once, and asks again with the complaint if the first answer did
// not pass.
func askChunk(ctx context.Context, root string, host ocr.Host, g *glossary.Glossary, lang string, j job, c translate.Chunk, keep bool, deadline time.Duration, logf func(string, ...any)) (string, string, []translate.Problem) {
	terms := g.For(j.meta.Book)
	// A chunk with nothing in it to translate is not put to anybody. See
	// translate.SelfTranslation: what comes back has to be what went out, so the
	// ask can only cost time and lose entries, and chunk 30 of the historical
	// note of chapters I to IV lost nine of twenty every time it was tried.
	if translate.SelfTranslation(lang, c.Body) {
		logf("%s chunk %d of %d: nothing in it is translated, so it is copied", j.source, c.Index, c.Of)
		return c.Body, copiedModel, nil
	}
	// The bibliography is not asked for either. See translate.WithoutBiblio: the
	// entries stand as printed, so putting them in the question only asks a model
	// to copy thousands of characters of German and French titles letter for
	// letter, which is the one thing models are worst at and which no route
	// managed on chunk 30. What is asked for is the rest, and the entries go back
	// where they stood in the answer.
	body := translate.WithoutBiblio(c.Body)
	if body != c.Body {
		logf("%s chunk %d of %d: %d of its %d characters are bibliography and stand as printed, so they are not asked for",
			j.source, c.Index, c.Of, len(c.Body)-len(body), len(c.Body))
	}
	question, err := translateQuestion(terms, lang, body)
	if err != nil {
		return "", "", []translate.Problem{{Rule: "prompt", Msg: err.Error()}}
	}
	var last []translate.Problem
	for attempt := 1; attempt <= 2; attempt++ {
		ask := question
		if attempt == 2 {
			if ask, err = translateQuestionWithNote(terms, lang, body, retryNote(last)); err != nil {
				return "", "", []translate.Problem{{Rule: "prompt", Msg: err.Error()}}
			}
		}
		answer, err := ocr.NewAskWithin(host, fleet.SSH{Timeout: 2 * time.Minute}, ocr.Rsync{Timeout: 5 * time.Minute},
			ask, chunkID(lang, j.source, c, attempt), keep, deadline).Do(ctx)
		if err != nil {
			logf("%s chunk %d on %s: %v", j.source, c.Index, host.Name, err)
			last = []translate.Problem{{Rule: "transport", Msg: err.Error()}}
			if ctx.Err() != nil {
				// The first ask did not fail, it was stopped. Asking a second
				// time down a context that is already cancelled costs another
				// two minutes of ssh timeouts and cannot answer, and both of
				// those minutes are spent after the person has pressed Ctrl-C.
				break
			}
			continue
		}
		// Both halves are kept before either is judged, for the reason the
		// repair path keeps them: an accepted answer goes into a file, and
		// without the question there is afterwards no way to tell a model that
		// answered badly from one that was asked the wrong thing.
		if err := archiveChunk(root, lang, j.source, c, attempt, ask, answer.Text, answer.Conversation); err != nil {
			return "", "", []translate.Problem{{Rule: "archive", Msg: err.Error()}}
		}
		// The archive holds what the model wrote and the audit reads what the
		// repair leaves, which is the answer with the English layout back in any
		// formula the model re-spaced and nothing else moved. See
		// translate.Respace: a correct translation whose only fault is that it
		// wrote $M \cap N$ for $M\cap N$ is worth putting right rather than
		// asking again for five minutes to get it laid out some third way.
		answer.Text = translate.Respace(body, answer.Text)
		problems := translate.Audit(lang, body, answer.Text)
		// The terminology is asked here and not inside Audit, which compares
		// two texts and holds no glossary. It is the same test L10 makes of the
		// finished file, put while the run can still do something: a term left
		// in English is a complaint askChunk can hand back on the second ask,
		// and after the file is written it is a finding nobody acts on.
		problems = append(problems, translate.AuditTerms(lang, terms, body, answer.Text)...)
		// The entries go back before anything is written, and the whole chunk is
		// read once more with them in it. What was asked for and what is kept are
		// two different texts here, and the rules have to hold over the one that
		// is kept.
		if len(problems) == 0 && body != c.Body {
			whole, ok := translate.WithBiblio(c.Body, answer.Text)
			if !ok {
				problems = append(problems, translate.Problem{Rule: translate.RuleStructure,
					Msg: "the answer has no block for every block it was asked for, so the bibliography cannot go back around it"})
			} else {
				answer.Text = whole
				problems = append(problems, translate.Audit(lang, c.Body, whole)...)
			}
		}
		if len(problems) == 0 {
			// Said here rather than left to L08, which reads the file after it is
			// written. Nobody chooses the model, so this is not a refusal; it is
			// the difference between finding out in the first minute and finding
			// out after a chapter. An account was moved down between two runs of
			// the same section and the whole of the second one came back on the
			// small model, which nobody knew until the audit ran.
			note := ""
			if quality.SmallModel(answer.Model) {
				note = ", on " + answer.Model + ", which is a cut down model"
			}
			logf("%s chunk %d of %d on %s: accepted%s", j.source, c.Index, c.Of, host.Name, note)
			return answer.Text, answer.Model, nil
		}
		logf("%s chunk %d of %d on %s: %d problems on attempt %d", j.source, c.Index, c.Of, host.Name, len(problems), attempt)
		last = problems
	}
	return "", "", last
}

// transportOnly says the fleet never got an answer, as against getting one that
// was wrong.
//
// askChunk labels the two apart already: a question that did not reach a model,
// or reached one and came back as an error, is rule "transport", and everything
// else in this list is package translate saying what was wrong with the text. So
// the test is that every complaint is a transport one, and an empty list is not,
// since an empty list is a chunk that passed.
func transportOnly(bad []translate.Problem) bool {
	if len(bad) == 0 {
		return false
	}
	for _, p := range bad {
		if p.Rule != "transport" {
			return false
		}
	}
	return true
}

// retryNote is what the second ask adds.
//
// The complaints go over as they are, because they are already written for a
// person to read and a model reads them the same way. What is not sent is the
// first answer: asking a model to fix its own text invites it to keep the parts
// it likes, and the parts it likes are the ones this is complaining about.
//
// A few rules also carry a sentence saying what to do, and those are there
// because the complaint alone did not work. "an entry stands as printed" tells
// a reader everything and told the model nothing: chunk 3 of the historical
// note of chapter III came back four times with Vol. written Tap and and
// written va inside a numbered bibliography entry, on three different models,
// each time after being told what was wrong. Naming the words is what the
// answer needed.
//
// The sentences live here and not in the prompt on purpose. The prompt is
// hashed into every translated file, so a sentence added there marks all 240
// Vietnamese files stale at once and they would have to go over the fleet
// again; the note is built at ask time and changes nothing that is already
// written. That is the whole reason the retry note exists, and it is the right
// place for a rule that only the hard chunks need.
func retryNote(problems []translate.Problem) string {
	var b strings.Builder
	b.WriteString("Your previous answer to this section was thrown away. What was wrong with it:\n\n")
	for i, p := range problems {
		if i == 8 {
			b.WriteString(fmt.Sprintf("and %d more of the same kind\n", len(problems)-i))
			break
		}
		b.WriteString("  " + p.String() + "\n")
	}
	for _, line := range retryAdvice(problems) {
		b.WriteString("\n" + line + "\n")
	}
	b.WriteString("\nTranslate the section again from the beginning. Do not send the previous\n" +
		"answer back with a correction on top of it.\n")
	return b.String()
}

// retryAdvice is the extra sentence for each rule that has one, in rule order
// and once each however many times the rule fired.
func retryAdvice(problems []translate.Problem) []string {
	said := map[string]bool{}
	var out []string
	for _, p := range problems {
		if said[p.Rule] {
			continue
		}
		said[p.Rule] = true
		if line, ok := advice[p.Rule]; ok {
			out = append(out, line)
		}
	}
	return out
}

var advice = map[string]string{
	translate.RuleBibliography: "A numbered bibliography entry is copied out of the English character for\n" +
		"character. Nothing inside one is translated: not the title, not the name of\n" +
		"the journal, not the place, and not the words around them such as Vol., and,\n" +
		"ed., pp. or the abbreviation of a series. A reader follows a citation to a\n" +
		"library, and a citation in another language does not lead anywhere.",
	translate.RuleReference: "Every citation in the English is in the answer, spelled as the English\n" +
		"spells it: the page numbers, the volume numbers and the bracketed numbers of\n" +
		"the bibliography. Those are addresses and not words.",
	translate.RuleScript: "Write the answer in the language that was asked for and in no other. A\n" +
		"single character of another writing system is enough for the answer to be\n" +
		"thrown away.",
}

func translateQuestion(g *glossary.Glossary, lang, body string) (string, error) {
	return translateQuestionWithNote(g, lang, body, "")
}

func translateQuestionWithNote(g *glossary.Glossary, lang, body, note string) (string, error) {
	return prompt.Translate(lang, translate.GlossaryBlock(g, lang, body), note, body)
}

// modelsUsed is every model that answered a chunk of one section, in the order
// the chunks are in and without repeats.
//
// It used to be the first one that came back, which was wrong in the one case
// that matters. Nobody chooses the model: the ask goes to a browser profile and
// whatever the account is being served answers, and an account can be moved down
// in the middle of a section. Fifteen chunks over fifteen minutes is long enough
// for that to happen, and a file that names the model of chunk one is a file
// that says gpt-5-6 about a section half of which came back on the small one.
// L08 reads this field, so the field deciding to mention only the good half is
// the audit deciding not to look.
func modelsUsed(models []string) string {
	var out []string
	for _, m := range models {
		if m != "" && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return strings.Join(out, ", ")
}

// chunkID names the scratch directory on the host.
//
// The attempt is in it, so that a second ask does not land on the first one's
// files and read the first one's answer back when the second call dies before
// writing.
func chunkID(lang, source string, c translate.Chunk, attempt int) string {
	sum := sha256.Sum256([]byte(source))
	return fmt.Sprintf("tr-%s-%s-%03d-%d", lang, hex.EncodeToString(sum[:])[:6], c.Index, attempt)
}

// runID names the run in the front matter of every file it wrote.
//
// It is the language, the prompt and the list of sections, hashed. Two runs
// over the same sections with the same instructions carry the same id, which is
// what makes a re-run of a failed batch recognisable as the same piece of work
// rather than as a new one.
func runID(lang, promptHash string, jobs []job) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n", lang, promptHash)
	for _, j := range jobs {
		fmt.Fprintf(h, "%s\n", j.source)
	}
	return fmt.Sprintf("translate-%s-%s", lang, hex.EncodeToString(h.Sum(nil))[:8])
}

// archiveChunk keeps the question and the answer under work, which is not in
// the repository.
func archiveChunk(root, lang, source string, c translate.Chunk, attempt int, question, answer, conversation string) error {
	dir := filepath.Join(root, "work", "translate", lang, strings.ReplaceAll(source, "/", "_"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stem := filepath.Join(dir, fmt.Sprintf("%03d-%d", c.Index, attempt))
	if err := os.WriteFile(stem+".ask.md", []byte(question), 0o644); err != nil {
		return err
	}
	text := answer
	if conversation != "" {
		text = "<!-- " + conversation + " -->\n\n" + answer
	}
	return os.WriteFile(stem+".answer.md", []byte(text), 0o644)
}
