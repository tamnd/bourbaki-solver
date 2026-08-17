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
func prose(body string) string {
	var b strings.Builder
	inDisplay := false
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "$$" {
			inDisplay = !inDisplay
			continue
		}
		if inDisplay || bibEntryRE.MatchString(line) {
			continue
		}
		b.WriteString(mathtex.Strip(attrRE.ReplaceAllString(line, " ")))
		b.WriteString("\n")
	}
	return b.String()
}
