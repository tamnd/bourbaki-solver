package publish

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/tamnd/bourbaki-solver/quality"
)

// The front page and /about/. The numbers on both are counted out of the
// corpus at build time, so they cannot drift from what the site holds.

const (
	corpusRepo = "https://github.com/tamnd/bourbaki"
	solverRepo = "https://github.com/tamnd/bourbaki-solver"
)

// langName is what to call a language in a sentence.
func langName(code string) string {
	if code == "en-mt" {
		return "English, translated"
	}
	return quality.LangName(code)
}

// langNumbers is one row of the table on /about/.
type langNumbers struct {
	Lang       string
	Sections   int
	Statements int
	Exercises  int
	Translated bool
}

// numbers counts what this build loaded, once, for the front page and /about/.
func (s *Site) numbers() []langNumbers {
	by := map[string]*langNumbers{}
	for _, lang := range s.Langs {
		by[lang] = &langNumbers{Lang: lang}
	}
	for _, sec := range s.Sections {
		n := by[sec.Lang]
		if n == nil {
			continue
		}
		n.Sections++
		n.Statements += sec.Meta.Statements
		if sec.Meta.TranslatedFrom != "" {
			n.Translated = true
		}
	}
	for _, ex := range s.Exercises {
		if n := by[ex.Lang]; n != nil {
			n.Exercises++
		}
	}
	var out []langNumbers
	for _, lang := range s.Langs {
		out = append(out, *by[lang])
	}
	return out
}

// made is what the table says about where a language's text comes from.
func (n langNumbers) made() string {
	switch {
	case n.Lang == "fr":
		return "Bourbaki’s original French"
	case n.Lang == "en":
		return "the published English translation"
	case n.Lang == "en-mt":
		return "our English for the Books never published in English, not yet reviewed"
	case n.Translated:
		return "our translation, not yet reviewed"
	}
	return "transcribed from the printed volumes"
}

func (s *Site) about() (string, error) {
	var b strings.Builder
	b.WriteString("<h1>About</h1>\n")
	b.WriteString("<p>This is a reading edition of Nicolas Bourbaki’s <em>Éléments de mathématique</em>, " +
		"made for study. The aim is simple: to be able to open any Book at any section, search it, follow a " +
		"reference to the result it points at, and cite a theorem in a way that will still make sense years " +
		"from now.</p>\n")
	b.WriteString("<p>It is a personal project. It is not affiliated with Bourbaki, Hermann or Springer, and " +
		"it is no substitute for the books themselves. If you find it useful, buy them.</p>\n")

	b.WriteString("<h2>What is here</h2>\n")
	b.WriteString("<table class=\"coverage\">\n<tr><th>language</th><th>sections</th><th>statements</th>" +
		"<th>exercises</th><th></th></tr>\n")
	for _, n := range s.numbers() {
		note := n.made()
		if s.Draft[n.Lang] {
			note += "; only partly done so far"
		}
		fmt.Fprintf(&b, "<tr><td><a href=%q>%s</a> <span class=\"count\">%s</span></td>"+
			"<td>%d</td><td>%d</td><td>%d</td><td>%s</td></tr>\n",
			s.url(n.Lang), n.Lang, langName(n.Lang), n.Sections, n.Statements, n.Exercises, note)
	}
	b.WriteString("</table>\n")
	if s.drafted() {
		b.WriteString("<p>A language that is only partly done is not offered in the language menu on " +
			"each page, so you are not sent to a page that does not exist yet. It is linked from here and " +
			"from the front page instead.</p>\n")
	}

	b.WriteString("<h2>How to use it</h2>\n<ul>\n")
	b.WriteString("<li>Every numbered statement, a definition, a proposition, a theorem, a lemma, a " +
		"corollary, a remark or an example, has its own page, and so does every exercise.</li>\n")
	b.WriteString("<li>References in the text are links. Hover over one to see the statement it points " +
		"at, and each statement lists the places that cite it.</li>\n")
	fmt.Fprintf(&b, "<li>Each Book has a <a href=%q>statement index</a>, by kind and by chapter, with the "+
		"results the rest of the treatise leans on most.</li>\n", s.url("en", "statements"))
	fmt.Fprintf(&b, "<li><a href=%q>Search</a> runs in your browser over every statement and exercise.</li>\n",
		s.url("search"))
	b.WriteString("<li>Any page can be copied as Markdown, mathematics included, for your own notes.</li>\n")
	b.WriteString("</ul>\n")

	b.WriteString("<h2>Citing</h2>\n")
	fmt.Fprintf(&b, "<p>Bourbaki renumbers between editions, so every statement and exercise here also "+
		"carries a permanent four character tag, in the spirit of the Stacks project. A tag is given once "+
		"and never reused, so a citation such as <code>[Bourbaki, Tag 0AZ1]</code> keeps pointing at the same "+
		"result. Type a tag into the box on the <a href=%q>tags page</a> to go straight to it, or browse the "+
		"tags Book by Book.</p>\n", s.url("tags"))

	b.WriteString("<h2>Mistakes</h2>\n")
	fmt.Fprintf(&b, "<p>The text was transcribed and laid out with the help of software, and the "+
		"translations that are marked as not yet reviewed have not been read through by a person. Formulas "+
		"can come out wrong and a statement can be cut in the wrong place. When something matters, check "+
		"it against the printed volume, and if you find an error, please <a href=%q>report it</a>.</p>\n",
		corpusRepo+"/issues")

	b.WriteString("<h2>Copyright</h2>\n")
	fmt.Fprintf(&b, "<p><em>Éléments de mathématique</em> is copyright N. Bourbaki and its publishers, "+
		"Hermann, Masson, Springer and Springer Nature, and nothing here changes that. The transcriptions "+
		"and translations are for personal study and not for commercial use. No scan of the books is "+
		"kept or served here. The <a href=%q>text</a> and the <a href=%q>code that builds this site</a> are "+
		"on GitHub; the code is under the MIT licence.</p>\n", corpusRepo, solverRepo)

	return s.render(layout{Title: "About", Here: "about", Content: template.HTML(b.String())})
}
