package book

import (
	"strings"
	"testing"
)

// An image link whose target holds a space is not what Markdown says and is
// what this corpus has, in ten places. What made it worth a test is that an
// image the pattern misses is not left alone: the brackets go through as prose
// and the underscores in the target reach TeX as subscripts, which is a fatal
// error on a file name. It stopped both editions of the Vietnamese Lie IV-VI
// and it stopped them at page 140 of 314, so most of the book was missing and
// the only sign of it was tectonic's exit status.

func TestAFigureIsSetFromItsAltTextHoweverItsTargetIsSpelled(t *testing.T) {
	var r Renderer
	for _, tc := range []struct{ name, in string }{
		// The tenth link: the translation translated the file name, and the
		// space it introduced is between the two underscores that TeX choked on.
		{"a target with a space and two underscores",
			`![Eleven Coxeter graphs of hạng 4](../images/coxeter_graphs_hạng_4_hệ số_6.png)`},
		// The other nine: a caption standing where the target goes. These are in
		// the English and the French too, so they came out of the reading rather
		// than out of a translation.
		{"a caption where the target goes", `![Eleven Coxeter graphs](Fig. 2)`},
		{"an ordinary target", `![Graphes de Coxeter](../images/134_1.png)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := r.TeX(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(out, `\bfigure{`) {
				t.Errorf("did not become a figure:\n  %s\n  -> %q", tc.in, out)
			}
			// The two characters that make an unmatched link fatal. Neither can
			// survive into the document, because the target is thrown away and
			// only the alt text is set.
			if strings.ContainsAny(out, "_^") {
				t.Errorf("the target reached the document, where _ and ^ are "+
					"subscripts and superscripts: %q", out)
			}
			if strings.Contains(out, "!") {
				t.Errorf("the exclamation mark was left standing as prose: %q", out)
			}
		})
	}
}
