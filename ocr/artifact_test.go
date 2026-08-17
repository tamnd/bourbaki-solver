package ocr

import "testing"

func TestCleanTakesOffTheLineTheTransportWrote(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want string
	}{
		{
			"the note as the tool writes it",
			"<!-- https://chatgpt.com/?temporary-chat=true -->\n## 1. THUẬT NGỮ\n\nMột hợp thành.",
			"## 1. THUẬT NGỮ\n\nMột hợp thành.",
		},
		{
			"the note with a blank line after it",
			"<!-- https://chatgpt.com/?temporary-chat=true -->\n\nMột hợp thành.",
			"\nMột hợp thành.",
		},
		{
			"a saved conversation rather than a temporary one",
			"<!-- https://chatgpt.com/c/6a7af1d4-31e0-83ec-afd1-608a29c56c91 -->\nMột hợp thành.",
			"Một hợp thành.",
		},
		{
			"an answer with nothing to take off",
			"Một hợp thành.",
			"Một hợp thành.",
		},
		// A comment the model wrote is the model's, and the audit has a rule
		// that says so. This one only knows about the head of the answer.
		{
			"a comment further down",
			"Một hợp thành.\n\n<!-- https://chatgpt.com/?temporary-chat=true -->",
			"Một hợp thành.\n\n<!-- https://chatgpt.com/?temporary-chat=true -->",
		},
		// Whatever a page of the book holds, it does not hold this, and a
		// comment about anything else is left where it is.
		{
			"a comment that is not the transport's",
			"<!-- a note somebody left in the source -->\nMột hợp thành.",
			"<!-- a note somebody left in the source -->\nMột hợp thành.",
		},
	} {
		if got := clean(c.in); got != c.want {
			t.Errorf("%s: clean() = %q, want %q", c.name, got, c.want)
		}
	}
}
