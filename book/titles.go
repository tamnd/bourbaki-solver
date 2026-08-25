package book

import (
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The volume title, per Book, per language.
//
// This is a table and not a field read out of the corpus, which took a build to
// work out. book_title is on every section file, and in content/en it holds the
// short title the cover has, "Algebra" and not "Algebra I, Chapters 1-3", which
// is exactly what a cover wants because the chapter line is set under it and
// composed separately. So the first Vietnamese build read book_title, and put
// "Algebra" on the cover of a Vietnamese volume.
//
// The reason is that the pipeline translates the body of a file and copies the
// front matter across unchanged. book_title is English in every one of the 331
// files of content/vi and in most of content/fr as well. The section and chapter
// titles are recoverable, because a translated file opens with its own title as
// a heading and loadSection reads it off the body. The volume title is not: it
// appears nowhere in the text of the volume. It has to be written down.
//
// The English column is already written down, in corpus.BookTitle, so it is not
// repeated here. What is here is the two languages that table has no room for.
// The French is off the printed covers. The Vietnamese is the corpus's own
// wording, counted rather than chosen: content/vi writes "Tôpô đại cương" 52
// times against 5 for "Tôpô tổng quát", "Không gian vectơ tôpô" 221 times and
// "Không gian vector tôpô" never, and "Hàm của một biến thực" 8 times while
// "Hàm số một biến thực" does not appear at all. Where the corpus has settled on
// a wording, that wording is what is here.
//
// Four Books have no Vietnamese in the corpus at all, and hist, ta, ts and var
// are given a Vietnamese title anyway. It costs a line each and it means that
// the day the first chapter of one of them is translated, the cover is right
// rather than in English.
var titles = map[string]map[string]string{
	"ac": {
		"fr": "Algèbre commutative",
		"vi": "Đại số giao hoán",
	},
	"alg": {
		"fr": "Algèbre",
		"vi": "Đại số",
	},
	"ens": {
		"fr": "Théorie des ensembles",
		"vi": "Lý thuyết tập hợp",
	},
	"evt": {
		"fr": "Espaces vectoriels topologiques",
		"vi": "Không gian vectơ tôpô",
	},
	"fvr": {
		"fr": "Fonctions d'une variable réelle",
		"vi": "Hàm của một biến thực",
	},
	"hist": {
		"fr": "Éléments d'histoire des mathématiques",
		"vi": "Các yếu tố lịch sử toán học",
	},
	"int": {
		"fr": "Intégration",
		"vi": "Tích phân",
	},
	"lie": {
		"fr": "Groupes et algèbres de Lie",
		"vi": "Nhóm Lie và đại số Lie",
	},
	"ta": {
		"fr": "Topologie algébrique",
		"vi": "Tôpô đại số",
	},
	"top": {
		"fr": "Topologie générale",
		"vi": "Tôpô đại cương",
	},
	"ts": {
		"fr": "Théories spectrales",
		"vi": "Lý thuyết phổ",
	},
	"var": {
		"fr": "Variétés différentielles et analytiques",
		"vi": "Đa tạp khả vi và giải tích",
	},
}

// listed is a title as the contents sets it, which is small caps, out of a title
// as the corpus has it, which is capitals.
//
// The two are the same words, and the difference between them is only where the
// tall letters fall: the printing lists chapter II as "CHAPTER II. LINEAR
// ALGEBRA" with the C, the I, the L and the A a size up and the rest of each
// word a size down. Feeding the corpus's ALGEBRAIC STRUCTURES straight to
// \scshape gives capitals throughout, which is a heavier line than the page has.
//
// Getting the case wrong inside a word costs nothing here, which is what makes
// this safe where the same trick on a subsection title is not. Small caps sets a
// lower case letter as a small capital, so "algébres" and "Algébres" set
// identically apart from the height of the A. Only the first letter of each word
// is visible as a decision, and the particles that should not take one are a
// closed list in each of the three languages.
// Math is left exactly as it stands. A subsection of Commutative Algebra is
// called "Properties of the ring $\mathbf{A}^{(d)}$", and a title caser that ran
// over the whole string would set \Gamma as \gamma and turn a group into a
// function. So the string is cut at the dollars first and only the prose between
// them is touched.
func listed(s, lang string) string {
	// An odd number of dollars is a title whose math nobody closed. There are a
	// few in the corpus and the audit reports them. Cutting on the dollars would
	// case the tail of such a title as prose and set a \Gamma as a \gamma, so it
	// is left alone and printed as it stands.
	if strings.Count(s, "$")%2 != 0 {
		return s
	}
	var b strings.Builder
	for i, part := range strings.Split(s, "$") {
		if i%2 == 1 {
			b.WriteString("$" + part + "$")
			continue
		}
		b.WriteString(listedProse(part, lang, i == 0))
	}
	return b.String()
}

// listedProse title cases one run of prose. first says whether it opens the
// title, because the word that opens a title takes a capital even when it is one
// of the particles that would not take one anywhere else.
func listedProse(s, lang string, first bool) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	lead, trail := leading(s), trailing(s)
	words := strings.Fields(s)
	for i, w := range words {
		w = lowerProse(w)
		words[i] = w
		if !(first && i == 0) && particles[lang][w] {
			continue
		}
		rs := []rune(w)
		if !unicode.Is(unicode.Greek, rs[0]) {
			rs[0] = unicode.ToUpper(rs[0])
		}
		words[i] = string(rs)
	}
	return lead + strings.Join(words, " ") + trail
}

// lowerProse lower cases a word and leaves its Greek where it is.
//
// A Greek letter in one of these titles is a symbol and not a word. Thirty
// three of them are in manifests/toc/ standing outside a math span, written as
// letters by the reading of the contents page: a subsection of Algebra VIII is
// listed as "τ -Extensions of Groups" where the printing sets a tau. Casing
// them is not a cosmetic error. It turns tau into cap tau, which is a different
// character that Latin Modern has no glyph for, and it turns cap theta into
// theta, which is a different symbol that sets perfectly well and says
// something the book does not.
//
// The right place for the rest of that fix is the manifest, since a symbol
// belongs in a math span, and it is being done volume by volume. This is the
// guard that stops the build mangling the ones that are still there, and it
// stays afterwards, because the next reading of a contents page will do the
// same thing.
func lowerProse(w string) string {
	rs := []rune(w)
	for i, r := range rs {
		if !unicode.Is(unicode.Greek, r) {
			rs[i] = unicode.ToLower(r)
		}
	}
	return string(rs)
}

// leading and trailing keep the space either side of a math span, which Fields
// would eat and which is the difference between "the ring $A$" and "the ring$A$".
func leading(s string) string {
	if t := strings.TrimLeft(s, " \t"); t != s && t != "" {
		return " "
	}
	return ""
}

func trailing(s string) string {
	if t := strings.TrimRight(s, " \t"); t != s && t != "" {
		return " "
	}
	return ""
}

// particles are the words a title leaves in lower case. The English is off the
// printed front matter of Algebra I to III, which lists "To THE READER" with the
// article a size down, and the French and the Vietnamese follow the same rule
// their own printings use.
var particles = map[string]map[string]bool{
	"en": {"a": true, "an": true, "and": true, "for": true, "in": true,
		"of": true, "on": true, "the": true, "to": true, "with": true},
	"fr": {"à": true, "au": true, "aux": true, "de": true, "des": true,
		"du": true, "en": true, "et": true, "la": true, "le": true,
		"les": true, "sur": true, "un": true, "une": true},
	"vi": {"của": true, "và": true, "các": true, "một": true, "trên": true,
		"trong": true, "cho": true, "với": true},
}

// title is the volume title for one Book in one language.
//
// English, and any language nobody has written a column for, come from
// corpus.BookTitle, which is the name of the Book and not of the volume and so
// carries no chapter span. A Book that table has not got either falls back to
// the manifest entry, which is the title of the printing and does carry the
// span: "Algebra I, Chapters 1-3" under a chapter line that says Chapters 1 to 3
// again. That reads as a complaint, which is the point. It is only reached by a
// Book nobody has named in either table, and a cover that repeats itself is
// easier to notice than a cover that is quietly in the wrong language.
func title(book, lang, manifest string) string {
	if byLang, ok := titles[book]; ok {
		if t := byLang[lang]; t != "" {
			return t
		}
	}
	if t := corpus.BookTitle(book); t != book {
		return t
	}
	return manifest
}
