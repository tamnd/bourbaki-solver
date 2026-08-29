// Package textguard catches the ways a model's answer is not what was asked
// for, before it reaches the corpus.
//
// A model handed a page image sometimes talks instead of transcribing. It
// apologises, it announces what it is about to do, it summarises, or it hands
// back the prompt. All of that is fluent English and none of it is Bourbaki,
// and once it is written to a page file it looks exactly like text that was
// read off the page. Catching it costs a string search; not catching it costs a
// hand audit of 1194 pages.
package textguard

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Leak is one thing found in an answer that should not be there.
type Leak struct {
	// Kind is gateway, refusal, no-image, meta, prompt, markup or empty.
	Kind string
	// Detail is the phrase that was found, as it appeared.
	Detail string
	// Line is where it was found, one based, 0 when the whole answer is at
	// fault.
	Line int
}

// refusals are the model declining. A page that opens with one of these has no
// transcription in it at all.
var refusals = []string{
	"i'm sorry",
	"i am sorry",
	"i apologize",
	"i apologise",
	"i cannot",
	"i can't",
	"i'm unable",
	"i am unable",
	"i'm not able",
	"unable to assist",
	"can't help with",
	"cannot help with",
	"as an ai",
	"as a language model",
	"i'm just an ai",
	"against my guidelines",
	// Narrow on purpose. A refusal says the request violates something; a
	// theorem says a map violates no relation, and the bare word rejected a
	// real page of chapter IV the first time this ran.
	"violates our",
	"violates my",
	"violates the content",
}

// gateway is the service turning the account away before the model ever sees
// the question.
//
// It is not a refusal, because nothing was refused: no model read the page and
// no model declined it. It is the anti abuse layer in front of the account, and
// it is worth its own kind because the remedy is different in every respect. A
// refusal is answered by asking again, perhaps of another model. This is
// answered by leaving the account alone for a while, and asking again at once
// is the one thing that makes it last longer.
//
// Seventeen files were written out of it before this list existed. The whole
// body of each was the sentence and a trace id, about a hundred characters,
// which cleared the minimum length once the tool had put its own header above
// it, exactly as "I don't see an image attached" did. Six were Vietnamese
// exercises of General Topology and three were machine English.
//
// The trace id is why the phrases stop where they do. Every one of these
// carries a different guid, so the sentence has to be matched on the part that
// does not vary.
var gateway = []string{
	"unusual activity has been detected from your device",
	"you've been rate limited",
	"you have been rate limited",
	"too many requests in a short period",
	"our systems have detected unusual activity",
	"access denied",
	"verify you are human",
}

// noImage is the model answering politely to a message that arrived without its
// attachment.
//
// This is not a refusal and not narration. It is the upload having failed: the
// prompt got through, the page did not, and the model says so and asks for it.
// It is worth its own kind because the answer is not to ask a model again more
// loudly, it is that a browser upload did not finish, and the failures report
// should say which of the two happened.
//
// It cost three pages of the first live run. The answer is about a hundred and
// thirty characters, which is under the length rule, but the tool writes four
// lines of its own header above it, and with that on top it cleared the
// minimum and "I don't see an image attached" was written to the corpus as
// page 42 of Algebra I.
// The first person is load bearing. "we do not see" is ordinary mathematical
// prose and "the image is not attached to any choice of basis" is a sentence
// Bourbaki could print; both were rejected by a looser version of this list
// before it ever ran on a page.
var noImage = []string{
	"i don't see an image",
	"i don't see the image",
	"i don't see any image",
	"i do not see an image",
	"i do not see the image",
	"i didn't receive an image",
	"i did not receive an image",
	"no image was attached",
	"there is no image attached",
	"please upload the",
	"please upload an image",
	"please attach the image",
	"upload the page image",
	"you want transcribed",
}

// metas are the model narrating. These are worse than refusals because the
// transcription usually follows and the page looks almost right.
var metas = []string{
	"here is the transcription",
	"here's the transcription",
	"here is the text",
	"here's the text",
	"here is the transcribed",
	"the image shows",
	"the image contains",
	"this image appears",
	"this page appears to be",
	"sure, here",
	"sure! here",
	"certainly, here",
	"certainly! here",
	"below is the transcription",
	"i have transcribed",
	"transcription of the image",
	"let me know if",
	"hope this helps",
	"note that i have",
}

// Deliberately not here: in summary, to summarize, and the like. A model that
// summarises a page says so at the top, and it is caught by the phrases above
// or by the length check. Mathematical prose says it too, and a page rejected
// for saying it costs three reads at 151 seconds each and then lands in the
// failures report as a defect that is not one.

// prompts are the model handing back its instructions rather than the answer.
// These are specific to how we ask, so they are kept apart from the general
// narration above.
//
// The second group is the solve side. It cost six files: two solutions of the
// Theory of Sets went into the corpus containing the repair prompt read back,
// its headings, its rules and its placeholder tag line, and nothing else. They
// came out unverified with "solution unintelligible" against every part, which
// is the pipeline saying an answer failed where there was no answer to fail.
// Every phrase here is a line of prompt/solve_correct.md or a heading it sets,
// and none of them is a sentence the register of the book has room for.
var prompts = []string{
	"transcribe the complete text",
	"render all mathematical expressions as latex",
	"output only the raw transcribed content",
	"do not summarize, paraphrase",

	"the solution as it stands",
	"what the judges said",
	"do not write a list of changes",
	"a note about what you fixed",
	"uses: xxxx",
	"xxxx and yyyy",
}

// thinking is the model handing back its reasoning rather than its answer.
//
// It is not narration and it is not a refusal. The model works out what it has
// been asked, in the first person and in numbered steps, and stops before it
// writes the thing it was working out. There is no answer under it at all,
// which is what separates it from a meta line with a transcription following.
//
// The opener is the whole of what is looked for. A model that thinks out loud
// says so at the top and then goes on for pages, and every line after the first
// is ordinary English about the problem, which is also what a solution is made
// of. Bourbaki does not open a proof by announcing a thinking process.
var thinking = []string{
	"here's a thinking process",
	"here is a thinking process",
	"here's my thinking process",
	"here is my thinking process",
	"let me think through this step by step",
}

// markup is the provider's own formatting, wrapped around an answer that is
// otherwise fine.
//
// This is a different failure from the ones above and it is the nastier one,
// because there is no English sentence to search for and every rule that
// compares a translation with its English can pass it. It happened: a retranslation
// of the appendix on the Nullstellensatz came back inside
//
//	:::writing{variant="document" id="58321"}
//	...
//	:::
//
// and reached the corpus. The math spans matched, the tags matched, the heading
// tree matched, the block count matched because the fence lines had no blank
// line around them and so joined the paragraphs either side of them, and all
// seven translation rules passed the file. It was found by reading the diff.
//
// A ::: line is a directive fence, which the Markdown this corpus is written in
// does not use anywhere: 3,891 English paragraphs and not one of them has a
// ::: on it. The private use area is where a provider hides its own citation
// anchors, and 【 】 is where another one puts them. None of the three can be
// part of a page of Bourbaki, so all three are refused wherever they turn up
// rather than only at the start of an answer.
// directive is a line the provider opens or closes its own writing surface
// with. Strip removes it and Check refuses it, and they share the one pattern
// so that the two can never come to disagree about what a fence is.
var directive = regexp.MustCompile(`(?m)^\s*:::`)

var markup = []struct {
	what string
	re   *regexp.Regexp
}{
	{"a directive fence, which is the provider's markup and not Markdown this corpus uses", directive},
	{"a citation anchor", regexp.MustCompile(`【[^】]*】|oai_citation|contentReference`)},
	{"a private use character, which is a provider's own marker", regexp.MustCompile(`[\x{e000}-\x{f8ff}]`)},
}

// Check reads an answer and reports everything wrong with it.
//
// Every leak is reported rather than the first, because a page that both
// apologises and narrates is a different failure from one that only narrates,
// and the retry that follows is chosen from what was found.
func Check(text string) []Leak {
	var leaks []Leak
	if strings.TrimSpace(text) == "" {
		return []Leak{{Kind: "empty", Detail: "the answer is empty"}}
	}
	for i, line := range strings.Split(text, "\n") {
		for _, m := range markup {
			if found := m.re.FindString(line); found != "" {
				leaks = append(leaks, Leak{Kind: "markup",
					Detail: m.what + ", " + strconv.Quote(strings.TrimSpace(found)), Line: i + 1})
				break
			}
		}
	}
	// One leak per line, worst kind first. A line that both apologises and
	// narrates is one bad line, and reporting it twice makes the failures
	// report count problems it does not have.
	kinds := []struct {
		kind    string
		phrases []string
	}{
		{"gateway", gateway},
		{"refusal", refusals},
		{"no-image", noImage},
		{"prompt", prompts},
		{"thinking", thinking},
		{"meta", metas},
	}
	for i, line := range strings.Split(text, "\n") {
		lower := straighten(strings.ToLower(line))
		if found, kind, ok := first(lower, kinds); ok {
			leaks = append(leaks, Leak{Kind: kind, Detail: found, Line: i + 1})
		}
	}
	return leaks
}

// straighten turns the typographic apostrophe into the ASCII one for matching
// only.
//
// A model writes I don’t and I’m sorry with U+2019, and every phrase above is
// spelled with U+0027, so none of them matched. That is not a hypothetical
// either: it is why "I don’t see an image attached" reached the corpus with no
// leak reported at all. The answer itself is untouched, because the apostrophe
// a page prints is the page's business.
var straightener = strings.NewReplacer("’", "'", "‘", "'", "＇", "'")

func straighten(text string) string { return straightener.Replace(text) }

func first(lower string, kinds []struct {
	kind    string
	phrases []string
}) (string, string, bool) {
	for _, group := range kinds {
		for _, phrase := range group.phrases {
			if strings.Contains(lower, phrase) {
				return phrase, group.kind, true
			}
		}
	}
	return "", "", false
}

// Clean says whether an answer is free of leaks.
func Clean(text string) bool { return len(Check(text)) == 0 }

// fences finds the code fence a model wraps an answer in when it was asked for
// Markdown and decided to be helpful.
var fences = regexp.MustCompile("(?s)\\A```[a-zA-Z]*\\n(.*)\\n```\\s*\\z")

// Strip removes the wrapping a model adds around an otherwise good answer.
//
// This is not the same as Check. A fenced answer is correct text in the wrong
// packaging and unwrapping it is safe; a narrated answer is text that may have
// been altered, and no amount of trimming makes that safe, so it is rejected
// rather than repaired.
func Strip(text string) string {
	trimmed := strings.TrimSpace(text)
	if match := fences.FindStringSubmatch(trimmed); match != nil {
		trimmed = strings.TrimSpace(match[1])
	}
	return strings.TrimSpace(undirect(trimmed))
}

// undirect drops the provider's directive fences.
//
// This is the same argument as the code fence above it and it took a corpus
// commit to learn. Six translated files came back wrapped in
// :::writing{variant="document"} and its closing :::, ten pairs in Theory of
// Sets II section 3 alone, and the first machine English section ever written
// had the same fault, so it is the provider and not the language. Check has
// refused the pattern from the start and H07 found the files, but both of those
// are after the fact: a run started with -raw keeps the answer and writes it,
// which is what the corpus asked for and what it got.
//
// So the fence is removed rather than refused. The text either side of it is
// the translation and it is not touched, and a corpus that has no ::: on any
// line of any file cannot lose anything to a rule that drops lines beginning
// with one.
//
// The blank line in front of a fence goes with it. A fence sits in its own
// paragraph, and taking the line out and leaving the blank behind would turn one
// paragraph break into two, which is a change to the text rather than to its
// packaging.
func undirect(text string) string {
	if !strings.Contains(text, ":::") {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if directive.MatchString(line) {
			if n := len(out); n > 0 && strings.TrimSpace(out[n-1]) == "" {
				out = out[:n-1]
			}
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// Normalise fixes the typography a model substitutes for what the page prints.
//
// Only substitutions that are unambiguously wrong in this corpus are made. The
// minus sign, the two dash lengths and the quotation marks all carry meaning in
// mathematics and are left alone.
// bareBlackboard is the same substitution without the braces. A single letter
// argument does not need them and a model does not always write them: the first
// live page came back with $\mathbb Z$ and $\mathbb N$, which the list below
// spells with braces and therefore missed. The following character must not be
// a letter, or \mathbb Zeta would lose its tail.
var bareBlackboard = regexp.MustCompile(`\\mathbb\s+([ZQRCNFP])(?:\{\})?\b`)

var normalise = strings.NewReplacer(
	// A model that was told to write bold sets sometimes writes blackboard
	// bold anyway. Bourbaki prints bold, and a corpus that mixes the two makes
	// every search for a ring name miss half its hits.
	`\mathbb{Z}`, `\mathbf{Z}`,
	`\mathbb{Q}`, `\mathbf{Q}`,
	`\mathbb{R}`, `\mathbf{R}`,
	`\mathbb{C}`, `\mathbf{C}`,
	`\mathbb{N}`, `\mathbf{N}`,
	`\mathbb{F}`, `\mathbf{F}`,
	`\mathbb{P}`, `\mathbf{P}`,
	// The dangerous bend comes back as several near misses.
	"⚠", "☡",
	"⛰", "☡",
	// Non-breaking and thin spaces read as ordinary spaces everywhere they
	// appear in these volumes, and they break every word match if kept.
	" ", " ",
	" ", " ",
	" ", " ",
	// Zero width characters carry nothing and hide differences in a diff. They
	// are written as escapes because a byte order mark in the middle of a Go
	// source file is a compile error, and a zero width space in one is
	// invisible to whoever reads it next.
	"\u200b", "",
	"\ufeff", "",
)

// Normalise applies those substitutions, writes the corpus's delimiters and its
// star, and trims trailing space from every line, which is invisible in review
// and shows up in every later diff.
//
// The star is not in the replacer with the dangerous bend, though the two faults
// are the same fault. A replacer cannot see where the mathematics is, and two of
// the five spellings the star comes back as are operations inside a span: U+2217
// is a binary law or a dual, and the plain asterisk is a convolution, an adjoint
// or the units of a ring, running through the volumes in their thousands as K^*.
// So Stars runs on its own, after the delimiters have been turned round, since
// it has to be able to find the spans to keep out of them.
func Normalise(text string) string {
	text, _ = Stars(substitute(text))
	return trimRight(text)
}

// NormaliseProse is Normalise with the star left out, for text that is written
// rather than read off a page.
//
// A solution is Markdown somebody's own hand wrote, and Markdown uses the
// asterisk for its own purposes: a bullet list opens with one at the head of a
// line with a space after it, which is exactly the shape Stars was written to
// find. On a scanned page that shape is Bourbaki's mark, because the volumes set
// no bullet lists; in a solution it is a list, and turning it into \* would put a
// backslash at the head of every item. The rest of Normalise has no such
// argument against it, so a solution gets the rest of it.
func NormaliseProse(text string) string { return trimRight(substitute(text)) }

// substitute is everything the two of them share.
func substitute(text string) string {
	text = bareBlackboard.ReplaceAllString(text, `\mathbf{$1}`)
	text, _ = Dollars(text)
	return normalise.Replace(text)
}

func trimRight(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}

// EchoLines is how many whole lines of the prompt have to come back before the
// answer is called an echo.
//
// One line is enough to be certain in principle and two is what is used, on the
// grounds that a page of a book about mathematics could in principle print a
// sentence that also appears in an instruction about transcribing mathematics.
// Two of them in the same answer, matching end to end after the spacing is
// flattened, could not happen by accident. Counted over the 4862 readings on
// disk, 4860 echo none of the four OCR prompts and 2 echo fifteen lines each,
// so anything from one to fifteen separates them and the number is not delicate.
const EchoLines = 2

// EchoLength is how long a line of the prompt has to be to be worth looking for.
//
// Short lines are headings, blank lines and fragments such as `# Rules`, and
// those turn up in a page of a book often enough to be no evidence at all.
const EchoLength = 40

// Echo reports a prompt handed back in place of the page.
//
// This is the failure that rule 3's phrase list was written for, and the phrase
// list does not catch it, because the phrases were taken off a prompt that has
// since been rewritten. Searching for the prompt the model was actually given
// needs no list and cannot go stale: pages 3 and 9 of Algebra IV to VII are in
// the corpus today carrying every rule of prompt/ocr_bourbaki.md as their body,
// having passed all the rules of the day, because the text is long, it is prose,
// no unbalanced mathematics in it and its first line reads like a running head.
//
// Whole lines, and not phrases inside a line. A page that quotes six words of an
// instruction has quoted six words; a page that reproduces a paragraph of it end
// to end was never read.
func Echo(text, prompt string) []Leak {
	want := map[string]bool{}
	for _, line := range strings.Split(prompt, "\n") {
		if flat := flatten(line); len([]rune(flat)) >= EchoLength {
			want[flat] = true
		}
	}
	if len(want) == 0 {
		return nil
	}
	var hits []Leak
	seen := map[string]bool{}
	for i, line := range strings.Split(text, "\n") {
		flat := flatten(line)
		if len([]rune(flat)) < EchoLength || !want[flat] || seen[flat] {
			continue
		}
		seen[flat] = true
		hits = append(hits, Leak{Kind: "prompt", Detail: clipLine(line), Line: i + 1})
	}
	if len(hits) < EchoLines {
		return nil
	}
	return hits
}

// flatten puts a line in the form the two sides are compared in: lower case,
// one space between words, and the Markdown that marks a list item or a heading
// taken off the front, because the model reindents what it hands back.
func flatten(line string) string {
	line = strings.ToLower(straighten(line))
	line = strings.TrimLeft(line, " \t-*#>_`")
	return strings.Join(strings.Fields(line), " ")
}

// clipLine keeps the report readable. A rule of the prompt runs to three
// hundred characters and the failures report puts one leak on a line.
func clipLine(line string) string {
	line = strings.TrimSpace(line)
	if r := []rune(line); len(r) > 60 {
		return string(r[:60]) + "..."
	}
	return line
}
