package translate

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/mathtex"
)

// RuleTerminology is a glossary term that came back in English.
const RuleTerminology = "terminology"

// AuditTerms is L10 of the audit, asked of one chunk while the run can still do
// something about it.
//
// L10 reads the file after it is written and reports a term the translation
// left standing in English. That is the right report and it is the wrong
// moment: the file is in content/, the chunk's answer is cached under work/,
// and nothing puts the question again, so the finding sits in the audit until
// somebody translates the section over. Asked here, the answer is refused and
// askChunk puts it again with the complaint attached, which is the one place in
// this program where a model is told what it did wrong.
//
// It is separate from Audit rather than inside it because it needs the
// glossary and Audit does not. Audit is a comparison of two texts and nothing
// else, and every caller of it has the two texts; only the run has the rows.
//
// The rows are the ones this file's own volume is translated against, which is
// what the caller passes and what the prompt carried. A term scoped to another
// book was never in the question and cannot be what the model was told to
// write.
func AuditTerms(lang string, g *glossary.Glossary, en, tr string) []Problem {
	if g == nil {
		return nil
	}
	source, answer := prose(en), prose(tr)
	source, answer = strings.ToLower(source), strings.ToLower(asPrinted(source, answer))
	var out []Problem
	for _, t := range g.Mentioned(lang, source) {
		// A term whose rendering holds the English word is not evidence of
		// anything. Bourbaki keeps a good many names as they stand, so the
		// rendering is often the English itself, and a Vietnamese sentence
		// holding one of those is a correct sentence. The containment and not
		// only equality, because the glossary writes fortiori as "a fortiori"
		// and every correct answer then has the word fortiori in it. Those
		// five were the last false positives left on the 769 answers on disk.
		if glossary.Mentions(strings.ToLower(t.In(lang)), t.EN) {
			continue
		}
		if !glossary.Mentions(answer, t.EN) {
			continue
		}
		out = append(out, Problem{Rule: RuleTerminology, Msg: fmt.Sprintf(
			"leaves %q in English, and the glossary writes it %q", t.EN, t.In(lang))})
	}
	return out
}

// quoted is a run between quotation marks. The opening mark is the curly one or
// the straight one and so is the closing one, because the printed book sets
// them curly and the reading of these pages has pairs that open curly and close
// straight.
// The mark itself is not part of what is compared with the English, because the
// two sides do not always set the same one: the English of this note has
// "Calculus ratiocinator" straight and the answer came back curly.
var quoted = regexp.MustCompile(`[“"]([^“”"]+)[”"]`)

// asPrinted takes out of the answer the quotations that stand as printed: a run
// between quotation marks that the English has word for word and that holds no
// English word of its own.
//
// Leibniz called his logic a "Calculus ratiocinator", and the historical note
// of chapter IV prints the Latin in quotation marks. A translation keeps it,
// because it is what the man called the thing, and every other name in that
// paragraph is kept the same way. The terminology rule read the word calculus
// in it and asked for "phép tính" instead, so chunk 4 was refused six times
// over two models, and it was the last thing standing between that note and the
// corpus.
//
// The condition that the run hold no English word is what keeps this from
// covering a model that copies a quoted English sentence rather than rendering
// it. The same paragraph quotes Leibniz in English, "The true method should
// provide us with a filum Ariadnes", and that one has English words in it, so
// it is read as it was before. A copied English sentence is also a run of a
// dozen words with nothing Vietnamese in it, which is what auditLanguage
// reports, so it is refused whatever this rule says about it.
func asPrinted(en, tr string) string {
	return quoted.ReplaceAllStringFunc(tr, func(run string) string {
		inside := quoted.FindStringSubmatch(run)[1]
		if glossary.EnglishWords(inside) > 0 || !strings.Contains(en, inside) {
			return run
		}
		return " "
	})
}

// prose is the text with the mathematics taken out, which is what a term has to
// be looked for in.
//
// A display block goes entirely, since a term inside one is a term the
// translator was told to copy rather than to render, and inline spans go
// through mathtex.Strip.
//
// The attribute block of a heading goes too, and leaving it in was the whole
// difference between a rule and a nuisance. Every statement in this corpus
// carries the class .statement, which is markup the translator is ordered to
// copy, and the glossary has a row for the word statement. Run over the 769
// answers already on disk, the rule without this line reported 281 findings
// and with it reports far fewer, and the difference was all that one class
// name.
//
// A bibliography entry goes for the reason auditBiblio gives: it stands as
// printed, so the English words in the title of a book are not English left in
// the answer. Seven terms of chunk 30 of the historical note were that, and the
// chunk was refused over them every time it was asked for.
func prose(body string) string { return proseText(body, false) }

// weldedRE is a formula together with the name written hard against it, on
// either side and with no space between.
//
// A functor is set upright and the extraction leaves the upright part outside
// the dollars, so the corpus writes End$(E)$, Aut$(V)$, Hom$(E,F)$, and in § 17
// of Algebra VIII the reduced norm and trace as Nrd$_{A/K}(a)$, Trd$_{A/K}(a)$
// and Pcrd$_{A/K}(a;X)$. Counted over the English corpus that shape occurs 2165
// times in 155 distinct names, and reading down the list by frequency they are
// operators the whole way: End, Aut, Hom, ad, dim, Tr, Id, exp, Ind, pr, Nrd,
// Pc, Trd, Res, Coind, Ker, Int, Pcrd, Alt. The two that read as ordinary words
// are not: long is the length function long$_A$ of § 11, and the one weld of
// Algebra is inside a citation, which stands as printed for its own reason.
//
// The span has to hold something, and that is what keeps a display safe. A
// display fenced on its own lines never reaches here, because the loop takes
// those lines out first. A display written inline does reach here, and with the
// span allowed to be empty the two opening dollars of
//
//	$$\neg 0 = 1, \qquad \neg 1 = 0,$$
//
// are themselves a match, so the opener is eaten, the rest no longer parses as
// mathematics, and neg and qquad are read as prose. That is the wrong answer in
// the direction that matters: it puts a display back into the prose and refuses
// the one translation a display has. Two tests were already standing on that
// and they caught this on the first run.
var weldedRE = regexp.MustCompile(`\p{L}*\$[^$\n]+\$\p{L}*`)

// ctrlWordRE is a TeX control word, which is never an English word however much
// of one it looks like.
//
// mathtex.Strip takes out what stands between dollar signs, and an index of
// notation has none: its entries are bare LaTeX, "\sum_{i=p}^q \sum_{j=r}^s
// x_{ij} : \text{I, § 1, no. 5}". Nothing there is inside a formula as far as
// Strip can tell, so the whole line came through as prose and the glossary read
// the word sum in \sum and asked for "tổng". Four of the five indexes of
// notation the last run could not land died on that, and the largest of them
// was refused on every attempt over two models.
//
// The comment in Strip about the words left, right and square being read as
// prose is this same fault seen from the other side. It was fixed there for a
// display written $$...$$ on one line, which is where it was found; a control
// word outside dollars altogether was still read, and \left, \right and \square
// are control words.
var ctrlWordRE = regexp.MustCompile(`\\[a-zA-Z]+`)

// notationEntryRE is an entry of an index of notation: the notation itself, a
// colon, and then where in the book it is defined.
//
// The notation is a symbol however much of it is spelled with letters. An index
// writes "Map(M, N), Pol_A(M, N), Pol(M, N) : IV, p. 57", and Map there is the
// functor set upright, the same thing as Hom, End and Aut, which weldedRE says
// occur 2165 times over 155 names. weldedRE cannot help here because it wants
// the dollars that a welded name is written with and an index writes none, so
// the glossary read the word map and asked for "ánh xạ" in a list of symbols.
//
// Only what stands before the colon goes. Where the entry is defined is not
// prose either, but leaving it costs nothing and taking it needs a second guess
// about where the locator ends.
//
// Over content/en this matches 3398 lines in the fourteen index_of_ files and
// four lines anywhere else, all four in one section and all four the heading
// "Applications : I. Canonical decompositions ...", where the I is the number of
// a part and not a volume. Headings are left alone for them.
var notationEntryRE = regexp.MustCompile(`^.{1,400}?\s:\s*((?:\$?\\text\{)?\s*[IVXLC]+(?:[.,]|\s*,?\s*p\.))`)

// proseText is prose, and with welded set it also drops those names.
//
// Only hasProse asks for that. The terminology rule reads the other form and
// goes on seeing every word it saw before, because the question the two are
// asking is not the same one: hasProse asks whether there is anything here to
// translate, and a functor is not, while the terminology rule asks whether a
// term that was translated was translated the agreed way, and it can afford to
// look at a word that turns out not to be one.
func proseText(body string, welded bool) string {
	var b strings.Builder
	inDisplay := false
	// A display whose fences are welded to the prose either side of them is
	// taken out first, because the loop below cannot see one. The loop stays for
	// a fence with no partner, which the toggle handles and a regexp does not.
	for _, line := range strings.Split(mathtex.BlankDisplays(body), "\n") {
		if strings.TrimSpace(line) == "$$" {
			inDisplay = !inDisplay
			continue
		}
		if inDisplay || bibEntryRE.MatchString(line) || refTailRE.MatchString(line) {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			line = notationEntryRE.ReplaceAllString(line, " $1")
		}
		line = attrRE.ReplaceAllString(line, " ")
		if welded {
			line = weldedRE.ReplaceAllString(line, " ")
		}
		b.WriteString(ctrlWordRE.ReplaceAllString(mathtex.Strip(line), " "))
		b.WriteString("\n")
	}
	return b.String()
}
