package book

import (
	"regexp"
	"testing"
)

// The five words this build writes that did not come out of the corpus are
// written down twice, once in the class for the PDF and once in bookWords for
// the EPUB, and there is no way to write them once: one of the two files is TeX
// and the other is Go. Two copies of a table that nobody checks is two copies
// that drift, and the way they would drift is the quiet way. Somebody adds a
// language to the class for the PDF, the EPUB of that language keeps saying
// CHAPTER in English, and nothing fails because both files are valid and both
// builds finish.
//
// So the test reads the class and holds the Go table to it.
var (
	classDefault = regexp.MustCompile(`(?m)^\\newcommand\{\\b(\w+)name\}\{([^}]*)\}`)
	classBlock   = regexp.MustCompile(`(?s)\\ifx\\btmp\\btmp(\w\w)(.*?)\\fi`)
	classRename  = regexp.MustCompile(`\\renewcommand\{\\b(\w+)name\}\{([^}]*)\}`)
)

// classWords is the five words per language as the class has them: the
// \newcommand defaults are English and each \ifx block overrides them.
func classWords(t *testing.T) map[string]map[string]string {
	t.Helper()
	en := map[string]string{}
	for _, m := range classDefault.FindAllStringSubmatch(Class, -1) {
		if key, ok := classKey[m[1]]; ok {
			en[key] = m[2]
		}
	}
	if len(en) != len(classKey) {
		t.Fatalf("the class defines %d of the %d words in \\newcommand", len(en), len(classKey))
	}
	out := map[string]map[string]string{"en": en}
	for _, b := range classBlock.FindAllStringSubmatch(Class, -1) {
		lang := b[1]
		words := map[string]string{}
		for k, v := range en {
			words[k] = v
		}
		for _, m := range classRename.FindAllStringSubmatch(b[2], -1) {
			if key, ok := classKey[m[1]]; ok {
				words[key] = m[2]
			}
		}
		out[lang] = words
	}
	return out
}

// classKey maps the class's macro names to the keys bookWords uses.
var classKey = map[string]string{
	"chapter":      "chapter",
	"exercises":    "exercises",
	"exercisesfor": "exercisesFor",
	"historical":   "historical",
	"contents":     "contents",
}

func TestBookWordsMatchTheClass(t *testing.T) {
	class := classWords(t)
	if len(class) != len(bookWords) {
		t.Errorf("the class knows %d languages and bookWords knows %d", len(class), len(bookWords))
	}
	for lang, want := range class {
		got, ok := bookWords[lang]
		if !ok {
			t.Errorf("the class sets the words for %q and bookWords has no entry for it", lang)
			continue
		}
		for key, w := range want {
			if got[key] != w {
				t.Errorf("%s %s: the class says %q and bookWords says %q", lang, key, w, got[key])
			}
		}
	}
	for lang := range bookWords {
		if _, ok := class[lang]; !ok {
			t.Errorf("bookWords has %q and the class never sets it, so the PDF of that language is in English", lang)
		}
	}
}

// bookWord falls back to English rather than to an empty string, because a
// heading that is missing is a heading nobody notices and a heading in the
// wrong language is one somebody reports.
func TestBookWordFallsBackToEnglish(t *testing.T) {
	if got := bookWord("zh", "chapter"); got != "CHAPTER" {
		t.Errorf("bookWord for a language with no table gave %q, want CHAPTER", got)
	}
	if got := bookWord("vi", "chapter"); got != "CHƯƠNG" {
		t.Errorf("bookWord vi chapter gave %q", got)
	}
	if got := bookWord("en", "nosuchword"); got != "" {
		t.Errorf("bookWord for a key nothing has gave %q, want the empty string", got)
	}
}

// The words appendixWord and spanWords write are on the same footing as the
// five above: they are the only other sentences the build writes that are not
// in the corpus, and a Vietnamese volume that heads a division "Appendix 2" has
// an English word in it that nothing will ever translate.
func TestEveryLanguageHasItsOwnFurniture(t *testing.T) {
	for lang := range bookWords {
		if appendixWord[lang] == "" {
			t.Errorf("%s has the five class words and no word for an appendix", lang)
		}
		if spanWords[lang] == [3]string{} {
			t.Errorf("%s has the five class words and no words for the chapter span on the cover", lang)
		}
	}
}
