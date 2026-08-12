package publish

import (
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/quality"
)

// The front page and /about/, spec 12 §4.4: the coverage table generated and
// not typed, and the unflattering numbers up with the rest.
//
// Nothing on either page is a number somebody wrote down. Every one of them is
// counted out of the corpus at build time, because a hand written coverage
// claim is wrong the week after it is written and nobody who reads it can tell.
// The front page holds the short version and /about/ holds the long one, with
// the method, what a machine wrote, what is wrong with it, and the licence.

const (
	corpusRepo = "https://github.com/tamnd/bourbaki"
	solverRepo = "https://github.com/tamnd/bourbaki-solver"
)

// langNumbers is one row of the coverage table.
type langNumbers struct {
	Lang       string
	Sections   int
	Statements int
	Exercises  int
	// Share of the English section count, 0 to 1. It is what the coverage floor
	// is measured against and it is what the table prints, since "2 sections" on
	// its own does not say whether that is most of the chapter or a tenth of it.
	Share float64
	// Models and Glossaries are the translation provenance of the language,
	// deduplicated and sorted, empty for a language that was extracted rather
	// than translated. They are lists rather than a single value because the
	// Vietnamese was written over two runs by two models against two versions of
	// the glossary, and a page that printed one of them would be lying about the
	// other section.
	Models     []string
	Glossaries []int
	// Extraction is the methods the files record, "native" and however many of
	// each, so that the claim about where the text came from is counted and not
	// asserted.
	Extraction map[string]int
	Translated bool
}

// numbers counts the corpus, once, for both pages.
//
// It counts what this build loaded rather than what the corpus holds. Under
// -lang fr the table is French alone and the share column is empty, which is
// right: the page describes the site it is part of.
func (s *Site) numbers() []langNumbers {
	by := map[string]*langNumbers{}
	for _, lang := range s.Langs {
		by[lang] = &langNumbers{Lang: lang, Extraction: map[string]int{}}
	}
	for _, sec := range s.Sections {
		n := by[sec.Lang]
		if n == nil {
			continue
		}
		n.Sections++
		n.Statements += sec.Meta.Statements
		method := sec.Meta.Extraction
		if method == "" {
			method = "unrecorded"
		}
		n.Extraction[method]++
		if sec.Meta.TranslatedFrom == "" {
			continue
		}
		n.Translated = true
		if m := sec.Meta.TranslationModel; m != "" {
			n.Models = append(n.Models, m)
		}
		if v := sec.Meta.GlossaryVersion; v > 0 {
			n.Glossaries = append(n.Glossaries, v)
		}
	}
	for _, ex := range s.Exercises {
		if n := by[ex.Lang]; n != nil {
			n.Exercises++
		}
	}
	var out []langNumbers
	for _, lang := range s.Langs {
		n := by[lang]
		if en := by["en"]; en != nil && en.Sections > 0 {
			n.Share = float64(n.Sections) / float64(en.Sections)
		}
		n.Models = uniq(n.Models)
		sort.Ints(n.Glossaries)
		n.Glossaries = uniqInts(n.Glossaries)
		out = append(out, *n)
	}
	return out
}

func uniq(in []string) []string {
	sort.Strings(in)
	var out []string
	for i, v := range in {
		if i == 0 || v != in[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func uniqInts(in []int) []int {
	var out []int
	for i, v := range in {
		if i == 0 || v != in[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// langNames is what to call a language in a sentence. The code is what the
// URLs and the -lang flag are in and it stays in the tables; a paragraph that
// says "the vi is machine translation" is written for the build and not for a
// reader. A language not in this list is printed as its code, which is wrong
// looking enough that somebody will add it.
var langNames = map[string]string{
	"en": "English",
	"fr": "French",
	"vi": "Vietnamese",
	"zh": "Chinese",
	"ja": "Japanese",
}

func langName(code string) string {
	if name := langNames[code]; name != "" {
		return name
	}
	return code
}

// made is the phrase the coverage table prints for how a language got here.
func (n langNumbers) made() string {
	if !n.Translated {
		return "transcribed from the printed volume"
	}
	return "machine translation, not checked by a person"
}

// uncited is how many of the loaded statements nothing in the chapter cites.
//
// It is on the about page because it is the number that says what the reference
// graph is worth. A chapter where half the statements are cited by nothing is
// either a chapter of definitions or a graph that is not reading the references
// properly, and the reader gets to decide which.
func (s *Site) uncited() int {
	n := 0
	for _, st := range s.Statements {
		if len(s.CitedBy[st.Tag]) == 0 {
			n++
		}
	}
	return n
}

// unplaced is how many statements the page index could not put on a printed
// page, which is the one thing on a tag page that a reader with the volume open
// actually needs.
func (s *Site) unplaced() int {
	n := 0
	for _, st := range s.Statements {
		if st.Page == 0 {
			n++
		}
	}
	return n
}

func (s *Site) about() (string, error) {
	set, err := s.TagSet()
	if err != nil {
		return "", err
	}
	nums := s.numbers()

	// A tag in the file with nothing to serve at its URL. The tag table prints
	// these one by one and marked; here it is the count, because the count is
	// the promise: the tag scheme is worth what this number is close to zero.
	noPage := 0
	for _, e := range set.Tags {
		if s.Tag(string(e.Tag)) == nil && s.ExerciseTag(string(e.Tag)) == nil {
			noPage++
		}
	}

	var b strings.Builder
	b.WriteString("<h1>About</h1>\n")
	b.WriteString("<p>A Markdown corpus of Bourbaki's <em>Éléments de mathématique</em> with a permanent " +
		"tag on every numbered statement, and this site built out of it. It is a study project. It is not " +
		"Bourbaki's, it is not Springer's, and it is not a substitute for the books.</p>\n")
	fmt.Fprintf(&b, "<p>The site is generated by <code>bourbaki publish</code> out of committed Markdown "+
		"and nothing else. No PDF, no scratch directory and no network at build time, so anybody with a "+
		"clone of <a href=%q>the corpus</a> and <a href=%q>the tooling</a> gets the same bytes. Every number "+
		"on this page is counted out of those files by the build.</p>\n", corpusRepo, solverRepo)

	b.WriteString("<h2>What is here</h2>\n")
	b.WriteString("<p>One chapter of one Book, and the library holds forty three volumes. A site that " +
		"looked like the <em>Éléments</em> and held chapter VIII of Algebra would be a worse thing than no " +
		"site, so here is what is actually in it.</p>\n")
	b.WriteString("<table class=\"coverage\">\n<tr><th>language</th><th>sections</th><th>statements</th>" +
		"<th>exercises</th><th>of the English</th><th></th></tr>\n")
	for _, n := range nums {
		share := ""
		if n.Share > 0 {
			share = fmt.Sprintf("%.0f%%", n.Share*100)
		}
		note := n.made()
		if s.Draft[n.Lang] {
			note += ", under the coverage floor"
		}
		fmt.Fprintf(&b, "<tr><td><a href=%q>%s</a> <span class=\"count\">%s</span></td>"+
			"<td>%d</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td></tr>\n",
			s.langHome(n.Lang), n.Lang, langName(n.Lang), n.Sections, n.Statements, n.Exercises, share, note)
	}
	b.WriteString("</table>\n")
	if s.drafted() {
		fmt.Fprintf(&b, "<p>A language holding less than %d per cent of the English is left out of the "+
			"language switcher on the pages themselves and is reached from here and from the front page. "+
			"A switcher that offers a language and then has nothing to switch to is worse than one that "+
			"does not offer it. The floor is about how much of the chapter a language has and it is not a "+
			"judgement about the translation.</p>\n", int(coverageFloor*100))
	}

	b.WriteString("<h2>Where the text came from</h2>\n")
	b.WriteString(s.extraction(nums))
	b.WriteString("<p>What the machine did decide is where one statement ends and the next begins, and " +
		"which of them is Proposition 6. That segmentation is the assembler reading the printed text, not " +
		"markup the book carries, and the whole tag scheme sits on top of it. A statement cut in the wrong " +
		"place is the failure to watch for, and it is what the printings report in the corpus is for: the " +
		"French and the English of one chapter should hold the same statements section by section, and " +
		"where they do not, one of the two is being read wrongly.</p>\n")

	b.WriteString("<h2>What a machine wrote</h2>\n")
	b.WriteString(s.written(nums))
	b.WriteString("<p>Solutions to the exercises are machine written and machine judged and are not " +
		"Bourbaki's. None are in the corpus yet, so every exercise page says there is no solution. When " +
		"they land, the note above them does not go away.</p>\n")

	b.WriteString("<h2>What is wrong with it</h2>\n")
	b.WriteString("<p>The numbers that do not flatter it are generated with the rest of them.</p>\n")
	b.WriteString("<table class=\"coverage\">\n")
	fmt.Fprintf(&b, "<tr><td>references found in the chapter</td><td>%d</td></tr>\n", s.Edges)
	fmt.Fprintf(&b, "<tr><td>references that do not resolve</td><td>%d</td></tr>\n", s.Unresolved)
	fmt.Fprintf(&b, "<tr><td>statements nothing in the chapter cites</td><td>%d of %d</td></tr>\n",
		s.uncited(), len(s.Statements))
	fmt.Fprintf(&b, "<tr><td>statements whose printed page is not known</td><td>%d of %d</td></tr>\n",
		s.unplaced(), len(s.Statements))
	fmt.Fprintf(&b, "<tr><td>tags with no page to serve</td><td>%d of %d</td></tr>\n", noPage, len(set.Tags))
	fmt.Fprintf(&b, "<tr><td>label renames the tags outlived</td><td>%d</td></tr>\n", len(set.Aliases))
	fmt.Fprintf(&b, "<tr><td>retired tags</td><td>%d</td></tr>\n", len(set.Inactive))
	b.WriteString("</table>\n")
	fmt.Fprintf(&b, "<p>Most of what does not resolve is a reference out of the chapter, to a Book this "+
		"corpus does not hold yet, and that is the ingestion order rather than a fault. The lists are "+
		"generated into <a href=%q>reports/</a> in the corpus, one file per question, and they are "+
		"regenerated and diffed on every change so that none of them can quietly go stale.</p>\n",
		corpusRepo+"/tree/main/reports")

	b.WriteString("<h2>Licence</h2>\n")
	b.WriteString("<p><em>Éléments de mathématique</em> is copyright N. Bourbaki and its publishers, " +
		"Hermann, Masson, Springer and Springer Nature. Nothing here transfers or diminishes that. The " +
		"transcriptions, the translations and the solutions are derived works made for personal study and " +
		"they are not for commercial use. No source PDF is in either repository or served from this site. " +
		"The tooling that builds all of it is separate and is under the MIT licence.</p>\n")

	return s.render(layout{Title: "About", Content: template.HTML(b.String())})
}

// extraction is the paragraph on how the text got out of the book, with the
// methods counted rather than claimed.
//
// It matters more than it looks. "Extracted by a model" and "taken off the text
// layer by pdftotext" are two completely different levels of trust in a
// sentence, and the front matter records which of the two every file is. A
// reader deciding whether to believe a formula should not have to guess.
func (s *Site) extraction(nums []langNumbers) string {
	total := map[string]int{}
	for _, n := range nums {
		if n.Translated {
			continue
		}
		for method, count := range n.Extraction {
			total[method] += count
		}
	}
	var methods []string
	for m := range total {
		methods = append(methods, m)
	}
	sort.Strings(methods)

	var parts []string
	for _, m := range methods {
		parts = append(parts, fmt.Sprintf("%d %s", total[m], m))
	}
	var b strings.Builder
	b.WriteString("<p>The volumes are PDFs with a text layer, so the words are lifted off that layer by " +
		"<code>pdftotext -layout</code> and assembled into sections by code. No model wrote those " +
		"sentences and no model rewrote them, which is why the transcriptions are worth more than the " +
		"translations are. ")
	switch {
	case len(methods) == 1 && methods[0] == "native":
		fmt.Fprintf(&b, "All %d transcribed sections record <code>native</code> extraction, with no model "+
			"in the path at all.</p>\n", total["native"])
	case len(methods) == 0:
		b.WriteString("This build loaded no transcribed section, so there is nothing to count here.</p>\n")
	default:
		fmt.Fprintf(&b, "The transcribed sections record %s. Anything that is not <code>native</code> had "+
			"a model somewhere in it, and the page for such a section says so at its foot.</p>\n",
			english(parts))
	}
	return b.String()
}

// written is the paragraph on the translations, generated from what the files
// record. Spec 12 §7: the model, the glossary version and whether the model was
// a cut down one, on every page and gathered here.
func (s *Site) written(nums []langNumbers) string {
	var b strings.Builder
	var small []string
	said := false
	// The English section count is what a translation's own count is worth
	// reading against, since "2 sections" says nothing on its own.
	whole := 0
	for _, n := range nums {
		if n.Lang == "en" {
			whole = n.Sections
		}
	}
	for _, n := range nums {
		if !n.Translated {
			continue
		}
		said = true
		fmt.Fprintf(&b, "<p>The %s is machine translation of the English and is not checked by a person. ",
			langName(n.Lang))
		if whole > 0 {
			fmt.Fprintf(&b, "%d sections of the %d the English holds", n.Sections, whole)
		} else {
			fmt.Fprintf(&b, "%d sections", n.Sections)
		}
		switch len(n.Models) {
		case 0:
			b.WriteString(", by a model the files do not name")
		default:
			fmt.Fprintf(&b, ", written by %s", english(n.Models))
		}
		if len(n.Glossaries) > 0 {
			fmt.Fprintf(&b, " against %s of the terminology glossary", versions(n.Glossaries))
		}
		if len(n.Models) > 1 || len(n.Glossaries) > 1 {
			b.WriteString(", which is what a translation written over more than one run looks like")
		}
		b.WriteString(". Every one of those pages carries its own line saying which model wrote it and " +
			"which glossary it was held to, since that is what a reader needs before deciding whether to " +
			"believe a sentence.</p>\n")
		for _, m := range n.Models {
			if quality.SmallModel(m) {
				small = append(small, m)
			}
		}
	}
	if !said {
		return "<p>This build holds no translated section. Every page of it is transcription.</p>\n"
	}
	if len(small) > 0 {
		fmt.Fprintf(&b, "<p>%s a cut down version of the model the rest of the language was written by. "+
			"Nobody chose that, the account was moved down between one section and the next, and the "+
			"sections it wrote say so on their own pages rather than only in the audit.</p>\n", isAre(small))
	}
	// No date. The front matter records a run identifier and a content hash and
	// not a date, so the page says the model and the glossary version, which are
	// the two things it can actually stand behind.
	b.WriteString("<p>What is not recorded anywhere is when a translation was written. The front matter " +
		"carries the run it came from and the hash of the English it was made from, and staleness is " +
		"decided on the hash rather than on a date, so a date would be decoration.</p>\n")
	return b.String()
}

// isAre opens the sentence about the cut down models, in the number there are.
func isAre(models []string) string {
	if len(models) == 1 {
		return template.HTMLEscapeString(models[0]) + " is"
	}
	return english(models) + " are"
}

func versions(vs []int) string {
	var parts []string
	for _, v := range vs {
		parts = append(parts, strconv.Itoa(v))
	}
	if len(parts) == 1 {
		return "version " + parts[0]
	}
	return "versions " + english(parts)
}

// english joins a list the way a sentence does, so that the generated prose
// reads as prose and not as a comma separated field. It escapes as it goes and
// it copies rather than escaping the caller's slice, which is a model name that
// the caller has more to do with.
func english(in []string) string {
	parts := make([]string, len(in))
	for i, p := range in {
		parts[i] = template.HTMLEscapeString(p)
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}
