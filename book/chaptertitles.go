package book

// The chapter title, per Book, per chapter numeral, per language.
//
// This is the same problem the volume title has, one level down, and it has the
// same cause. loadChapter takes the chapter title off the chapter_title field of
// the section files, and the pipeline translates the body of a file and copies
// its front matter across unchanged, so chapter_title is the source printing's
// English in all seventy five chapters of content/vi and the source printing's
// French in sixteen of the seventeen in content/en-mt. A Vietnamese volume was
// therefore set with an English chapter title on the chapter opening page, in
// the running head, and on every chapter line of its table of contents, under
// Vietnamese section titles, because those the corpus does carry: a translated
// file opens with its own title as a heading and loadSection reads it off the
// body. The chapter title appears in no body, so like the volume title it has to
// be written down.
//
// The Vietnamese was read on the fleet against the corpus glossary and then
// checked term by term against what content/vi already says, on the same
// principle the volume titles use: where the corpus has settled on a wording,
// that wording is what is here. Thirteen came back needing a change. Measure is
// do do and not do, which content/vi writes 3141 times, so the five Integration
// chapters were reset. Principal ideal domain is mien idean chinh, which is in
// the glossary and in the corpus 219 times, against the prime it came back with.
// Flat modules had picked up a domain that is not in the English. General
// topology is topo dai cuong, 131 against 30 for the alternative, which is the
// count titles.go already records for the volume title. Groupoid and
// sesquilinear the corpus leaves in the original, 278 and 61 times, so the
// chapter titles leave them there too rather than introducing a word the body
// does not use. Unitary is unita, 234 times in content/ts.
//
// The English is the sixteen French only volumes, read the same way. Two were
// changed to match the corpus rather than the answer: Algebraic Topology chapter
// IV is unravelable spaces, which is what content/en-mt calls it in its own
// section titles, and Spectral Theories chapter III is linear mappings and not
// linear operators, because mapping is the word content/en uses everywhere else,
// including the title of chapter III of Topological Vector Spaces.
//
// The key is the Book and the chapter numeral, which is what loadChapter has in
// hand. A language missing from a row falls through to the corpus, so adding the
// Chinese and the Japanese later is a line each and nothing else moves.
var chapterTitles = map[string]map[string]string{
	"ac/I":     {"vi": "MÔĐUN PHẲNG"},
	"ac/II":    {"vi": "ĐỊA PHƯƠNG HÓA"},
	"ac/III":   {"vi": "PHÂN BẬC. LỌC VÀ TÔPÔ"},
	"ac/IV":    {"vi": "CÁC IĐÊAN NGUYÊN TỐ LIÊN KẾT VÀ PHÂN TÍCH NGUYÊN SƠ"},
	"ac/V":     {"vi": "CÁC SỐ NGUYÊN"},
	"ac/VI":    {"vi": "ĐỊNH GIÁ"},
	"ac/VII":   {"vi": "ƯỚC"},
	"ac/VIII":  {"vi": "CHIỀU"},
	"ac/IX":    {"en": "COMPLETE NOETHERIAN LOCAL RINGS", "vi": "VÀNH ĐỊA PHƯƠNG NOETHER HOÀN CHỈNH"},
	"ac/X":     {"en": "Depth, regularity, duality", "vi": "Độ sâu, chính quy, đối ngẫu"},
	"alg/I":    {"vi": "CẤU TRÚC ĐẠI SỐ"},
	"alg/II":   {"vi": "ĐẠI SỐ TUYẾN TÍNH"},
	"alg/III":  {"vi": "ĐẠI SỐ TENXƠ, ĐẠI SỐ NGOÀI, ĐẠI SỐ ĐỐI XỨNG"},
	"alg/IV":   {"vi": "ĐA THỨC VÀ PHÂN THỨC HỮU TỈ"},
	"alg/V":    {"vi": "TRƯỜNG GIAO HOÁN"},
	"alg/VI":   {"vi": "NHÓM CÓ THỨ TỰ VÀ TRƯỜNG CÓ THỨ TỰ"},
	"alg/VII":  {"vi": "MÔĐUN TRÊN MIỀN IĐÊAN CHÍNH"},
	"alg/VIII": {"vi": "Các môđun và vành nửa đơn"},
	"alg/IX":   {"en": "Sesquilinear forms and quadratic forms", "vi": "Dạng sesquilinear và dạng toàn phương"},
	"alg/X":    {"en": "HOMOLOGICAL ALGEBRA", "vi": "ĐẠI SỐ ĐỒNG ĐIỀU"},
	"ens/I":    {"vi": "MÔ TẢ TOÁN HỌC HÌNH THỨC"},
	"ens/II":   {"vi": "LÝ THUYẾT TẬP HỢP"},
	"ens/III":  {"vi": "TẬP HỢP CÓ THỨ TỰ, LỰC LƯỢNG, SỐ NGUYÊN"},
	"ens/IV":   {"vi": "CẤU TRÚC"},
	"evt/I":    {"vi": "KHÔNG GIAN VECTƠ TÔPÔ TRÊN MỘT VÀNH CHIA CÓ GIÁ TRỊ"},
	"evt/II":   {"vi": "TẬP HỢP LỒI VÀ KHÔNG GIAN LỒI ĐỊA PHƯƠNG"},
	"evt/III":  {"vi": "KHÔNG GIAN CÁC ÁNH XẠ TUYẾN TÍNH LIÊN TỤC"},
	"evt/IV":   {"vi": "ĐỐI NGẪU TRONG KHÔNG GIAN VECTƠ TÔPÔ"},
	"evt/V":    {"vi": "KHÔNG GIAN HILBERT (LÝ THUYẾT SƠ CẤP)"},
	"fvr/I":    {"vi": "ĐẠO HÀM"},
	"fvr/II":   {"vi": "NGUYÊN HÀM VÀ TÍCH PHÂN"},
	"fvr/III":  {"vi": "HÀM SƠ CẤP"},
	"fvr/IV":   {"vi": "PHƯƠNG TRÌNH VI PHÂN"},
	"fvr/V":    {"vi": "NGHIÊN CỨU ĐỊA PHƯƠNG CỦA HÀM"},
	"fvr/VI":   {"vi": "KHAI TRIỂN TAYLOR TỔNG QUÁT HÓA CÔNG THỨC TỔNG EULER-MACLAURIN"},
	"fvr/VII":  {"vi": "HÀM GAMMA"},
	"hist/1":   {"vi": "CÁC YẾU TỐ CỦA LỊCH SỬ TOÁN HỌC"},
	"int/I":    {"vi": "BẤT ĐẲNG THỨC CỦA TÍNH LỒI"},
	"int/II":   {"vi": "KHÔNG GIAN RIESZ"},
	"int/III":  {"vi": "CÁC ĐỘ ĐO TRÊN KHÔNG GIAN ĐỊA PHƯƠNG COMPACT"},
	"int/IV":   {"vi": "MỞ RỘNG CỦA MỘT ĐỘ ĐO. KHÔNG GIAN LP"},
	"int/V":    {"vi": "TÍCH PHÂN CỦA CÁC ĐỘ ĐO"},
	"int/VI":   {"vi": "TÍCH PHÂN VECTƠ"},
	"int/VII":  {"vi": "ĐỘ ĐO HAAR"},
	"int/VIII": {"vi": "TÍCH CHẬP VÀ BIỂU DIỄN"},
	"int/IX":   {"vi": "CÁC ĐỘ ĐO TRÊN KHÔNG GIAN TÔPÔ HAUSDORFF"},
	"lie/I":    {"vi": "Đại số Lie"},
	"lie/II":   {"vi": "ĐẠI SỐ LIE TỰ DO"},
	"lie/III":  {"en": "LIE GROUPS", "vi": "NHÓM LIE"},
	"lie/IV":   {"vi": "NHÓM COXETER VÀ HỆ TITS"},
	"lie/V":    {"vi": "NHÓM SINH BỞI CÁC PHÉP PHẢN XẠ"},
	"lie/VI":   {"vi": "HỆ NGHIỆM"},
	"lie/VII":  {"vi": "ĐẠI SỐ CON CARTAN VÀ CÁC PHẦN TỬ CHÍNH QUY"},
	"lie/VIII": {"vi": "ĐẠI SỐ LIE NỬA ĐƠN TÁCH"},
	"lie/IX":   {"vi": "NHÓM LIE THỰC COMPACT"},
	"ta/I":     {"en": "COVERING SPACES", "vi": "PHỦ"},
	"ta/II":    {"en": "GROUPOIDS", "vi": "GROUPOID"},
	"ta/III":   {"en": "HOMOTOPY AND THE POINCARÉ GROUPOID", "vi": "ĐỒNG LUÂN VÀ GROUPOID POINCARÉ"},
	"ta/IV":    {"en": "UNRAVELABLE SPACES", "vi": "KHÔNG GIAN THÁO GỠ ĐƯỢC"},
	"top/I":    {"vi": "Cấu trúc Tôpô"},
	"top/II":   {"vi": "Cấu trúc đều"},
	"top/III":  {"vi": "Nhóm Tôpô"},
	"top/IV":   {"vi": "Số thực"},
	"top/V":    {"vi": "Nhóm một tham số"},
	"top/VI":   {"vi": "Không gian số thực và không gian xạ ảnh"},
	"top/VII":  {"vi": "Các nhóm cộng tính $\\mathbf{R}^n$"},
	"top/VIII": {"vi": "Số phức"},
	"top/IX":   {"en": "USE OF REAL NUMBERS IN GENERAL TOPOLOGY", "vi": "Sử dụng số thực trong tôpô đại cương"},
	"top/X":    {"vi": "Không gian hàm"},
	"ts/I":     {"en": "NORMED ALGEBRAS", "vi": "ĐẠI SỐ CHUẨN HÓA"},
	"ts/II":    {"en": "LOCALLY COMPACT COMMUTATIVE GROUPS", "vi": "CÁC NHÓM GIAO HOÁN COMPACT ĐỊA PHƯƠNG"},
	"ts/III":   {"en": "COMPACT LINEAR MAPPINGS AND PERTURBATIONS", "vi": "CÁC ÁNH XẠ TUYẾN TÍNH COMPACT VÀ NHIỄU LOẠN"},
	"ts/IV":    {"en": "HILBERTIAN SPECTRAL THEORY", "vi": "LÝ THUYẾT PHỔ HILBERT"},
	"ts/V":     {"en": "UNITARY REPRESENTATIONS", "vi": "BIỂU DIỄN UNITA"},
	"var/1":    {"en": "DIFFERENTIABLE AND ANALYTIC MANIFOLDS, FASCICLE OF RESULTS", "vi": "ĐA TẠP VI PHÂN VÀ GIẢI TÍCH, TẬP KẾT QUẢ"},
}

// chapterTitle is the title of one chapter in one language. It prefers the
// table, and falls back to whatever the corpus put on the section files, which
// is right for content/en and content/fr, where the chapter_title field is
// already in the language being built.
func chapterTitle(book, numeral, lang, corpusTitle string) string {
	if byLang, ok := chapterTitles[book+"/"+numeral]; ok {
		if t := byLang[lang]; t != "" {
			return t
		}
	}
	return corpusTitle
}
