package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

Translates English section files into Vietnamese, Chinese or Japanese, and
refuses any answer that is not the same section.

  -lang CODE     vi, zh or ja, required
  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -book ID       only this book, as books.yaml names it
  -chapter ID    only this chapter, as VIII
  -file PATH     only this English file, relative to the corpus root
  -limit N       stop after this many sections
  -force         translate again even where a translation is already there and
                 not stale
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -dry           print the first question and stop, without asking anything
  -stale         list what needs translating and why, and ask nothing
  -keep          leave the questions on the boxes, for debugging

A section is skipped when its translation is there, its source_content_sha256
is the English file's content_sha256, its glossary_terms_sha256 is the digest of
the glossary rows its English mentions today, and its prompt_sha256 is this
binary's. Any of those four out of date is what stale means, and a stale file is
translated again.

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

Every question and every answer is kept under work/translate, which is not in
the repository. What goes in the repository is the file, and the file is only
written when the audit found nothing.
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
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	dry := fs.Bool("dry", false, "print the first question and stop")
	stale := fs.Bool("stale", false, "list what needs translating and why, and ask nothing")
	keep := fs.Bool("keep", false, "leave the questions on the boxes")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if !known(*lang) {
		fs.Usage()
		os.Exit(2)
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
	promptHash, err := prompt.TranslateSHA256(*lang)
	if err != nil {
		return err
	}
	jobs, skipped, err := translateJobs(root, g, *lang, *book, *chapter, *file, promptHash, *force)
	if err != nil {
		return err
	}
	if *stale {
		return reportStale(jobs, skipped, *lang)
	}
	if len(jobs) == 0 {
		fmt.Printf("translate: nothing to do, %d sections are already translated and current\n", skipped)
		return nil
	}
	if *limit > 0 && len(jobs) > *limit {
		// Said out loud, because a run that quietly stopped at the limit reads
		// exactly like a run that covered everything.
		fmt.Printf("translate: %d sections need %s, doing the first %d\n", len(jobs), *lang, *limit)
		jobs = jobs[:*limit]
	} else {
		fmt.Printf("translate: %d sections need %s, %d are current\n", len(jobs), *lang, skipped)
	}
	if *dry {
		question, err := translateQuestion(g, *lang, jobs[0].chunks[0].Body)
		if err != nil {
			return err
		}
		fmt.Printf("\n%s: chunk 1 of %d\n\n%s\n", jobs[0].source, len(jobs[0].chunks), question)
		return nil
	}
	hosts, err := ocrHosts(*routeFile, *hostList)
	if err != nil {
		return err
	}
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run := runID(*lang, promptHash, jobs)
	var written, refused int
	for _, job := range jobs {
		body, model, problems := translateSection(ctx, root, hosts, g, *lang, job, *keep, logf)
		if len(problems) > 0 {
			refused++
			logf("%s: refused, nothing written", job.source)
			for _, p := range problems {
				logf("\t%s", p)
			}
			continue
		}
		out := job.meta
		out.Lang = *lang
		out.TranslatedFrom = job.source
		out.SourceSHA256 = job.meta.ContentSHA256
		out.ContentSHA256 = corpus.ContentSHA256(body)
		out.TranslationModel = model
		out.TranslationRun = run
		out.GlossaryVersion = g.Version
		out.GlossaryTerms = job.terms
		out.PromptSHA256 = promptHash
		path := corpus.SectionPath(root, *lang, out)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := (corpus.SectionFile{Meta: out, Body: body}).Write(path); err != nil {
			return err
		}
		written++
		logf("%s: written, %d chunks", rel(root, path), len(job.chunks))
		if ctx.Err() != nil {
			break
		}
	}
	fmt.Printf("translate: %d sections written, %d refused, %d were already current\n", written, refused, skipped)
	return ctx.Err()
}

// reportStale prints what a run would do, and asks nothing.
//
// The reason is printed beside every file, because "stale" on its own is not
// actionable: the English changing means a section was re-extracted, the
// instructions changing means every file is going again, and the terminology
// changing means one glossary row reached this section. Those are three
// different sizes of job and the count on its own hides which one this is.
func reportStale(jobs []job, skipped int, lang string) error {
	for _, j := range jobs {
		fmt.Printf("%-64s %s\n", j.source, j.why)
	}
	fmt.Printf("translate: %d sections need %s, %d are current\n", len(jobs), lang, skipped)
	return nil
}

// A job is one English section and the chunks it was cut into.
type job struct {
	source string // relative to the corpus root
	meta   corpus.SectionFrontMatter
	body   string
	chunks []translate.Chunk
	terms  string // digest of the glossary rows this section's English mentions
	why    string // why the translation on disk is not the one this run would make
}

// translateJobs is the English that needs this language.
//
// The walk is over content/en rather than over the sections manifest, because
// the manifest records what the assembler produced and this has to translate
// what is on disk. The two agree today and the file is the thing being read.
func translateJobs(root string, g *glossary.Glossary, lang, book, chapter, only, promptHash string, force bool) ([]job, int, error) {
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
		f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
		if err != nil {
			// An exercise file or anything else that is not a section. The
			// exercises are a batch of their own and are not this command's.
			continue
		}
		if f.Meta.Lang != "en" || f.Meta.Book == "" {
			continue
		}
		source := rel(root, path)
		switch {
		case only != "" && source != only:
			continue
		case book != "" && f.Meta.Book != book:
			continue
		case chapter != "" && !strings.EqualFold(f.Meta.Chapter, chapter):
			continue
		}
		terms := translate.GlossaryDigest(g, lang, f.Body)
		ok, why := current(root, lang, source, f.Meta, g.Version, promptHash, terms)
		if !force && ok {
			skipped++
			continue
		}
		chunks := translate.Chunks(f.Body)
		if len(chunks) == 0 {
			continue
		}
		jobs = append(jobs, job{source: source, meta: f.Meta, body: f.Body, chunks: chunks, terms: terms, why: why})
	}
	return jobs, skipped, nil
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

// translateSection asks for every chunk and puts the answers together.
//
// A chunk that fails after its second ask fails the section. There is no
// partial write: half a translated § is worse than none, because the audit
// counts it as a translated file and a reader has no way to see where the
// English stopped being carried over.
func translateSection(ctx context.Context, root string, hosts []ocr.Host, g *glossary.Glossary, lang string, j job, keep bool, logf func(string, ...any)) (string, string, []translate.Problem) {
	answers := make([]string, len(j.chunks))
	models := make([]string, len(j.chunks))
	bad := make([][]translate.Problem, len(j.chunks))

	next := make(chan int)
	go func() {
		defer close(next)
		for i := range j.chunks {
			select {
			case <-ctx.Done():
				return
			case next <- i:
			}
		}
	}()
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, host := range hosts {
		for lane := 0; lane < host.Lanes; lane++ {
			wg.Add(1)
			go func(host ocr.Host) {
				defer wg.Done()
				for i := range next {
					text, model, problems := askChunk(ctx, root, host, g, lang, j, j.chunks[i], keep, logf)
					mu.Lock()
					answers[i], models[i], bad[i] = text, model, problems
					mu.Unlock()
				}
			}(host)
		}
	}
	wg.Wait()

	var problems []translate.Problem
	for i, ps := range bad {
		for _, p := range ps {
			p.Msg = fmt.Sprintf("chunk %d of %d: %s", i+1, len(j.chunks), p.Msg)
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
func askChunk(ctx context.Context, root string, host ocr.Host, g *glossary.Glossary, lang string, j job, c translate.Chunk, keep bool, logf func(string, ...any)) (string, string, []translate.Problem) {
	question, err := translateQuestion(g, lang, c.Body)
	if err != nil {
		return "", "", []translate.Problem{{Rule: "prompt", Msg: err.Error()}}
	}
	var last []translate.Problem
	for attempt := 1; attempt <= 2; attempt++ {
		ask := question
		if attempt == 2 {
			if ask, err = translateQuestionWithNote(g, lang, c.Body, retryNote(last)); err != nil {
				return "", "", []translate.Problem{{Rule: "prompt", Msg: err.Error()}}
			}
		}
		answer, err := (ocr.Ask{
			Host: host, Shell: fleet.SSH{Timeout: 2 * time.Minute}, Copy: ocr.Rsync{Timeout: 5 * time.Minute},
			Prompt: ask, ID: chunkID(lang, j.source, c, attempt), Keep: keep,
		}).Do(ctx)
		if err != nil {
			logf("%s chunk %d on %s: %v", j.source, c.Index, host.Name, err)
			last = []translate.Problem{{Rule: "transport", Msg: err.Error()}}
			continue
		}
		// Both halves are kept before either is judged, for the reason the
		// repair path keeps them: an accepted answer goes into a file, and
		// without the question there is afterwards no way to tell a model that
		// answered badly from one that was asked the wrong thing.
		if err := archiveChunk(root, lang, j.source, c, attempt, ask, answer.Text, answer.Conversation); err != nil {
			return "", "", []translate.Problem{{Rule: "archive", Msg: err.Error()}}
		}
		problems := translate.Audit(lang, c.Body, answer.Text)
		if len(problems) == 0 {
			logf("%s chunk %d of %d on %s: accepted", j.source, c.Index, c.Of, host.Name)
			return answer.Text, answer.Model, nil
		}
		logf("%s chunk %d of %d on %s: %d problems on attempt %d", j.source, c.Index, c.Of, host.Name, len(problems), attempt)
		last = problems
	}
	return "", "", last
}

// retryNote is what the second ask adds.
//
// The complaints go over as they are, because they are already written for a
// person to read and a model reads them the same way. What is not sent is the
// first answer: asking a model to fix its own text invites it to keep the parts
// it likes, and the parts it likes are the ones this is complaining about.
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
	b.WriteString("\nTranslate the section again from the beginning. Do not send the previous\n" +
		"answer back with a correction on top of it.\n")
	return b.String()
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
