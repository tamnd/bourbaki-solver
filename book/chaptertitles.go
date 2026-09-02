package book

// The chapter title, per Book and chapter numeral, per language, for the
// chapters that have no title anywhere in the corpus to read.
//
// Almost none of them are in that state, which is the point of this table being
// three rows long rather than ninety. A translated file carries its own title as
// a heading on its body, and loadSection reads the chapter title off the front
// of a chapter the same way it reads a section title off a section. What is left
// here is the two Books that have no chapter front file at all: Elements of the
// History of Mathematics and the Differentiable and Analytic Manifolds fascicle
// are each a single chapter whose sections sit directly under the chapter
// directory, so there is no front matter file for the title to be written on and
// nothing for the loader to recover. Those have to be written down, the same way
// the volume titles in titles.go are.
//
// Both are single chapter volumes, so the chapter title is the volume title, and
// the wording here is the wording titles.go already uses for the cover rather
// than a second rendering of the same words. A volume whose cover and whose
// chapter opening page disagree about its own name is worse than either wording
// on its own.
//
// A language missing from a row falls through to the corpus, which is right for
// content/en and content/fr, where chapter_title is already in the language
// being built.
var chapterTitles = map[string]map[string]string{
	"hist/1": {"vi": "Các yếu tố lịch sử toán học"},
	"var/1": {
		"en": "Differentiable and analytic manifolds, fascicle of results",
		"vi": "Đa tạp khả vi và giải tích, tập kết quả",
	},
}

// chapterTitle is the title of one chapter in one language. It prefers the
// table, and falls back to whatever the loader worked out from the corpus.
func chapterTitle(book, numeral, lang, corpusTitle string) string {
	if byLang, ok := chapterTitles[book+"/"+numeral]; ok {
		if t := byLang[lang]; t != "" {
			return t
		}
	}
	return corpusTitle
}
