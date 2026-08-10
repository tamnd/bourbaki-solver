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
	"strings"
	"unicode"
)

// Leak is one thing found in an answer that should not be there.
type Leak struct {
	// Kind is refusal, meta, prompt or empty.
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

// prompts are the model handing back its instructions rather than the page.
// This one is specific to how we ask, so it is kept apart from the general
// narration above.
var prompts = []string{
	"transcribe the complete text",
	"render all mathematical expressions as latex",
	"output only the raw transcribed content",
	"do not summarize, paraphrase",
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
	// One leak per line, worst kind first. A line that both apologises and
	// narrates is one bad line, and reporting it twice makes the failures
	// report count problems it does not have.
	kinds := []struct {
		kind    string
		phrases []string
	}{
		{"refusal", refusals},
		{"prompt", prompts},
		{"meta", metas},
	}
	for i, line := range strings.Split(text, "\n") {
		lower := strings.ToLower(line)
		if found, kind, ok := first(lower, kinds); ok {
			leaks = append(leaks, Leak{Kind: kind, Detail: found, Line: i + 1})
		}
	}
	return leaks
}

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
	return trimmed
}

// Normalise fixes the typography a model substitutes for what the page prints.
//
// Only substitutions that are unambiguously wrong in this corpus are made. The
// minus sign, the two dash lengths and the quotation marks all carry meaning in
// mathematics and are left alone.
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

// Normalise applies those substitutions and trims trailing space from every
// line, which is invisible in review and shows up in every later diff.
func Normalise(text string) string {
	text = normalise.Replace(text)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}
