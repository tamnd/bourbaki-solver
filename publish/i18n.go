package publish

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The words of the site itself, as against the words of the books.
//
// A reader who opens the Vietnamese should not be handed an English frame
// around it, so the chrome is said in the language of the page. The books are
// in whatever language the corpus holds them in; this is only the buttons, the
// headings of the lists and the line that says where a page came from.
//
// en-mt is English, a machine's English of a French volume, so it is said in
// English. A language with no table of its own falls back to English.

type ui map[string]string

var uiText = map[string]ui{
	"en": {
		"copy": "Copy as Markdown", "copied": "Copied",
		"ix.title": "Index of statements", "ix.lede": "The %d statements of %s, by chapter and §. Pick a kind, filter by a word, or put the most cited first.",
		"ix.all": "All", "ix.filter": "Filter, e.g. compact", "ix.cited": "Most cited first", "ix.most": "Most cited statements",
		"ix.all.lede": "Every Book's statements by kind, and the statements the rest of the treatise cites most. Each count is read off the cross-references Bourbaki writes into the prose.",
		"library":     "Library", "search": "Search", "tags": "Tags", "reports": "Reports", "about": "About",
		"skip": "Skip to the text", "find": "Search statements, exercises, tags",
		"contents": "Contents", "onpage": "On this page", "top": "Back to top ↑",
		"prev": "Previous", "next": "Next", "chapter": "Chapter", "book": "Book",
		"books": "The Books", "booksN": "books", "exercises": "Exercises", "statements": "statements",
		"exercisesN": "exercises", "sections": "sections", "chapters": "chapters",
		"provenance": "Provenance", "theme": "Light or dark", "lang": "Language",
		"front": "Front matter", "back": "Indexes", "read": "Start reading",
		"mt":      "translated from the French",
		"tagline": "Nicolas Bourbaki’s treatise, set as a searchable, citable text, with a permanent tag on every numbered statement.",
		"pick":    "Read in", "stmts": "tagged statements", "pages": "printed pages", "langs": "languages",
		"notheld": "Not held in this language", "alsoin": "Also in",
		"exfor": "Exercises for", "exercise": "Exercise", "appendix": "Appendix", "solution": "solution",
		"personal": "A study edition, not affiliated with Bourbaki or Springer.",
		"cite":     "Cite as", "tag": "Tag",
		"p.printed": "Transcribed from the printed volume. Not translated.",
		"p.edition": "Transcribed from the printed edition, %s. Not translated.",
		"p.mt":      "Translated with machine help", "p.unchecked": ", not yet reviewed by a person.",
		"p.gt": "A rough draft by Google Translate, low quality. Terms follow the glossary, but the sentences can be wrong; a careful translation will replace it.",
		"s.ph": "a word, or a tag", "s.go": "Search",
		"s.intro": "Search runs in your browser over every statement and exercise. Nothing you type is sent anywhere.",
		"s.hint":  "Type a word, or a tag such as 00QO.", "s.loading": "Loading the index.",
		"s.failed": "The index did not load.", "s.one": "1 result.", "s.many": "{n} results.",
		"s.first": "{n} results, the first 50 shown.", "s.partial": "partial",
		"x.citedby": "Cited by", "x.cites": "Cites", "x.nothing": "Nothing.", "x.solution": "Solution",
		"x.where":   "Bourbaki sets these at the end of %s.",
		"x.none":    "%d exercises. No solution has been written for any of them yet.",
		"x.some":    "%d exercises, %d of them with a solution. The solutions are not Bourbaki's and have not been checked by a person.",
		"x.nosol":   "No solution has been written yet.",
		"x.tagline": "Tag %s, permanent. Cite it as %s.",
		"x.caveat":  "Bourbaki puts the proof directly after the statement, so what is above is the statement and its proof together.",
	},
	"fr": {
		"copy": "Copier en Markdown", "copied": "Copié",
		"ix.title": "Index des énoncés", "ix.lede": "Les %d énoncés de %s, par chapitre et paragraphe. Choisissez un type, filtrez par un mot, ou mettez les plus cités en tête.",
		"ix.all": "Tous", "ix.filter": "Filtrer, p. ex. compact", "ix.cited": "Les plus cités d’abord", "ix.most": "Énoncés les plus cités",
		"ix.all.lede": "Les énoncés de chaque Livre par type, et ceux que le reste du traité cite le plus. Chaque nombre est lu dans les renvois que Bourbaki écrit dans le texte.",
		"library":     "Bibliothèque", "search": "Recherche", "tags": "Étiquettes", "reports": "Rapports", "about": "À propos",
		"skip": "Aller au texte", "find": "Chercher énoncés, exercices, étiquettes",
		"contents": "Table des matières", "onpage": "Sur cette page", "top": "Haut de page ↑",
		"prev": "Précédent", "next": "Suivant", "chapter": "Chapitre", "book": "Livre",
		"books": "Les Livres", "booksN": "livres", "exercises": "Exercices", "statements": "énoncés",
		"exercisesN": "exercices", "sections": "paragraphes", "chapters": "chapitres",
		"provenance": "Provenance", "theme": "Clair ou sombre", "lang": "Langue",
		"front": "Préliminaires", "back": "Index", "read": "Commencer la lecture",
		"mt":      "traduit du français",
		"tagline": "Le traité de Nicolas Bourbaki, en texte cherchable et citable, avec une étiquette permanente sur chaque énoncé numéroté.",
		"pick":    "Lire en", "stmts": "énoncés étiquetés", "pages": "pages imprimées", "langs": "langues",
		"notheld": "Absent dans cette langue", "alsoin": "Aussi en",
		"exfor": "Exercices du", "exercise": "Exercice", "appendix": "Annexe", "solution": "solution",
		"personal": "Une édition d’étude, sans lien avec Bourbaki ni Springer.",
		"cite":     "Citer comme", "tag": "Étiquette",
		"p.printed": "Transcrit du volume imprimé. Non traduit.",
		"p.edition": "Transcrit de l’édition imprimée, %s. Non traduit.",
		"p.mt":      "Traduit avec l’aide d’une machine", "p.unchecked": ", pas encore relu par une personne.",
		"p.gt": "Un brouillon de Google Traduction, de faible qualité. Les termes suivent le glossaire, mais les phrases peuvent être fausses ; une traduction soignée le remplacera.",
		"s.ph": "un mot, ou une étiquette", "s.go": "Chercher",
		"s.intro": "La recherche se fait dans le navigateur, sur un index des énoncés et des exercices. Rien n’est envoyé.",
		"s.hint":  "Tapez un mot, ou une étiquette comme 00QO.", "s.loading": "Chargement de l’index.",
		"s.failed": "L’index n’a pas pu être chargé.", "s.one": "1 résultat.", "s.many": "{n} résultats.",
		"s.first": "{n} résultats, les 50 premiers affichés.", "s.partial": "partiel",
		"x.citedby": "Cité par", "x.cites": "Cite", "x.nothing": "Rien.", "x.solution": "Solution",
		"x.where":   "Bourbaki les place à la fin de %s.",
		"x.none":    "%d exercices. Aucune solution n’a encore été écrite.",
		"x.some":    "%d exercices, dont %d avec une solution. Les solutions ne sont pas de Bourbaki et n’ont pas été vérifiées par une personne.",
		"x.nosol":   "Aucune solution n’a encore été écrite.",
		"x.tagline": "Étiquette %s, permanente. À citer comme %s.",
		"x.caveat":  "Bourbaki place la démonstration juste après l’énoncé : ce qui précède est l’énoncé avec sa démonstration.",
	},
	"vi": {
		"copy": "Sao chép Markdown", "copied": "Đã sao chép",
		"ix.title": "Chỉ mục mệnh đề", "ix.lede": "%d mệnh đề của %s, theo chương và mục. Chọn một loại, lọc theo từ, hoặc xếp những mệnh đề được trích dẫn nhiều nhất lên đầu.",
		"ix.all": "Tất cả", "ix.filter": "Lọc, ví dụ compact", "ix.cited": "Trích dẫn nhiều nhất trước", "ix.most": "Các mệnh đề được trích dẫn nhiều nhất",
		"ix.all.lede": "Các mệnh đề của mỗi Quyển theo loại, và những mệnh đề được phần còn lại của bộ sách trích dẫn nhiều nhất. Mỗi con số được đọc từ các tham chiếu chéo Bourbaki viết trong văn bản.",
		"library":     "Thư viện", "search": "Tìm kiếm", "tags": "Nhãn", "reports": "Báo cáo", "about": "Giới thiệu",
		"skip": "Chuyển đến nội dung", "find": "Tìm mệnh đề, bài tập, nhãn",
		"contents": "Mục lục", "onpage": "Trên trang này", "top": "Lên đầu trang ↑",
		"prev": "Trước", "next": "Tiếp", "chapter": "Chương", "book": "Quyển",
		"books": "Các Quyển", "booksN": "quyển", "exercises": "Bài tập", "statements": "mệnh đề",
		"exercisesN": "bài tập", "sections": "mục", "chapters": "chương",
		"provenance": "Nguồn gốc", "theme": "Sáng hoặc tối", "lang": "Ngôn ngữ",
		"front": "Phần đầu", "back": "Chỉ mục", "read": "Bắt đầu đọc",
		"mt":      "dịch từ tiếng Pháp",
		"tagline": "Bộ chuyên luận của Nicolas Bourbaki, dưới dạng văn bản tra cứu và trích dẫn được, mỗi mệnh đề đánh số có một nhãn cố định.",
		"pick":    "Đọc bằng", "stmts": "mệnh đề có nhãn", "pages": "trang in", "langs": "ngôn ngữ",
		"notheld": "Chưa có trong ngôn ngữ này", "alsoin": "Cũng có bằng",
		"exfor": "Bài tập của", "exercise": "Bài tập", "appendix": "Phụ lục", "solution": "lời giải",
		"personal": "Một ấn bản để học, không liên quan đến Bourbaki hay Springer.",
		"cite":     "Trích dẫn", "tag": "Nhãn",
		"p.printed": "Chép lại từ bản in. Không dịch.",
		"p.edition": "Chép lại từ ấn bản in, %s. Không dịch.",
		"p.mt":      "Dịch với sự hỗ trợ của máy", "p.unchecked": ", chưa được người đọc duyệt lại.",
		"p.gt": "Bản nháp thô của Google Dịch, chất lượng thấp. Thuật ngữ theo bảng thuật ngữ, nhưng câu văn có thể sai; một bản dịch cẩn thận sẽ thay thế nó.",
		"s.ph": "một từ, hoặc một nhãn", "s.go": "Tìm",
		"s.intro": "Việc tìm kiếm chạy ngay trong trình duyệt, trên chỉ mục các mệnh đề và bài tập. Không có gì được gửi đi.",
		"s.hint":  "Gõ một từ, hoặc một nhãn như 00QO.", "s.loading": "Đang tải chỉ mục.",
		"s.failed": "Không tải được chỉ mục.", "s.one": "1 kết quả.", "s.many": "{n} kết quả.",
		"s.first": "{n} kết quả, hiển thị 50 kết quả đầu.", "s.partial": "chưa đủ",
		"x.citedby": "Được trích dẫn bởi", "x.cites": "Trích dẫn đến", "x.nothing": "Không có.", "x.solution": "Lời giải",
		"x.where":   "Bourbaki đặt các bài này ở cuối %s.",
		"x.none":    "%d bài tập. Chưa có lời giải nào được viết.",
		"x.some":    "%d bài tập, %d bài có lời giải. Các lời giải không phải của Bourbaki và chưa được người kiểm tra.",
		"x.nosol":   "Chưa có lời giải nào được viết.",
		"x.tagline": "Nhãn %s, cố định. Trích dẫn là %s.",
		"x.caveat":  "Bourbaki đặt chứng minh ngay sau mệnh đề, nên phần trên là mệnh đề cùng với chứng minh.",
	},
}

// uiLang is the table a page's language is said in.
func uiLang(lang string) string {
	if _, ok := uiText[lang]; ok {
		return lang
	}
	return "en"
}

// T is one word of the frame in a language.
func T(lang, key string) string {
	if v, ok := uiText[uiLang(lang)][key]; ok {
		return v
	}
	return uiText["en"][key]
}

// nativeName is what a language calls itself, which is what a switcher offers.
var nativeName = map[string]string{
	"en": "English", "fr": "Français", "vi": "Tiếng Việt", "en-mt": "English (translated)",
	"zh": "中文", "ja": "日本語",
}

func native(lang string) string {
	if n := nativeName[lang]; n != "" {
		return n
	}
	return lang
}

// bookInfo is one of the twelve Books, in the order Bourbaki numbers them, with
// its name in each language of the frame. The corpus front matter carries the
// English name in every language so that the manifests join, which is right for
// the manifests and wrong for a reader of the French.
type bookInfo struct {
	ID    string
	Abbr  string // how Bourbaki abbreviates the Book in a reference
	Names map[string]string
}

var bookList = []bookInfo{
	{"ens", "E", map[string]string{"en": "Theory of Sets", "fr": "Théorie des ensembles", "vi": "Lý thuyết tập hợp"}},
	{"alg", "A", map[string]string{"en": "Algebra", "fr": "Algèbre", "vi": "Đại số"}},
	{"top", "TG", map[string]string{"en": "General Topology", "fr": "Topologie générale", "vi": "Tô pô đại cương"}},
	{"fvr", "FVR", map[string]string{"en": "Functions of a Real Variable", "fr": "Fonctions d’une variable réelle", "vi": "Hàm số một biến thực"}},
	{"evt", "EVT", map[string]string{"en": "Topological Vector Spaces", "fr": "Espaces vectoriels topologiques", "vi": "Không gian vectơ tô pô"}},
	{"int", "INT", map[string]string{"en": "Integration", "fr": "Intégration", "vi": "Tích phân"}},
	{"ac", "AC", map[string]string{"en": "Commutative Algebra", "fr": "Algèbre commutative", "vi": "Đại số giao hoán"}},
	{"var", "VAR", map[string]string{"en": "Differentiable and Analytic Manifolds", "fr": "Variétés différentielles et analytiques", "vi": "Đa tạp khả vi và giải tích"}},
	{"lie", "LIE", map[string]string{"en": "Lie Groups and Lie Algebras", "fr": "Groupes et algèbres de Lie", "vi": "Nhóm Lie và đại số Lie"}},
	{"ts", "TS", map[string]string{"en": "Spectral Theories", "fr": "Théories spectrales", "vi": "Các lý thuyết phổ"}},
	{"ta", "TA", map[string]string{"en": "Algebraic Topology", "fr": "Topologie algébrique", "vi": "Tô pô đại số"}},
	{"hist", "HM", map[string]string{"en": "Elements of the History of Mathematics", "fr": "Éléments d’histoire des mathématiques", "vi": "Các yếu tố của lịch sử toán học"}},
}

// bookRank is where a Book stands in Bourbaki's order, with a Book the list
// does not know after all of them.
func bookRank(id string) int {
	for i, b := range bookList {
		if b.ID == id {
			return i
		}
	}
	return len(bookList)
}

// bookName is a Book's name in the language of a page, falling back to the
// name the corpus gave it.
func bookName(lang, id, fallback string) string {
	for _, b := range bookList {
		if b.ID == id {
			if n := b.Names[uiLang(lang)]; n != "" {
				return n
			}
			return b.Names["en"]
		}
	}
	return fallback
}

// chapterWord is "Chapter I" in the language of the page. The History has no
// chapters and a chapter of it is a part.
func chapterWord(lang, chapter string) string {
	if chapter == "" {
		return ""
	}
	return T(lang, "chapter") + " " + chapter
}

// romanValue reads a chapter name as a number, so that IX sorts after VIII and
// not after I. A chapter that is not a numeral sorts after the ones that are.
func romanValue(s string) int {
	if n, ok := atoiOK(s); ok {
		return n
	}
	vals := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100}
	total, prev := 0, 0
	rs := []rune(strings.ToUpper(s))
	for i := len(rs) - 1; i >= 0; i-- {
		v, ok := vals[rs[i]]
		if !ok {
			return 1000
		}
		if v < prev {
			total -= v
		} else {
			total += v
			prev = v
		}
	}
	return total
}

func atoiOK(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// keepCaps are the names that stay capitalised when a heading the printer set
// in capitals is brought down to a sentence.
var keepCaps = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`lie hilbert banach haar coxeter tits galois noether dedekind krull weyl
	cartan killing fourier riemann lebesgue stieltjes borel radon hausdorff baire stone weierstrass zorn
	euclide euclid jacobson artin hensel poincaré witt clifford grassmann gelfand euler taylor newton cauchy
	abel jordan hölder sylow schur morita kähler zariski laurent laplace fubini stokes frobenius chevalley
	harish-chandra hopf iwasawa levi engel lagrange gauss schwartz mackey peter plancherel pontryagin
	bernoulli stirling bessel legendre dirichlet heine cantor darboux kronecker hamilton dynkin bruhat
	casimir campbell hausdorff baker steinberg demazure weil bourbaki`) {
		keepCaps[w] = true
	}
}

// sentence brings a heading printed in capitals down to how a sentence is
// written. A heading already in mixed case is left as it is.
func sentence(s string) string { return sentenceIn("", s) }

// sentenceIn is sentence for a heading in a known language. Vietnamese has
// words spelled like numerals, vi and xi among them, so it keeps one only
// where it counts something: Chương IV.
func sentenceIn(lang, s string) string {
	s = strings.TrimSpace(s)
	letters, upper := 0, 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	if letters == 0 || upper*10 < letters*8 {
		return s
	}
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		core := strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && r != '-' })
		if keepCaps[core] || i == 0 || strings.HasSuffix(words[max(i-1, 0)], ".") {
			words[i] = upperFirst(w)
		}
		// Roman numerals stay numerals.
		if core != "" && romanRE.MatchString(core) && (lang != "vi" || i > 0 && viCounted[strings.ToLower(strings.Trim(words[i-1], ".,:;"))]) {
			words[i] = strings.ToUpper(w)
		}
	}
	return strings.Join(words, " ")
}

// viCounted are the Vietnamese words a roman numeral is the number of.
var viCounted = map[string]bool{"chương": true, "phần": true, "quyển": true, "tập": true, "mục": true}

var romanRE = regexp.MustCompile(`^(x{0,3})(ix|iv|v?i{0,3})$`)

func upperFirst(w string) string {
	for i, r := range w {
		if unicode.IsLetter(r) {
			_, n := utf8.DecodeRuneInString(w[i:])
			return w[:i] + strings.ToUpper(w[i:i+n]) + w[i+n:]
		}
	}
	return w
}
