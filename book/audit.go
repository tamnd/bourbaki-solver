package book

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The audit is the point of the whole milestone and the build is only how it
// gets its evidence.
//
// Every gate this repository has so far reads one file at a time. The corpus
// audit reads a body, publish -check reads a math span, the anchor census reads
// an identifier, and all three can pass on a corpus that is not a book: a
// chapter missing from the middle, a § that lost half its subsections in the
// assembly, a volume that is a fifth of the length it was taken from. None of
// those is visible from inside one file, and every one of them is obvious the
// moment somebody tries to make a book out of it.
//
// So the checks below are all of the kind that need the whole volume at once,
// and every one of them either passes or names something a person can go and
// look at. What is deliberately not here is a quality score. A number that goes
// up is a number people learn to make go up; a list of named failures is a list
// somebody has to either fix or argue with.

// A Check is one thing the audit asked about a built volume.
type Check struct {
	Name string
	OK   bool
	// Detail is what was found, and it is written whether the check passed or
	// failed, because "3 of 3 chapters" is worth as much as "1 of 3 chapters"
	// to somebody reading the report six months later.
	Detail string
	// Notes are the individual things behind a failure, capped when reported.
	Notes []string
}

// An Audit is one volume, built and looked at.
type Audit struct {
	Volume string
	Lang   string
	Title  string

	Doc   *Document
	Build *Build
	EPUB  *EPUB

	// Have and Want are the sections of the printing this language holds, off
	// the sections manifest.
	Have, Want int
	Absent     []string

	Checks []Check
}

// ok records a check that passed or failed on a condition.
func (a *Audit) ok(name string, pass bool, detail string, notes ...string) {
	a.Checks = append(a.Checks, Check{Name: name, OK: pass, Detail: detail, Notes: notes})
}

// Failed is the number of checks that did not pass, which is what the command
// exits on.
func (a *Audit) Failed() int {
	n := 0
	for _, c := range a.Checks {
		if !c.OK {
			n++
		}
	}
	return n
}

// AuditOptions says how strict to be about the two numbers that are judgements
// rather than facts.
type AuditOptions struct {
	// Overfull is the most lines that may run past the measure. A few dozen in a
	// volume is ordinary typesetting for a book this dense. A few thousand is a
	// class that does not fit its content.
	Overfull int
	// Stray and Wide are ceilings on the two checks that are counting damage the
	// text layer did rather than anything the build got wrong: TeX control
	// sequences left loose in the prose, and arrays whose preamble came out
	// narrower than their own rows.
	//
	// They are ceilings rather than plain checks for the reason publish gives
	// about its refused formulae. Both are real defects and both are in the
	// corpus today, so a check with no ceiling is a check that is red every
	// night, and a gate that is always red is a gate everybody learns to ignore.
	// A ceiling may be lowered as the corpus is repaired and may never be
	// raised, which turns each of them into a ratchet: the number cannot get
	// worse without somebody noticing.
	Stray, Wide int
	// Short is how far under the printing's own text the volume may sit before
	// the check fails, as a fraction. A tenth is room for the front matter and
	// the publisher's pages that no reading of the book puts into content/, and
	// not room for a chapter.
	Short float64
	// Cover asks pdftoppm to render the first page and looks at it, which is the
	// only way to know that the cover is the cover. It is a flag because
	// poppler is not on every machine and a missing tool should skip a check
	// rather than fail a build.
	Cover bool
}

// DefaultAuditOptions are what the command uses when nothing is said.
func DefaultAuditOptions() AuditOptions {
	return AuditOptions{Overfull: 200, Short: 0.10, Cover: true}
}

// Inspect runs every check over a volume that has been loaded, written, built
// and packed. Any of the three results may be nil, and the checks that need one
// are skipped rather than guessed at, so that an audit of a document that never
// reached the typesetter still says everything it can.
func Inspect(root string, v *Volume, d *Document, b *Build, e *EPUB, opt AuditOptions) *Audit {
	a := &Audit{Volume: v.Meta.ID, Lang: v.Lang, Title: v.Title, Doc: d, Build: b, EPUB: e}
	a.structure(v)
	a.coverage(root, v)
	a.length(root, v, opt)
	a.written(d, opt)
	a.typeset(v, b, opt)
	a.packed(e)
	a.cover(b, opt)
	return a
}

// structure is what can be asked of the volume before anything renders it: that
// the chapters are the ones the manifest names, that the §§ are numbered the
// way a book numbers them, and that what the front matter claims is on the page.
func (a *Audit) structure(v *Volume) {
	want := v.Meta.Chapters
	var absent []string
	for _, numeral := range want {
		if c, ok := v.Chapter(numeral); !ok || (c.Front == nil && len(c.Sections) == 0) {
			absent = append(absent, numeral)
		}
	}
	a.ok("the chapters the manifest names are all here", len(absent) == 0,
		fmt.Sprintf("%d of %d chapters", len(want)-len(absent), len(want)), absent...)

	// A § that is not there leaves a hole in the numbering, and a hole in the
	// numbering is the shape a lost file makes. It is checked per chapter over
	// the §§ alone, since an appendix is numbered in its own series.
	var holes []string
	for _, c := range v.Chapters {
		seen := map[int]bool{}
		high := 0
		for _, s := range c.Sections {
			if s.Kind == "" || s.Kind == "section" {
				seen[s.Number] = true
				if s.Number > high {
					high = s.Number
				}
			}
		}
		for n := 1; n <= high; n++ {
			if !seen[n] {
				holes = append(holes, fmt.Sprintf("chapter %s has no §%d and has a §%d", c.Numeral, n, high))
			}
		}
	}
	a.ok("the §§ of every chapter run without a gap", len(holes) == 0,
		fmt.Sprintf("%d gaps", len(holes)), holes...)

	// The subsections are the same question one level down, and the one that
	// catches an assembly that dropped a page: a § that runs 1, 2, 3, 7 lost
	// three subsections somewhere between the scan and the file.
	var gaps []string
	for _, s := range v.Pieces() {
		if !s.IsSection() {
			continue
		}
		nums := subsectionNumbers(s.Body)
		for i := 1; i < len(nums); i++ {
			if nums[i] != nums[i-1]+1 {
				gaps = append(gaps, fmt.Sprintf("%s: no. %d is followed by no. %d", s.Path, nums[i-1], nums[i]))
			}
		}
	}
	a.ok("the numbered subsections run without a gap", len(gaps) == 0,
		fmt.Sprintf("%d gaps", len(gaps)), gaps...)

	// The front matter of each file says how many statements it holds, written
	// by the assembly. A file whose body has fewer than it claims lost one.
	var short []string
	for _, s := range v.Pieces() {
		if s.Statements == 0 {
			continue
		}
		if got := statementCount(s.Body); got < s.Statements {
			short = append(short, fmt.Sprintf("%s: claims %d statements, sets %d", s.Path, s.Statements, got))
		}
	}
	a.ok("every statement the front matter claims is on the page", len(short) == 0,
		fmt.Sprintf("%d files short", len(short)), short...)
}

// subsectionRE is a numbered subsection heading in a body, "### 3. ASSOCIATIVE
// LAWS". It is the same shape latex.go sets from and is read again here rather
// than being counted during the render, because the audit has to be able to run
// over a volume the writer refused.
var subsectionRE = regexp.MustCompile(`(?m)^###\s+(\d+)\.\s`)

func subsectionNumbers(body string) []int {
	var out []int
	for _, m := range subsectionRE.FindAllStringSubmatch(body, -1) {
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		out = append(out, n)
	}
	return out
}

// statementRE here is the heading level the assembly writes a statement at,
// four hashes or more. The kind and the number are read elsewhere; all that is
// wanted here is how many there are.
var statementHeadRE = regexp.MustCompile(`(?m)^#{4,6}\s+\S`)

func statementCount(body string) int {
	return len(statementHeadRE.FindAllString(body, -1))
}

// coverage is how much of the printing this language holds.
func (a *Audit) coverage(root string, v *Volume) {
	have, want, absent, err := Coverage(root, v)
	if err != nil {
		a.ok("the sections manifest knows this volume", false, err.Error())
		return
	}
	a.Have, a.Want, a.Absent = have, want, absent
	// A language the volume was printed in should hold all of it. A language it
	// was translated into holds what has been translated, and the check is that
	// the number is written down and not that it is a hundred: a Vietnamese
	// Algebra at eighty per cent is a real thing that should build, and calling
	// it a failure every night would teach everyone to ignore the audit.
	pass := have == want || v.Lang != v.Meta.Lang
	detail := fmt.Sprintf("%d of %d sections, %.1f%%", have, want, 100*float64(have)/float64(max(want, 1)))
	notes := absent
	if len(notes) > 12 {
		notes = append(notes[:12:12], fmt.Sprintf("and %d more", len(absent)-12))
	}
	a.ok("the printing's sections are all in this language", pass, detail, notes...)
}

// length is whether the volume holds the text the printing has.
//
// This used to be a page count. The build set 326 pages, the printing has 443,
// anything past a fifth either way was a failure. It was the wrong measure, and
// it took three volumes to see why: General Topology I-IV and Algebre
// commutative V-VII both sat at 74 per cent of their printings and both hold
// every word of them. What differs is the type. Algebre commutative sets 41
// lines of 55 characters to the page where this class sets 44 of 62, so 346 of
// those pages is 257 of these, and Topological Vector Spaces, whose printing is
// nearly as dense as the class, comes out at 95 per cent of its pages from the
// same corpus and the same build. A check that turns on the publisher's leading
// is a check about Springer and not about the book.
//
// What it was reaching for is that nothing is missing, and the repository holds
// the printing's own text to ask that of. pages/<volume>/ is the reading of
// every page of the printing, page by page, and content/ is assembled out of
// it, so the two are the same book in the same words at two stages. Counting
// characters asks the question in the one unit that does not move when the type
// does. Measured that way the two volumes that were failing sit at 99 and 102
// per cent, and Theory of Sets, which passed the page check comfortably, sits
// at 84 because the Summary of Results, 67 pages of it, is not in content/ at
// all.
//
// The comparison is one-sided on purpose. content/ writes \varphi where the
// printing sets one letter and $\mathfrak{S}$ where it sets one more, so a
// faithful volume runs a few per cent over the printing and how far over is a
// fact about the markup rather than about the book. Short is the only direction
// that means something is gone.
func (a *Audit) length(root string, v *Volume, opt AuditOptions) {
	printed, pages, err := printingChars(root, v.Meta.ID)
	if err != nil || pages == 0 {
		a.ok("the volume holds the text the printing has", true,
			"the corpus has no reading of this printing to compare against")
		return
	}
	held := v.Chars()
	// A language the volume was never printed in is compared against the
	// printing scaled by how much of it exists, for the reason the coverage
	// check gives: half a translation should hold about half the text, and
	// calling that a failure would be calling the translation's progress a
	// defect.
	scale := 1.0
	if a.Want > 0 && a.Have < a.Want {
		scale = float64(a.Have) / float64(a.Want)
	}
	ratio := float64(held) / float64(printed) / scale
	a.ok("the volume holds the text the printing has", ratio >= 1-opt.Short,
		fmt.Sprintf("%d characters against the reading's %d over %d pages, %.0f%% of it",
			held, printed, pages, 100*ratio))
}

// printingChars counts the characters in the reading of the printing, which is
// pages/<volume>/ and nothing else. The front matter of each page file is a
// record about the page rather than text off it, so it is not counted, and
// whitespace is not counted either because a line break is a decision the
// reading made and not a character the printing set.
func printingChars(root, id string) (chars, pages int, err error) {
	dir := filepath.Join(root, "pages", id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return 0, 0, err
		}
		_, body, err := corpus.SplitFrontMatter(raw)
		if err != nil {
			body = raw
		}
		chars += countNonSpace(string(body))
		pages++
	}
	return chars, pages, nil
}

// countNonSpace counts the runes that are not whitespace.
func countNonSpace(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}

// Chars is the text of the volume, in the same unit printingChars uses, so that
// the two can be divided. It is every body the build would set: the Book's
// introduction, each chapter's opening page, its §§ and appendices with their
// exercises, and its historical note.
func (v *Volume) Chars() int {
	n := 0
	if v.Intro != nil {
		n += countNonSpace(v.Intro.Body)
	}
	for _, c := range v.Chapters {
		if c.Front != nil {
			n += countNonSpace(c.Front.Body)
		}
		for _, s := range c.Sections {
			n += countNonSpace(s.Body)
			for _, e := range s.Exercises {
				n += countNonSpace(e.Body)
			}
		}
		if c.Historical != nil {
			n += countNonSpace(c.Historical.Body)
		}
	}
	return n
}

// written is what the writer found on its way through.
func (a *Audit) written(d *Document, opt AuditOptions) {
	if d == nil {
		return
	}
	a.ok("every character has a glyph the build can set", len(d.Missing) == 0,
		fmt.Sprintf("%d characters over %d places", distinct(d.Missing), len(d.Missing)),
		top(d.Missing, 12)...)
	a.ok("no TeX control sequence is loose in the prose", len(d.Stray) <= opt.Stray,
		fmt.Sprintf("%d sequences over %d places, the ceiling is %d",
			distinct(d.Stray), len(d.Stray), opt.Stray),
		top(d.Stray, 12)...)
	a.ok("every cross reference has something to point at", len(d.Dangling) == 0,
		fmt.Sprintf("%d references over %d places", distinct(d.Dangling), len(d.Dangling)),
		top(d.Dangling, 12)...)
	a.ok("no array had to be widened to hold its own rows", len(d.Wide) <= opt.Wide,
		fmt.Sprintf("%d arrays over %d places, the ceiling is %d",
			distinct(d.Wide), len(d.Wide), opt.Wide),
		top(d.Wide, 12)...)
	// The aligned displays are not a failure and are not checked as one. They
	// are the places the build decided a layout the corpus did not spell out,
	// and the number is here so that somebody comparing pages against the
	// printing knows how many decisions to look at.
	a.ok("displays the build laid out itself", true,
		fmt.Sprintf("%d multi-line displays set as an alignment", len(d.Aligned)))
}

// typeset is what the typesetter found.
func (a *Audit) typeset(v *Volume, b *Build, opt AuditOptions) {
	if b == nil {
		return
	}
	a.ok("the typesetter defined every command the writer used", len(b.Undefined) == 0,
		fmt.Sprintf("%d undefined", len(b.Undefined)), b.Undefined...)
	a.ok("every character reached the page", len(b.MissingGlyphs) == 0,
		fmt.Sprintf("%d characters with no glyph in their font", len(b.MissingGlyphs)), b.MissingGlyphs...)
	a.ok("every reference in the document resolved", b.Unresolved == 0,
		fmt.Sprintf("%d passes ended with references unresolved", b.Unresolved))
	a.ok("the lines fit the measure", b.Overfull <= opt.Overfull,
		fmt.Sprintf("%d overfull boxes, %d underfull, the ceiling is %d",
			b.Overfull, b.Underfull, opt.Overfull))

	// How many pages it came to is worth reporting and is not a failure. The
	// question of whether anything is missing is asked in characters, against
	// the reading of the printing, in length above; what is left here is how
	// this class set that text, and the honest answer is that it sets it
	// tighter than most of the printings. Two volumes out of the same corpus
	// and the same build land at 95 and 74 per cent of their printings because
	// the printings are two different books to look at. That is a fact about
	// the class worth having in the report and it is not a defect in the
	// volume, so it goes in the way the aligned displays do, as a number and
	// not a gate.
	if v.Meta.Pages <= 0 {
		a.ok("how long the volume came out", true, fmt.Sprintf("%d pages, the manifest has no page count to compare against", b.Pages))
		return
	}
	a.ok("how long the volume came out", true,
		fmt.Sprintf("%d pages against the printing's %d, %.0f%% of it",
			b.Pages, v.Meta.Pages, 100*float64(b.Pages)/float64(v.Meta.Pages)))
}

// packed reads the EPUB back off the disk and checks it.
//
// Reading it back rather than checking the strings that went in is the whole
// value of the check. An EPUB is a zip with rules about what is in it and how
// the parts name each other, and the ways it goes wrong are a document the
// manifest does not list, a link to a file that is not there, and an XHTML file
// that is not well formed XML. None of those is visible from the writer's side.
func (a *Audit) packed(e *EPUB) {
	if e == nil {
		return
	}
	a.ok("KaTeX reads every formula in the book", len(e.Refused) == 0,
		fmt.Sprintf("%d spans refused of %d", len(e.Refused), e.Math+len(e.Refused)),
		top(e.Refused, 12)...)

	z, err := zip.OpenReader(e.Path)
	if err != nil {
		a.ok("the EPUB opens", false, err.Error())
		return
	}
	defer z.Close()

	if len(z.File) == 0 || z.File[0].Name != "mimetype" || z.File[0].Method != zip.Store {
		a.ok("the EPUB opens", false, "the mimetype is not the first entry, stored")
		return
	}
	a.ok("the EPUB opens", true, fmt.Sprintf("%d entries, %d bytes", len(z.File), e.Bytes))

	held := map[string]bool{}
	body := map[string][]byte{}
	for _, f := range z.File {
		held[f.Name] = true
		if !strings.HasSuffix(f.Name, ".xhtml") && !strings.HasSuffix(f.Name, ".opf") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err == nil {
			body[f.Name] = raw
		}
	}

	// Well formed XML, every file of it. A reading system is entitled to stop
	// dead on a stray ampersand, and several do.
	var bad []string
	ids := map[string]map[string]bool{}
	for name, raw := range body {
		if err := wellFormed(raw); err != nil {
			bad = append(bad, name+": "+err.Error())
			continue
		}
		if strings.HasSuffix(name, ".xhtml") {
			ids[name] = idsIn(raw)
		}
	}
	a.ok("every document in the EPUB is well formed XML", len(bad) == 0,
		fmt.Sprintf("%d documents read", len(body)), bad...)

	// Every internal link points at a document that is in the book and, where it
	// names one, at an id that document has.
	var broken []string
	for name, raw := range body {
		if !strings.HasSuffix(name, ".xhtml") {
			continue
		}
		for _, href := range hrefsIn(raw) {
			if strings.Contains(href, "://") || strings.HasPrefix(href, "#") {
				continue
			}
			file, frag, _ := strings.Cut(href, "#")
			target := path.Join(path.Dir(name), file)
			if !held[target] {
				broken = append(broken, name+" -> "+href+": no such document")
				continue
			}
			if frag != "" && ids[target] != nil && !ids[target][frag] {
				broken = append(broken, name+" -> "+href+": no such id")
			}
		}
	}
	a.ok("every link in the EPUB has a target", len(broken) == 0,
		fmt.Sprintf("%d broken", len(broken)), cap12(broken)...)

	// The manifest has to list everything and everything has to be listed. A
	// document the manifest forgot is a document a reading system will not open,
	// and a manifest entry with no file behind it is a book that fails to load
	// on the strict readers and shows a blank page on the rest.
	var missing, orphan []string
	listed := map[string]bool{}
	for _, href := range hrefsIn(body["EPUB/package.opf"]) {
		full := path.Join("EPUB", href)
		listed[full] = true
		if !held[full] {
			missing = append(missing, href)
		}
	}
	for name := range held {
		if name == "mimetype" || strings.HasPrefix(name, "META-INF/") || name == "EPUB/package.opf" {
			continue
		}
		if !listed[name] {
			orphan = append(orphan, name)
		}
	}
	// The three the manifest does not list are the mimetype, the container and
	// the manifest itself, which are named by the format rather than by the book.
	a.ok("the manifest and the container hold the same files", len(missing) == 0 && len(orphan) == 0,
		fmt.Sprintf("%d listed, %d in the container", len(listed), len(held)-3),
		cap12(append(missing, orphan...))...)
}

// wellFormed reads a whole XML document and reports the first thing that is not.
func wellFormed(raw []byte) error {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	// The XHTML named entities are not declared anywhere the decoder can see,
	// and a book that writes &sect; is not malformed, it is XHTML. The three the
	// XML specification defines are handled by the decoder itself.
	dec.Strict = true
	dec.Entity = xml.HTMLEntity
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

var (
	idRE   = regexp.MustCompile(`\bid="([^"]*)"`)
	hrefRE = regexp.MustCompile(`\b(?:href|src|xlink:href)="([^"]*)"`)
)

func idsIn(raw []byte) map[string]bool {
	out := map[string]bool{}
	for _, m := range idRE.FindAllSubmatch(raw, -1) {
		out[string(m[1])] = true
	}
	return out
}

func hrefsIn(raw []byte) []string {
	var out []string
	for _, m := range hrefRE.FindAllSubmatch(raw, -1) {
		out = append(out, string(m[1]))
	}
	return out
}

// cover renders the first page of the PDF and looks at it.
//
// This is the check the milestone asked for by name and it is worth saying why
// it is a rendering and not a reading of the .tex. The class draws the cover
// with a page colour and four lines of type positioned by fractions of the
// trim, and every one of those is a thing that can silently not happen: a
// package that stopped shipping AddToShipoutPictureBG, a geometry change that
// moved the origin, a colour that came out as a named colour nobody defined. In
// every one of those cases the .tex still says \bcover and the PDF still has a
// first page. The only way to know it is yellow is to look at it.
func (a *Audit) cover(b *Build, opt AuditOptions) {
	if b == nil || !opt.Cover {
		return
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		a.ok("the cover is the printing's yellow", true, "pdftoppm is not installed, so the cover was not looked at")
		return
	}
	prefix := path.Join(b.Dir, "cover-check")
	cmd := exec.Command("pdftoppm", "-r", "18", "-f", "1", "-l", "1", "-forcenum", b.PDF, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		a.ok("the cover is the printing's yellow", false, "pdftoppm: "+strings.TrimSpace(string(out)))
		return
	}
	// pdftoppm pads the page number in the file name to the width of the page
	// count, so page one of a 873 page volume is -001 and page one of a nine
	// page one is -1. Asking for the file by name means knowing how long the
	// book is, which is a thing this has no business knowing, so the directory
	// is asked instead.
	written, _ := filepath.Glob(prefix + "-*.ppm")
	if len(written) != 1 {
		a.ok("the cover is the printing's yellow", false,
			fmt.Sprintf("pdftoppm wrote %d files, not one", len(written)))
		return
	}
	defer os.Remove(written[0])
	raw, err := os.ReadFile(written[0])
	if err != nil {
		a.ok("the cover is the printing's yellow", false, err.Error())
		return
	}
	yellow, blue, total, err := coverColours(raw)
	if err != nil {
		a.ok("the cover is the printing's yellow", false, err.Error())
		return
	}
	// Most of the page is the ground and a little of it is the type. Two thirds
	// yellow is a wide margin around the truth, which is about ninety five per
	// cent, and it is wide on purpose: the check is that the cover is the cover
	// and not that the wordmark is a particular number of pixels.
	pass := total > 0 && float64(yellow)/float64(total) > 0.66 && blue > 0
	a.ok("the cover is the printing's yellow", pass,
		fmt.Sprintf("%.1f%% of the first page is #FEC746 and %d pixels are the title blue",
			100*float64(yellow)/float64(max(total, 1)), blue))
}

// coverColours counts the two cover colours in a binary PPM.
//
// The format is parsed here rather than decoded with an image package because a
// P6 PPM is a magic number, three integers and the bytes, and pulling in a
// decoder for that is more code to read than the fifteen lines it saves.
func coverColours(raw []byte) (yellow, blue, total int, err error) {
	fields, rest, err := ppmHeader(raw)
	if err != nil {
		return 0, 0, 0, err
	}
	w, h, maxval := fields[0], fields[1], fields[2]
	if maxval != 255 {
		return 0, 0, 0, fmt.Errorf("the page rendered with %d levels, not 255", maxval)
	}
	if len(rest) < w*h*3 {
		return 0, 0, 0, fmt.Errorf("the page is %d by %d and carries %d bytes", w, h, len(rest))
	}
	near := func(r, g, b, wr, wg, wb int) bool {
		d := abs(r-wr) + abs(g-wg) + abs(b-wb)
		return d < 90
	}
	for i := 0; i < w*h*3; i += 3 {
		r, g, b := int(rest[i]), int(rest[i+1]), int(rest[i+2])
		total++
		switch {
		case near(r, g, b, 0xFE, 0xC7, 0x46):
			yellow++
		case near(r, g, b, 0x00, 0x77, 0xB4):
			blue++
		}
	}
	return yellow, blue, total, nil
}

// ppmHeader reads P6 and the three numbers after it, skipping the comments the
// format allows between them.
func ppmHeader(raw []byte) ([3]int, []byte, error) {
	var out [3]int
	if len(raw) < 2 || raw[0] != 'P' || raw[1] != '6' {
		return out, nil, fmt.Errorf("the page did not render as a binary PPM")
	}
	i := 2
	for n := 0; n < 3; n++ {
		for i < len(raw) {
			if raw[i] == '#' {
				for i < len(raw) && raw[i] != '\n' {
					i++
				}
				continue
			}
			if raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\n' || raw[i] == '\r' {
				i++
				continue
			}
			break
		}
		v := 0
		start := i
		for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
			v = v*10 + int(raw[i]-'0')
			i++
		}
		if i == start {
			return out, nil, fmt.Errorf("the PPM header is not three numbers")
		}
		out[n] = v
	}
	return out, raw[i+1:], nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// distinct is how many different things a list of findings is about, which is
// nearly always the number that matters: 239 occurrences of four characters is
// four problems and one afternoon.
func distinct(f []Finding) int {
	seen := map[string]bool{}
	for _, x := range f {
		seen[x.What] = true
	}
	return len(seen)
}

// top is the commonest findings, each with a place to go and look.
func top(f []Finding, n int) []string {
	by := map[string]int{}
	at := map[string]string{}
	for _, x := range f {
		by[x.What] += x.Count
		if at[x.What] == "" {
			at[x.What] = x.Where
		}
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if by[keys[i]] != by[keys[j]] {
			return by[keys[i]] > by[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var out []string
	for i, k := range keys {
		if i >= n {
			out = append(out, fmt.Sprintf("and %d more", len(keys)-n))
			break
		}
		out = append(out, fmt.Sprintf("%d  %s  %s", by[k], quoteWhat(k), at[k]))
	}
	return out
}

func cap12(s []string) []string {
	if len(s) <= 12 {
		return s
	}
	return append(s[:12:12], fmt.Sprintf("and %d more", len(s)-12))
}

// Report is the audit written out, one line per check with the failures opened
// up under them. It is the same text on the terminal and in the file the build
// leaves behind, so that a person watching a build and a person reading a report
// are arguing about one set of numbers.
func (a *Audit) Report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s-%s  %s\n", a.Volume, a.Lang, a.Title)
	if a.Doc != nil {
		fmt.Fprintf(&b, "  %d files, %d exercises, %d anchors\n", a.Doc.Files, a.Doc.Exercises, a.Doc.Anchors)
	}
	if a.Build != nil {
		fmt.Fprintf(&b, "  %d pages, %s\n", a.Build.Pages, a.Build.PDF)
	}
	if a.EPUB != nil {
		fmt.Fprintf(&b, "  %d documents, %d formulae, %s\n", a.EPUB.Documents, a.EPUB.Math, a.EPUB.Path)
	}
	b.WriteString("\n")
	for _, c := range a.Checks {
		mark := "ok  "
		if !c.OK {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "  %s  %-52s %s\n", mark, c.Name, c.Detail)
		if c.OK {
			continue
		}
		for _, n := range c.Notes {
			fmt.Fprintf(&b, "          %s\n", n)
		}
	}
	fmt.Fprintf(&b, "\n  %d of %d checks passed\n", len(a.Checks)-a.Failed(), len(a.Checks))
	return b.String()
}

// Markdown is the audit as a report to keep, in the corpus's own house style:
// one long line to a paragraph, a table of the checks, and the failures listed
// under it. It goes under work/, which the corpus does not commit, because it is
// a statement about a build and not about the corpus.
func (a *Audit) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s, %s\n\n", a.Title, a.Lang)
	fmt.Fprintf(&b, "Volume %s of the manifest, built out of content/%s.\n\n", a.Volume, a.Lang)

	b.WriteString("## What was built\n\n")
	b.WriteString("| | |\n| --- | --- |\n")
	if a.Doc != nil {
		fmt.Fprintf(&b, "| files | %d |\n| exercises | %d |\n| anchors | %d |\n",
			a.Doc.Files, a.Doc.Exercises, a.Doc.Anchors)
	}
	if a.Want > 0 {
		fmt.Fprintf(&b, "| sections of the printing | %d of %d |\n", a.Have, a.Want)
	}
	if a.Build != nil {
		fmt.Fprintf(&b, "| pages | %d |\n| overfull boxes | %d |\n| underfull boxes | %d |\n",
			a.Build.Pages, a.Build.Overfull, a.Build.Underfull)
	}
	if a.EPUB != nil {
		fmt.Fprintf(&b, "| EPUB documents | %d |\n| formulae as MathML | %d |\n| EPUB bytes | %d |\n",
			a.EPUB.Documents, a.EPUB.Math, a.EPUB.Bytes)
	}

	b.WriteString("\n## The checks\n\n| check | result | what was found |\n| --- | --- | --- |\n")
	for _, c := range a.Checks {
		mark := "ok"
		if !c.OK {
			mark = "**fail**"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", c.Name, mark, c.Detail)
	}

	failed := false
	for _, c := range a.Checks {
		if c.OK || len(c.Notes) == 0 {
			continue
		}
		if !failed {
			b.WriteString("\n## What failed\n")
			failed = true
		}
		fmt.Fprintf(&b, "\n### %s\n\n", c.Name)
		for _, n := range c.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	fmt.Fprintf(&b, "\n%d of %d checks passed.\n", len(a.Checks)-a.Failed(), len(a.Checks))
	return b.String()
}
