package ocr

import "regexp"

// The line the transport writes about itself.
//
// chatgpt-tool exports the assistant turn as markdown and puts the address of
// the conversation at the head of it, in an HTML comment:
//
//	<!-- https://chatgpt.com/?temporary-chat=true -->
//
// That is a note about where the answer came from and not a part of the answer,
// and every caller here has to take it off before anything reads the text. The
// translator counts blocks, and a comment is a block, so an answer with this
// line in it has one block more than the English and is refused as a structural
// change to the section. Measured over the 950 answers the Vietnamese run has
// on disk, 147 carry the line and taking it off turns 109 refusals into
// accepted answers, which is one question in nine that the run asked, waited
// five minutes for, threw away and asked again.
//
// It is stripped here rather than in the tool because this is the side that has
// to be right about answers from every version of the tool that is installed on
// any of the boxes, and because a comment naming chatgpt.com is never something
// a page of Bourbaki or a translation of one contains.
var transportNote = regexp.MustCompile(`\A\s*<!--\s*https://chatgpt\.com[^>]*-->[ \t]*\r?\n?`)

// clean is the answer without the transport's own note at the head of it.
//
// Only at the head, and only the one line. A comment further down is the
// model's and is the model's mistake to answer for; the audit has a rule about
// commentary and it is the rule that should speak, not this.
func clean(text string) string {
	return transportNote.ReplaceAllString(text, "")
}
