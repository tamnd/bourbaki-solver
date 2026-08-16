package translate

import (
	"fmt"
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
	source, answer := strings.ToLower(prose(en)), strings.ToLower(prose(tr))
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
func prose(body string) string {
	var b strings.Builder
	inDisplay := false
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "$$" {
			inDisplay = !inDisplay
			continue
		}
		if inDisplay {
			continue
		}
		b.WriteString(mathtex.Strip(attrRE.ReplaceAllString(line, " ")))
		b.WriteString("\n")
	}
	return b.String()
}
