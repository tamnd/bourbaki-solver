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

	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/ocr"
)

// Asking the fleet for terminology.
//
// This is the first thing in the project that sends prose rather than a page
// image, and it is deliberately the smallest such thing: forty terms out, forty
// lines back, every line checked before it is written. Whatever this run learns
// about how a browser answers a text question is learned forty terms at a time
// rather than a chapter at a time.
//
// Nothing here decides whether a rendering is right. That is the glossary
// package, which refuses a line that does not repeat its own English, that gave
// two renderings, that wrote a sentence, or that has none of the language in it.
// This file is the wiring: pick the terms, cut them into batches, put each batch
// on a host, and write what survived.

const glossaryTranslateUsage = `usage: bourbaki glossary translate -lang CODE [flags]

Asks the fleet for the rendering of every term that has none in this language,
audits each line, and merges what survives into manifests/glossary.yaml.

  -lang CODE     vi, zh or ja, required
  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -add N         also ask about the N commonest mined candidates that are not
                 in the glossary yet, from manifests/glossary-candidates.yaml.
                 This is how the file gets its first rows.
  -min N         when adding, only candidates the corpus uses at least N times
  -only SOURCE   when adding, only candidates this source found: title,
                 defined, prose or kind. The candidates are ordered by how
                 often the prose says them, so a volume whose pages are not
                 read yet sits at the bottom of the file however good its
                 terms are: Theory of Sets is mined from its table of
                 contents and every row of it is a title with a count of one
                 or two. -only title is how those are reached without
                 sending the four hundred prose phrases above them.
  -batch N       terms per question, default 40
  -limit N       stop after this many questions
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -dry           print the first question and stop, without asking anything
  -keep          leave the questions on the boxes, for debugging

A rendering already in the glossary is never overwritten. Whatever is there was
either written by a person or accepted by an earlier run, and a later run
changing it would make the file depend on the order the batches finished in.

Every question and every answer is kept under work/glossary, which is not in
the repository: the answer is the model's words until it has been audited, and
after that what matters is the glossary.

The model is told it may answer UNKNOWN, and that doing so is the wanted answer
for a term it cannot render. That is worth more than any check here. A model
with no way out invents, and an invented rendering is the one failure that
passes every mechanical test: one phrase, in the right script, that no
mathematician uses. Those terms are listed at the end for a person to write.
`

func glossaryTranslate(args []string) error {
	fs := flag.NewFlagSet("glossary translate", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, glossaryTranslateUsage) }
	dir := fs.String("corpus", "", "the checkout")
	lang := fs.String("lang", "", "vi, zh or ja")
	add := fs.Int("add", 0, "also ask about this many mined candidates")
	min := fs.Int("min", 0, "when adding, only candidates used at least this often")
	only := fs.String("only", "", "when adding, only candidates from this source")
	size := fs.Int("batch", glossary.DefaultBatch, "terms per question")
	limit := fs.Int("limit", 0, "stop after this many questions")
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	dry := fs.Bool("dry", false, "print the first question and stop")
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
	terms, err := needing(root, g, *lang, *add, *min, *only)
	if err != nil {
		return err
	}
	if len(terms) == 0 {
		fmt.Printf("glossary translate: every term already has a %s rendering, and -add would bring in none\n", *lang)
		return nil
	}
	batches := glossary.Batches(*lang, terms, *size)
	if *limit > 0 && len(batches) > *limit {
		batches = batches[:*limit]
		// Said out loud. A run that quietly stopped at the limit reads exactly
		// like a run that covered everything.
		fmt.Printf("glossary translate: %d terms need %s, asking about the first %d\n",
			len(terms), *lang, *limit**size)
	} else {
		fmt.Printf("glossary translate: %d terms need %s, in %d questions\n", len(terms), *lang, len(batches))
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

	replies := askEveryBatch(ctx, root, hosts, batches, *keep, logf)

	var rows, suspect []glossary.Row
	var unknown []string
	var rejects, collisions int
	for i, reply := range replies {
		if reply == nil {
			continue
		}
		rows = append(rows, reply.Rows...)
		suspect = append(suspect, reply.Suspect...)
		unknown = append(unknown, reply.Unknown...)
		rejects += len(reply.Rejects)
		collisions += len(reply.Collisions)
		for _, r := range reply.Rejects {
			logf("question %d: %s: %s", i+1, name(r.EN), r.Reason)
		}
		for _, c := range reply.Collisions {
			logf("question %d: %q answers %s, which may be right and may be two notions run together",
				i+1, c.TR, strings.Join(c.EN, ", "))
		}
	}

	added, kept := g.Merge(*lang, rows)
	if err := g.Validate(); err != nil {
		return fmt.Errorf("the merge would leave the glossary invalid, nothing was written: %w", err)
	}
	// A run where every question failed writes nothing. It would otherwise put
	// an empty glossary.yaml in the corpus, which is a file somebody has to
	// notice and delete, and which reads as a glossary that exists.
	version, bumped := g.Version, false
	if len(g.Terms) > 0 {
		var err error
		if version, bumped, err = g.Save(glossary.Path(root)); err != nil {
			return err
		}
	}

	fmt.Printf("glossary translate: %d accepted, %d already had one, %d refused, %d unknown, %d flagged, %d to look at\n",
		added, kept, rejects, len(unknown), collisions, len(suspect))
	fmt.Printf("\t%s now holds %d terms, %d with %s\n",
		rel(root, glossary.Path(root)), len(g.Terms), len(g.In(*lang)), *lang)
	if bumped {
		fmt.Printf("\tversion %d, so every translated file is stale and bourbaki translate will do it again\n", version)
	}
	if len(suspect) > 0 {
		fmt.Printf("\nthese %d were accepted and are worth an eye, because nothing here can tell\na diacritic-free Vietnamese word from an English one left standing:\n", len(suspect))
		for _, s := range suspect {
			fmt.Printf("\t%-30s %s\n", s.EN, s.TR)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		fmt.Printf("\nthe model said it does not know these %d, they have to be written by hand:\n", len(unknown))
		for _, u := range unknown {
			fmt.Printf("\t%s\n", u)
		}
	}
	return ctx.Err()
}

// askEveryBatch puts the questions on the hosts, one per lane at a time.
//
// A nil reply is a question that failed outright, which is a box that fell over
// or a browser that never answered. It is not merged and it is not retried
// here: the terms in it still have no rendering, so the next run picks them up
// by asking the same question again. That is the whole retry policy and it is
// enough, because this command is idempotent by construction.
func askEveryBatch(ctx context.Context, root string, hosts []ocr.Host, batches []glossary.Batch, keep bool, logf func(string, ...any)) []*glossary.Reply {
	out := make([]*glossary.Reply, len(batches))
	next := make(chan int)
	go func() {
		defer close(next)
		for i := range batches {
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
					reply, err := askOneBatch(ctx, root, host, batches[i], i, keep)
					if err != nil {
						logf("question %d on %s failed, its terms keep their place in the queue: %v", i+1, host.Name, err)
						continue
					}
					logf("question %d on %s: %d of %d accepted", i+1, host.Name, len(reply.Rows), len(batches[i].Terms))
					mu.Lock()
					out[i] = reply
					mu.Unlock()
				}
			}(host)
		}
	}
	wg.Wait()
	return out
}

func askOneBatch(ctx context.Context, root string, host ocr.Host, batch glossary.Batch, index int, keep bool) (*glossary.Reply, error) {
	question := batch.Prompt()
	call := ocr.NewAsk(host, fleet.SSH{Timeout: 2 * time.Minute}, ocr.Rsync{Timeout: 5 * time.Minute},
		question, batchID(batch.Lang, index, question), keep)
	answer, err := call.Do(ctx)
	if err != nil {
		return nil, err
	}
	// Both halves are kept before either is judged, for the reason the repair
	// path keeps them: an accepted rendering goes into a file, and without the
	// question there is afterwards no way to tell a model that answered badly
	// from one that was asked the wrong thing.
	if err := archiveAsk(root, batch.Lang, index, question, answer.Text, answer.Conversation); err != nil {
		return nil, err
	}
	reply := batch.Audit(answer.Text)
	return &reply, nil
}

// batchID names the scratch directory on the host. The question is hashed into
// it so that two runs over the same terms, which is what happens when the
// prompt changes, do not share a directory.
func batchID(lang string, index int, question string) string {
	sum := sha256.Sum256([]byte(question))
	return fmt.Sprintf("glossary-%s-%03d-%s", lang, index+1, hex.EncodeToString(sum[:])[:6])
}

// archiveAsk keeps the question and the answer under work/, out of the corpus.
func archiveAsk(root, lang string, index int, question, answer, conversation string) error {
	dir := filepath.Join(root, "work", "glossary", lang)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	base := filepath.Join(dir, fmt.Sprintf("%03d-%s", index+1, stamp))
	if err := os.WriteFile(base+".ask.md", []byte(question), 0o600); err != nil {
		return err
	}
	if conversation != "" {
		answer = "<!-- " + conversation + " -->\n\n" + answer
	}
	return os.WriteFile(base+".answer.md", []byte(answer), 0o600)
}

// needing is the terms to ask about: the glossary rows with no rendering in
// this language, then the commonest mined candidates that are not rows yet.
//
// The glossary first, because a term somebody has already decided is worth
// having is worth finishing before a term a miner noticed.
//
// only narrows the candidates to one source. The list is ordered by how often
// the prose says a phrase, which is the right order while the corpus is the
// volumes that are read, and no order at all for a volume that is not: Theory
// of Sets is mined from its table of contents and every candidate it puts in is
// a title with a count of one or two, sitting under two thousand phrases from
// Algebra and Lie. Asking for the titles is how the terminology of a Book gets
// settled before the Book is read.
func needing(root string, g *glossary.Glossary, lang string, add, min int, only string) ([]string, error) {
	var out []string
	have := map[string]bool{}
	for _, t := range g.Terms {
		have[glossary.Key(t.EN)] = true
		if t.In(lang) == "" {
			out = append(out, t.EN)
		}
	}
	if add <= 0 {
		return out, nil
	}
	cands, err := glossary.LoadCandidates(glossary.CandidatesPath(root))
	if err != nil {
		return nil, err
	}
	// -add counts the candidates brought in and not the terms asked about. The
	// two differ by however many rows the glossary already holds without a
	// rendering in this language, and mixing them up is what made -add 320 send
	// 777 terms on the second Vietnamese run.
	stop := len(out) + add
	// The candidates come out of the miner commonest first, so this takes the
	// head of the list and stops. Ordering it again here would be a second
	// opinion about what is common, which is not this command's to have.
	for _, c := range cands.Terms {
		if len(out) >= stop {
			break
		}
		if have[glossary.Key(c.EN)] || (min > 0 && c.Count < min) {
			continue
		}
		if only != "" && !slices.Contains(c.Sources, only) {
			continue
		}
		have[glossary.Key(c.EN)] = true
		out = append(out, c.EN)
	}
	return out, nil
}

func known(lang string) bool {
	for _, l := range glossary.Langs {
		if l == lang {
			return true
		}
	}
	return false
}

// name is a term for a log line, for the case where the line was so far from a
// row that there was no term to blame.
func name(en string) string {
	if strings.TrimSpace(en) == "" {
		return "the answer"
	}
	return en
}
