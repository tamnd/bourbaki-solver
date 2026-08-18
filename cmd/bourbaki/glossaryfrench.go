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
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/translate"
)

// Reading the French out of the book instead of asking for it.
//
// Most of the series exists twice over. Algebra, General Topology, Functions of
// a Real Variable, Topological Vector Spaces, Integration, Lie and the
// historical note were all printed in both languages, chapter for chapter and
// section for section, and the English is a translation of the French sitting
// right there in the corpus. Asking a model what the French for a term is throws
// that away and takes its word for something the shelf already answers.
//
// So this pairs a section with its French printing, block by block, and asks
// which words of the French paragraph are the term the English paragraph uses.
// An answer that is not in the paragraph is refused, which is a check nothing in
// the unaligned pass can make. What is left over after this, the terms that
// appear only in the volumes that were never translated, is the small part worth
// asking a model about.

const glossaryFrenchUsage = `usage: bourbaki glossary french [flags]

Fills the French column by reading the French printing, for every term the two
printings share a paragraph about.

The English of a section and the French of the same section are paired block by
block, and each question carries one English paragraph, the French paragraph
beside it and the terms the English one mentions. A rendering that is not in the
French paragraph is refused, so nothing here is a model's opinion about French.

  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -book ID       only this book, by the short id: alg, lie, ens
  -chapter N     only this chapter
  -batch N       terms per question, default 12. Smaller than the unaligned
                 pass, because a question here carries two paragraphs as well
                 as the terms
  -limit N       stop after this many questions
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -dry           print the first question and stop, without asking anything
  -keep          leave the questions on the boxes
`

func glossaryFrench(args []string) error {
	fs := flag.NewFlagSet("glossary french", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, glossaryFrenchUsage) }
	dir := fs.String("corpus", "", "the checkout")
	book := fs.String("book", "", "only this book")
	chapter := fs.String("chapter", "", "only this chapter")
	size := fs.Int("batch", 12, "terms per question")
	limit := fs.Int("limit", 0, "stop after this many questions")
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	dry := fs.Bool("dry", false, "print the first question and stop")
	keep := fs.Bool("keep", false, "leave the questions on the boxes")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpusRoot(*dir)
	if err != nil {
		return err
	}
	g, err := glossary.Load(root)
	if err != nil {
		return err
	}
	pairs, err := alignedPairs(root, *book, *chapter)
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		fmt.Println("glossary french: no section is in the corpus in both printings yet")
		return nil
	}
	batches := alignedBatches(g, pairs, *size)
	if len(batches) == 0 {
		fmt.Printf("glossary french: %d sections are in both printings, and every term they mention already has a French\n", len(pairs))
		return nil
	}
	if *limit > 0 && len(batches) > *limit {
		batches = batches[:*limit]
		fmt.Printf("glossary french: %d sections in both printings, asking the first %d questions\n", len(pairs), *limit)
	} else {
		fmt.Printf("glossary french: %d sections in both printings, in %d questions\n", len(pairs), len(batches))
	}
	if *dry {
		fmt.Printf("\n%s\n", batches[0].Prompt())
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

	var rows []glossary.Row
	var rejects int
	unknown := map[string]bool{}
	for i, reply := range askEveryBatch(ctx, root, hosts, batches, *keep, logf) {
		if reply == nil {
			continue
		}
		rows = append(rows, reply.Rows...)
		rows = append(rows, reply.Suspect...)
		rejects += len(reply.Rejects)
		for _, t := range reply.Unknown {
			unknown[t] = true
		}
		for _, r := range reply.Rejects {
			logf("question %d: %s: %s", i+1, name(r.EN), r.Reason)
		}
	}
	added, kept := g.Merge("fr", rows)
	if err := g.Validate(); err != nil {
		return fmt.Errorf("the merge would leave the glossary invalid, nothing was written: %w", err)
	}
	version, bumped := g.Version, false
	if len(g.Terms) > 0 {
		if version, bumped, err = g.Save(glossary.Path(root)); err != nil {
			return err
		}
	}
	fmt.Printf("glossary french: %d read out of the French, %d already had one, %d refused, %d the French paragraph does not say\n",
		added, kept, rejects, len(unknown))
	fmt.Printf("\t%s now holds %d terms, %d with fr\n", rel(root, glossary.Path(root)), len(g.Terms), len(g.In("fr")))
	if bumped {
		fmt.Printf("\tversion %d\n", version)
	}
	return nil
}

// A pair is one section in both printings.
type pair struct {
	en, fr string // the bodies
	source string // the English file, for a log line
}

// alignedPairs finds every section the corpus holds in English and in French.
//
// The file names are not the same in the two trees, because a French section is
// named after its French title, so the pairing is on the number the assembler
// puts in front of both: 01_s1_simple_modules.md and 01_s1_modules_simples.md
// are the same section and their names agree about the only part that is not a
// title. Exercises are named by number in both trees and pair by their path.
func alignedPairs(root, book, chapter string) ([]pair, error) {
	fr := map[string]string{}
	err := walkContent(filepath.Join(root, "content", "fr"), func(path, key string) {
		fr[key] = path
	})
	if err != nil {
		return nil, err
	}
	var out []pair
	err = walkContent(filepath.Join(root, "content", "en"), func(path, key string) {
		other, ok := fr[key]
		if !ok {
			return
		}
		if book != "" && !strings.HasPrefix(key, book+"/") {
			return
		}
		if chapter != "" && !strings.Contains(key, "/"+chapter+"/") {
			return
		}
		a, err := os.ReadFile(path)
		if err != nil {
			return
		}
		b, err := os.ReadFile(other)
		if err != nil {
			return
		}
		out = append(out, pair{en: body(string(a)), fr: body(string(b)), source: rel(root, path)})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].source < out[j].source })
	return out, nil
}

// walkContent calls back for every markdown file under a language tree, with
// the key that identifies the same section in the other one.
func walkContent(dir string, fn func(path, key string)) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rest, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rest), "/")
		if len(parts) < 3 {
			return nil
		}
		// Everything but the file name identifies the section in both trees.
		// The file name does not, because it carries the title, so what is kept
		// of it is the numbering the assembler wrote in front of the title.
		key := strings.Join(parts[:len(parts)-1], "/") + "/" + sectionKey(parts[len(parts)-1])
		fn(path, key)
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// sectionKey is the part of a file name that is the same in both printings:
// 01_s1 out of 01_s1_modules_simples.md, and the whole name of an exercise,
// which is a number.
func sectionKey(name string) string {
	name = strings.TrimSuffix(name, ".md")
	parts := strings.Split(name, "_")
	if len(parts) >= 2 && strings.HasPrefix(parts[1], "s") {
		return parts[0] + "_" + parts[1]
	}
	if len(parts) >= 1 && parts[0] == "00" {
		return "00"
	}
	return name
}

// body is the markdown under the front matter.
func body(text string) string {
	const fence = "---\n"
	if !strings.HasPrefix(text, fence) {
		return text
	}
	if i := strings.Index(text[len(fence):], "\n"+fence); i >= 0 {
		return strings.TrimSpace(text[len(fence)+i+len(fence)+1:])
	}
	return text
}

// alignedBatches turns the paired sections into questions.
//
// One question per English paragraph that mentions terms with no French yet,
// carrying the French paragraph beside it. A term is asked about once however
// many paragraphs mention it: the second answer would be the same word out of a
// different sentence, and the questions are the expensive part.
func alignedBatches(g *glossary.Glossary, pairs []pair, size int) []glossary.Batch {
	if size <= 0 {
		size = 12
	}
	need := map[string]bool{}
	for _, t := range g.Terms {
		if t.FR == "" && t.EN != "" {
			need[strings.ToLower(t.EN)] = true
		}
	}
	var cand []mention
	for _, p := range pairs {
		en, fr := translate.Blocks(p.en), translate.Blocks(p.fr)
		at := align(en, fr)
		for i, block := range en {
			lower := strings.ToLower(block)
			var terms []string
			for _, t := range g.Terms {
				if !need[strings.ToLower(t.EN)] || !glossary.Mentions(lower, glossary.Key(t.EN)) {
					continue
				}
				terms = append(terms, t.EN)
			}
			if len(terms) == 0 || at[i] < 0 {
				continue
			}
			cand = append(cand, mention{en: block, fr: window(fr, at[i]), terms: terms})
		}
	}
	// The richest paragraph first. A term is asked about once and most of them
	// are said in many paragraphs, so taking the paragraphs in the order they
	// are printed spends a whole question on one leftover term while a paragraph
	// further on would have carried a dozen. Ordering by how many terms are
	// still open puts the same work in a fraction of the questions.
	sort.SliceStable(cand, func(i, j int) bool { return len(cand[i].terms) > len(cand[j].terms) })
	var out []glossary.Batch
	asked := map[string]bool{}
	for _, c := range cand {
		var terms []string
		for _, term := range c.terms {
			key := strings.ToLower(term)
			if asked[key] {
				continue
			}
			terms = append(terms, term)
			asked[key] = true
			if len(terms) == size {
				break
			}
		}
		if len(terms) == 0 {
			continue
		}
		out = append(out, glossary.Batch{Lang: "fr", Terms: terms, EN: c.en, FR: c.fr})
	}
	return out
}

// mention is a paragraph in both printings and every open term it mentions,
// before the terms are handed out to one question each.
type mention struct {
	en, fr string
	terms  []string
}

// align says, for each English paragraph, which French paragraph is the same
// one, and -1 where it cannot tell.
//
// Counting from the top does not work. A footnote, a display or a paragraph the
// other printing runs together moves the count, and the count stays moved:
// across the 339 sections the corpus holds twice, 104 end with the two block
// counts apart and the worst is off by fifteen. A question built by position out
// of the far end of one of those carries the wrong French paragraph, and every
// answer read out of it is refused for not being in a passage it was never in.
//
// So the mathematics is the anchor. It is the one part of a paragraph the
// translation leaves alone, $\mathfrak{g}$ is $\mathfrak{g}$ in both books, and
// a paragraph with formulas in it is found by them. The search is a band around
// where the paragraph would be if nothing had slipped, wide enough for the drift
// the two counts admit. A paragraph whose match is not clear enough is dropped
// rather than guessed at: there are more paragraphs than terms, and one that is
// certain is always there to ask instead.
func align(en, fr []string) []int {
	out := make([]int, len(en))
	if len(fr) == 0 {
		for i := range out {
			out[i] = -1
		}
		return out
	}
	drift := len(en) - len(fr)
	if drift < 0 {
		drift = -drift
	}
	band := drift + 3
	taken := map[int]bool{}
	for i, block := range en {
		// Where the two counts agree, or are one apart, counting is sound: the
		// three paragraph window covers the whole of the slip there is room for.
		// It is the long sections that have drifted away that need an anchor.
		out[i] = -1
		if drift <= 1 && i < len(fr) {
			out[i] = i
		}
		want := formulas(block)
		if len(want) == 0 {
			continue
		}
		at := i * len(fr) / len(en)
		best, score := -1, 0
		for j := at - band; j <= at+band; j++ {
			if j < 0 || j >= len(fr) || taken[j] {
				continue
			}
			if n := shared(want, formulas(fr[j])); n > score {
				best, score = j, n
			}
		}
		// Two formulas in common, or one that is not a bare letter. A single
		// $n$ is in half the paragraphs of the book and matches nothing.
		if best >= 0 && (score > 1 || len(want) == 1 && len(want[0]) > 4) {
			out[i] = best
			taken[best] = true
		}
	}
	return out
}

// formulas is the mathematics of a paragraph, in the order it is written.
func formulas(block string) []string {
	var out []string
	for rest := block; ; {
		i := strings.Index(rest, "$")
		if i < 0 {
			return out
		}
		rest = rest[i+1:]
		j := strings.Index(rest, "$")
		if j < 0 {
			return out
		}
		if f := strings.Join(strings.Fields(rest[:j]), " "); f != "" {
			out = append(out, f)
		}
		rest = rest[j+1:]
	}
}

// shared counts the formulas two paragraphs have in common, each once.
func shared(a, b []string) int {
	have := map[string]int{}
	for _, f := range b {
		have[f]++
	}
	n := 0
	for _, f := range a {
		if have[f] > 0 {
			have[f]--
			n++
		}
	}
	return n
}

// window is the French around the paragraph in the English's place.
//
// Not the one paragraph. The two printings are the same text and they are not
// the same blocks: a footnote, a display or a run-together paragraph moves the
// count by one and everything after it is off by that one. Three paragraphs
// carry the sentence whichever way the count slipped, and cost a few hundred
// characters of prompt.
func window(fr []string, i int) string {
	if len(fr) == 0 {
		return ""
	}
	lo, hi := i-1, i+2
	if lo < 0 {
		lo = 0
	}
	if hi > len(fr) {
		hi = len(fr)
	}
	if lo >= hi {
		lo, hi = len(fr)-1, len(fr)
	}
	return strings.Join(fr[lo:hi], "\n\n")
}
