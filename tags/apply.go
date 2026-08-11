package tags

import "strings"

// Apply writes the tags of a section into its statement headings.
//
// It is a decoration and not a structural change, which is why it is a pass of
// its own rather than something assembly does while it builds the heading. Two
// commands put a tag into a file, assign when it allocates one and assemble
// every time it writes the corpus, and if those two disagreed by a space the
// determinism check would fail and the corpus would rewrite itself on every
// run. There is one of them here, and both call it.
//
// A label the lookup does not know is left bare rather than refused, because
// that is the state of every statement in the corpus the moment before it is
// first assigned a tag, and assembly has to work then too.
func Apply(body string, lookup map[string]Tag) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		m := HeadingRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		head, _, ok := strings.Cut(line, " {#")
		if !ok {
			continue
		}
		lines[i] = head + " " + attr(m[1], lookup[m[1]])
	}
	return strings.Join(lines, "\n")
}

// attr is the Pandoc attribute block a statement heading ends in. The order of
// what is in it is fixed: the identifier, then the class, then the tag.
func attr(label string, tag Tag) string {
	if tag == "" {
		return "{#" + label + " .statement}"
	}
	return "{#" + label + " .statement tag=" + string(tag) + "}"
}
