package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/gtranslate"
	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/translate"
)

// hardRules are the complaints a Google draft is refused for. Everything else
// the audit says, an English word left in, a term missed, is what a low
// quality draft is expected to have and what the model run that replaces it
// is for. These are not: they say the draft lost a formula, a heading, a
// citation or a block, and a page with a formula missing is worse than a page
// in English.
var hardRules = map[string]bool{
	translate.RuleMath:         true,
	translate.RuleTag:          true,
	translate.RuleStructure:    true,
	translate.RuleReference:    true,
	translate.RuleFrontMatter:  true,
	translate.RuleBibliography: true,
}

// ruleMathOrder is a math finding that only says the formulas moved.
const ruleMathOrder = "math order"

// reordered demotes the audit's math-in-order finding when the draft has
// exactly the source's formulas, each as many times. Unmask has already made
// sure of that for every formula Google was given, so what is left is a
// language that puts its clauses in another order: Chinese writes "for f in E,
// the map (f, x) ↦ f(x)" the other way round, and refusing it would refuse
// every page of Chinese.
func reordered(src, out string, problems []translate.Problem) []translate.Problem {
	if !sameSpans(src, out) {
		return problems
	}
	for i, p := range problems {
		if p.Rule == translate.RuleMath && strings.HasPrefix(p.Msg, "math span ") {
			problems[i].Rule = ruleMathOrder
		}
	}
	return problems
}

func sameSpans(a, b string) bool {
	x, u := mathtex.Split(a)
	y, v := mathtex.Split(b)
	if u != nil || v != nil || len(x) != len(y) {
		return false
	}
	count := map[string]int{}
	for _, s := range x {
		count[fmt.Sprint(s.Display, s.Text)]++
	}
	for _, s := range y {
		k := fmt.Sprint(s.Display, s.Text)
		if count[k] == 0 {
			return false
		}
		count[k]--
	}
	return true
}

func hard(problems []translate.Problem) []translate.Problem {
	var out []translate.Problem
	for _, p := range problems {
		if hardRules[p.Rule] {
			out = append(out, p)
		}
	}
	return out
}

// runGoogle is bourbaki translate -engine google: the same files a model run
// would pick, drafted by Google Translate and held to the same audit.
func runGoogle(root string, g *glossary.Glossary, from, lang, tree, book, chapter, only string, limit int, force, stale bool, within time.Duration) error {
	source := sourceLang(from)
	promptHash := gtranslate.PromptSHA256(source, lang)
	all, current, err := translateJobs(root, g, from, lang, tree, book, chapter, only, promptHash, force, false)
	if err != nil {
		return err
	}
	var jobs []job
	kept, printed := 0, 0
	for _, j := range all {
		// A chapter Springer printed in English is read from content/en and
		// never from the French, so the French copy of it is not a job.
		if from == "fr" && hasEnglishPrinting(root, j) {
			printed++
			continue
		}
		// A model's translation, current or stale, is better than a draft, so
		// a draft never goes over one.
		if !force && hasModelTranslation(root, tree, j) {
			kept++
			continue
		}
		jobs = append(jobs, j)
	}
	fmt.Printf("translate: %d files to draft in %s with Google, %d current, %d kept for their model translation",
		len(jobs), lang, current, kept)
	if printed > 0 {
		fmt.Printf(", %d have an English printing", printed)
	}
	fmt.Println()
	if stale {
		for _, j := range jobs {
			fmt.Printf("%-66s  %s\n", j.source, j.why)
		}
		return nil
	}
	if limit > 0 && len(jobs) > limit {
		fmt.Printf("translate: doing the first %d\n", limit)
		jobs = jobs[:limit]
	}
	if len(jobs) == 0 {
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if within > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, within)
		defer cancel()
	}
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}
	work := filepath.Join(filepath.Dir(defaultQueueRoot()), "gtranslate")
	client := gtranslate.New(work)
	run := runID(tree, promptHash, jobs)
	written, refused, soft := 0, 0, 0
	for _, j := range jobs {
		gl := g.For(j.meta.Book)
		d := gtranslate.Drafter{Client: client, Glossary: gl, Source: source, Target: lang}
		body, problems, err := draftFile(ctx, d, source, lang, j)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			refused++
			logf("%s: refused, %v", j.source, err)
			continue
		}
		if bad := hard(problems); len(bad) > 0 {
			refused++
			// The refused draft is kept beside the cache, where it can be read
			// against the source and never reaches the corpus or the site.
			kept := filepath.Join(work, "refused", tree, strings.TrimPrefix(j.source, "content/"))
			if os.MkdirAll(filepath.Dir(kept), 0o755) == nil {
				_ = os.WriteFile(kept, []byte(body), 0o644)
			}
			logf("%s: refused, nothing written; the draft is in %s", j.source, kept)
			for _, p := range bad {
				logf("\t%s", p)
			}
			continue
		}
		path, err := writeTranslation(root, from, lang, tree, run, promptHash, g.Version, j, body, corpus.GoogleModel)
		if err != nil {
			return err
		}
		written++
		soft += len(problems)
		note := ""
		if len(problems) > 0 {
			note = fmt.Sprintf(", %d soft findings left for the model run", len(problems))
		}
		logf("%s: drafted, %d chunks%s", rel(root, path), len(j.chunks), note)
	}
	fmt.Printf("translate: %d drafts written, %d refused, %d soft findings, %d requests to Google\n",
		written, refused, soft, client.Requests)
	if within > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Printf("translate: stopped after %s, which is what -within asked for; run it again to carry on\n", within)
		return nil
	}
	return ctx.Err()
}

// draftFile drafts every chunk of a file and audits the join, the way a model
// run does.
func draftFile(ctx context.Context, d gtranslate.Drafter, source, lang string, j job) (string, []translate.Problem, error) {
	var answers []string
	left := 0
	for _, c := range j.chunks {
		out, n, problems, err := draftChunk(ctx, d, source, lang, c.Body)
		left += n
		if err != nil {
			return "", nil, fmt.Errorf("chunk %d of %d: %w", c.Index, c.Of, err)
		}
		// One chunk with a hard complaint is asked again a block at a time,
		// which is the cut Google is least likely to lose a piece of.
		if len(hard(problems)) > 0 {
			var parts []string
			problems = nil
			left -= n
			for _, block := range strings.Split(c.Body, "\n\n") {
				got, n, p, err := draftChunk(ctx, d, source, lang, block)
				left += n
				if err != nil {
					return "", nil, fmt.Errorf("chunk %d of %d, block by block: %w", c.Index, c.Of, err)
				}
				parts = append(parts, got)
				problems = append(problems, p...)
			}
			out = strings.Join(parts, "\n\n")
		}
		if bad := hard(problems); len(bad) > 0 {
			return out, bad, nil
		}
		answers = append(answers, out)
	}
	// A few sentences left in the source are a draft with holes in it; many
	// are a page in the wrong language under a flag saying it is translated.
	if blocks := len(strings.Split(j.body, "\n\n")); left > 3 && left*10 > blocks {
		return "", nil, fmt.Errorf("google could not keep the formulas of %d sentences whole, too many to leave in the source", left)
	}
	whole := translate.Join(answers)
	problems := reordered(j.body, whole, translate.Audit(lang, j.body, whole))
	if source == "en" {
		problems = append(problems, translate.AuditTerms(lang, d.Glossary, j.body, whole)...)
	}
	return whole, problems, nil
}

// draftChunk is askChunk for Google: the same repairs, the same audit.
func draftChunk(ctx context.Context, d gtranslate.Drafter, source, lang, chunk string) (string, int, []translate.Problem, error) {
	if translate.SelfTranslation(lang, chunk) {
		return chunk, 0, nil, nil
	}
	body := translate.WithoutBiblio(chunk)
	out, left, err := d.Draft(ctx, body)
	if err != nil {
		return "", 0, nil, err
	}
	out = translate.Unescape(body, out)
	out = translate.Redollar(body, out)
	out = translate.Respace(body, out)
	out = translate.Readdress(lang, body, out)
	problems := reordered(body, out, translate.Audit(lang, body, out))
	if source == "en" {
		problems = append(problems, translate.AuditTerms(lang, d.Glossary, body, out)...)
	}
	if body != chunk {
		whole, ok := translate.WithBiblio(chunk, out)
		if !ok {
			problems = append(problems, translate.Problem{Rule: translate.RuleStructure,
				Msg: "the draft has no block for every block it was given, so the bibliography cannot go back"})
		} else {
			out = whole
		}
	}
	return out, left, problems, nil
}

// hasEnglishPrinting says the chapter a French file belongs to is printed in
// English, which is where its English comes from.
func hasEnglishPrinting(root string, j job) bool {
	dir := filepath.Join(root, "content", "en", j.meta.Book)
	if j.meta.Chapter != "" {
		dir = filepath.Join(dir, j.meta.Chapter)
	}
	_, err := os.Stat(dir)
	return err == nil
}

// hasModelTranslation says the file this job would write is there and was not
// written by Google.
func hasModelTranslation(root, tree string, j job) bool {
	if j.ex != nil {
		meta := *j.ex
		meta.Lang = tree
		f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](corpus.ExercisePath(root, tree, meta))
		return err == nil && f.Meta.TranslationQuality != corpus.QualityLow
	}
	meta := j.meta
	meta.Lang = tree
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](corpus.SectionPath(root, tree, meta))
	return err == nil && f.Meta.TranslationQuality != corpus.QualityLow
}
