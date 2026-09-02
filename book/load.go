package book

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"gopkg.in/yaml.v3"
)

// Load reads one volume in one language out of a corpus checkout.
//
// The manifest entry says what the volume is: which Book, which chapters, what
// trim, how many printed pages. The content says what is in it. Neither alone
// is enough. content/en/alg holds ten chapters across four printings, so a walk
// of the directory would build one enormous Algebra that was never bound; the
// manifest holds no text at all.
//
// A language the corpus has nothing of is an error rather than an empty book,
// for the reason publish gives about a typed -lang vn: a build that quietly
// falls back to English looks like a build of Vietnamese that came out short.
func Load(root, id, lang string) (*Volume, error) {
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return nil, err
	}
	meta, ok := books.Get(id)
	if !ok {
		return nil, fmt.Errorf("no volume %s in manifests/books.yaml", id)
	}
	dir := filepath.Join(root, "content", lang)
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("no content in %s: %w", lang, err)
	}
	dirs := []string{dir}
	if lang == "en" {
		dirs = append(dirs, filepath.Join(root, "content", "en-mt"))
	}
	v := &Volume{Meta: *meta, Lang: lang, Title: title(meta.Book, lang, meta.Title)}

	for _, numeral := range meta.Chapters {
		c, err := loadChapter(root, pick(dirs, meta.Book, numeral), meta.Book, numeral, lang)
		if err != nil {
			return nil, err
		}
		v.Chapters = append(v.Chapters, c)
	}
	if n := meta.ReaderNote; n != nil {
		name := frontName(n.File, "00_to_the_reader.md")
		note, err := loadFront(root, pick(dirs, meta.Book, name), meta.Book, lang, name)
		if err != nil {
			return nil, err
		}
		v.Reader = note
	}
	if in := meta.Introduction; in != nil {
		name := frontName(in.File, "00_introduction.md")
		intro, err := loadFront(root, pick(dirs, meta.Book, name), meta.Book, lang, name)
		if err != nil {
			return nil, err
		}
		v.Intro = intro
	}
	if err := loadContentsTitles(root, v); err != nil {
		return nil, err
	}
	return v, nil
}

// loadContentsTitles fills in the sentence case titles the contents lists the
// numbered subsections under, off manifests/toc/<id>.yaml.
//
// Only for a build in the language the volume was printed in. The manifest is a
// reading of the volume's own contents pages, so alg-i-iii.yaml is English and
// says nothing about how a Vietnamese contents should read. A build in another
// language gets nothing here and falls back to the heading.
//
// A volume whose contents has not been read yet is not an error. Twenty eight of
// the forty three are in the manifest and the rest build with the heading, which
// is the same fallback the translations take.
func loadContentsTitles(root string, v *Volume) error {
	if v.Lang != v.Meta.Lang {
		return nil
	}
	raw, err := os.ReadFile(corpus.TOCPath(root, v.Meta.ID))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var bt corpus.BookTOC
	if err := yaml.Unmarshal(raw, &bt); err != nil {
		return fmt.Errorf("%s: %w", corpus.TOCPath(root, v.Meta.ID), err)
	}
	printed := map[string]map[int]string{}
	for _, c := range bt.Chapters {
		for _, s := range c.Sections {
			byNo := map[int]string{}
			for _, ss := range s.Subsections {
				if ss.Title != "" {
					byNo[ss.Number] = strings.TrimSpace(ss.Title)
				}
			}
			if len(byNo) > 0 {
				printed[fmt.Sprintf("%s/%d", c.Numeral, s.Number)] = byNo
			}
		}
	}
	for _, c := range v.Chapters {
		for _, s := range c.Sections {
			if s.Kind == corpus.KindAppendix {
				continue
			}
			s.Contents = printed[fmt.Sprintf("%s/%d", c.Numeral, s.Number)]
		}
	}
	return nil
}

// loadChapter reads content/<lang>/<book>/<numeral>/.
//
// A chapter the language has not reached is not an error here. It comes back
// empty and the coverage check is what refuses the build, so that the message a
// person gets names every § that is missing rather than the first directory
// that was not there.
// pick says which content tree a chapter or a front matter file comes out of.
//
// For every language but English there is one tree and this is the identity.
// English has two. content/en is what Springer translated, and Springer
// translated 15 of the 43 printings. content/en-mt is what this pipeline read
// out of the French for the other 28, and it is held apart from content/en on
// purpose, because one is a publisher's translation and the other is a model's
// and the audit has to be able to tell them apart.
//
// A build in English wants both. Algebre chapitre 9 has no English printing, so
// content/en/alg holds chapters I to VIII and nothing else, and building
// alg-ix-fr in English out of content/en alone gave a four page volume with a
// title and no text. It was going into the book repo that way until the sync
// noticed the section count was zero.
//
// The choice is per chapter and per file rather than per volume, because the
// split runs through a Book and not around it: content/en/alg is Springer for
// I to VIII and content/en-mt/alg picks up at IX.
func pick(dirs []string, parts ...string) string {
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(append([]string{d}, parts...)...)); err == nil {
			return d
		}
	}
	return dirs[0]
}

func loadChapter(root, langDir, book, numeral, lang string) (*Chapter, error) {
	c := &Chapter{Numeral: numeral}
	dir := filepath.Join(langDir, book, numeral)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		s, err := loadSection(root, filepath.Join(dir, e.Name()), lang)
		if err != nil {
			return nil, err
		}
		switch s.Kind {
		case corpus.KindFront:
			c.Front = s
		case corpus.KindHistorical:
			c.Historical = s
		default:
			c.Sections = append(c.Sections, s)
		}
		if c.Title == "" && s.ChapterTitle != "" {
			c.Title = s.ChapterTitle
		}
	}
	sortSections(c.Sections)
	// The corpus copies chapter_title across a translation unchanged, so what
	// the loop above read is the source printing's language and not the one
	// being built. chapterTitle puts the built language's title back where
	// there is one, and returns what the corpus said where there is not.
	c.Title = chapterTitle(book, numeral, lang, c.Title)
	if err := loadExercises(root, dir, c.Sections); err != nil {
		return nil, err
	}
	return c, nil
}

// sortSections puts the §§ in printed order: the numbered ones first, then the
// appendices, each run in number order.
//
// Bourbaki prints the appendices of a chapter after its last §, and the corpus
// numbers an appendix in the same field a § is numbered in, so Appendix 1 and
// § 1 are both section 1 and only the Appendix flag tells them apart. Sorting
// on the number alone puts the appendix of chapter II between § 1 and § 2,
// which is a book nobody printed.
func sortSections(secs []*Section) {
	sort.SliceStable(secs, func(i, j int) bool {
		a, b := secs[i], secs[j]
		if ap, bp := a.Kind == corpus.KindAppendix, b.Kind == corpus.KindAppendix; ap != bp {
			return bp
		}
		return a.Number < b.Number
	})
}

// loadFront reads one of the two files that sit beside the chapter directories
// rather than in one, the Book's own introduction and the publisher's note to
// the reader, because neither is in a chapter.
//
// The file is named rather than searched for. A Book printed as several volumes
// keeps them all in one directory, so a search for the first file of the right
// kind would give both volumes of Theories spectrales the introduction of
// chapters I and II. The name is the default for the kind unless books.yaml
// gives another, which is the same rule assembly wrote the file under.
//
// A volume whose manifest claims one and whose directory has none is not an
// error here. It is a volume translated as far as its chapters and no further,
// and the build says so through the coverage checks rather than by refusing.
func frontName(want, fallback string) string {
	if want != "" {
		return want
	}
	return fallback
}

func loadFront(root, langDir, book, lang, name string) (*Section, error) {
	path := filepath.Join(langDir, book, name)
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	return loadSection(root, path, lang)
}

func loadSection(root, path, lang string) (*Section, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := corpus.ParseFile[corpus.SectionFrontMatter](raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	kind := f.Meta.Kind
	if kind == "" && f.Meta.Appendix {
		kind = corpus.KindAppendix
	}
	if kind == "" {
		kind = corpus.KindSection
	}
	s := &Section{
		Kind: kind, Number: f.Meta.Section, Title: f.Meta.SectionTitle,
		Body: f.Body, Path: filepath.ToSlash(rel), Head: corpus.BodyStart(raw),
		Lang: lang, Subsecs: f.Meta.Subsections, Statements: f.Meta.Statements,
		BookTitle: f.Meta.BookTitle, ChapterTitle: f.Meta.ChapterTitle,
	}
	// A translation is titled off its own body, because its front matter is not
	// translated. The pipeline translates the body and copies the front matter
	// across, so content/vi/alg/I/05_s5_groups_operating_on_a_set.md carries
	// section_title: Groups operating on a set on a file whose page prints CÁC
	// NHÓM TÁC ĐỘNG TRÊN MỘT TẬP HỢP. 329 of the 331 Vietnamese section files are
	// in that state, and a Vietnamese volume built off the front matter comes out
	// with English running heads and an English table of contents over Vietnamese
	// pages. The words are already on the page. This reads them off it.
	if f.Meta.TranslatedFrom != "" {
		h1, h2 := bodyTitles(f.Body)
		if kind == corpus.KindFront && h1 != "" {
			s.ChapterTitle = h1
		}
		if t := sectionTitle(cmp.Or(h2, h1), f.Meta.Section); t != "" {
			s.Title = t
		}
	}
	if s.IsSection() {
		s.Label = corpus.Ref{Book: f.Meta.Book, Chapter: f.Meta.Chapter,
			Section: f.Meta.Section, Appendix: kind == corpus.KindAppendix}.SectionLabel()
	}
	return s, nil
}

// sectionNumberRE is the "§ 5." a body heading carries in front of the title.
// The document composes the number itself out of the front matter, where an
// appendix has not had its numeral eaten by an OCR, so the copy in the heading
// would print twice.
var sectionNumberRE = regexp.MustCompile(`^§+\s*\d+\s*\.?\s*`)

// bareSectionNumberRE is that number with no section sign in front of it, which
// is how most of the corpus writes the heading of a §: "## 1. OPEN SETS,
// NEIGHBOURHOODS, CLOSED SETS".
var bareSectionNumberRE = regexp.MustCompile(`^(\d+)\s*\.\s*`)

// sectionTitle is the title of a § read off its own body heading, with the
// number the document composes for itself taken off the front.
//
// The sign is optional and that is the whole of it. Stripping only "§ 5." left
// the number on every heading written "1. TITLE", and the contents of the eight
// Vietnamese volumes came out reading "§ 1. 1. CAC TAP MO, LAN CAN, CAC TAP
// DONG" on 175 lines, with the page heads saying the same thing. Only a
// translation reads its title off the body, which is why this had not turned up
// in English or French.
//
// The bare number has to match the number of the §, and a § numbered zero is
// not a § at all. Without that this would eat the front of any title that opens
// on a number, and the corpus has titles that do: II, § 5 of Theorie des
// ensembles is not one of them, but nothing says the next volume read will not
// be.
func sectionTitle(h string, n int) string {
	h = sectionNumberRE.ReplaceAllString(h, "")
	if n == 0 {
		return h
	}
	if m := bareSectionNumberRE.FindStringSubmatch(h); m != nil {
		if got, _ := strconv.Atoi(m[1]); got == n {
			return h[len(m[0]):]
		}
	}
	return h
}

// bodyTitles returns the level one and level two headings a body opens with.
//
// A chapter front page opens with two, the chapter number as a level two and the
// chapter title as a level one. Everything else opens with one level two heading
// which is its own title. It stops at the first block that is neither, so a
// level two heading further down, which four files in the corpus have, is not
// mistaken for a title.
func bodyTitles(body string) (h1, h2 string) {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := headingRE.FindStringSubmatch(line)
		if m == nil || len(m[1]) > 2 {
			return
		}
		switch {
		case len(m[1]) == 1 && h1 == "":
			h1 = strings.TrimSpace(m[2])
		case len(m[1]) == 2 && h2 == "":
			h2 = strings.TrimSpace(m[2])
		}
	}
	return
}

// loadExercises fills in the exercises of every § of one chapter.
//
// The corpus keeps them under exercises/s5/ and exercises/a1/, named for the §
// they belong to in the same slug the § itself is known by, which is what makes
// this a lookup rather than a search.
func loadExercises(root, chapterDir string, secs []*Section) error {
	dir := filepath.Join(chapterDir, "exercises")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	for _, s := range secs {
		sub := filepath.Join(dir, exerciseDir(s))
		entries, err := os.ReadDir(sub)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			ex, err := loadExercise(root, filepath.Join(sub, e.Name()))
			if err != nil {
				return err
			}
			s.Exercises = append(s.Exercises, ex)
		}
		sort.Slice(s.Exercises, func(i, j int) bool { return s.Exercises[i].Number < s.Exercises[j].Number })
	}
	return nil
}

// exerciseDir is the directory a §'s exercises are in, s5 or a1, which is the
// same slug publish gives the § on the site.
func exerciseDir(s *Section) string {
	if s.Kind == corpus.KindAppendix {
		return "a" + strconv.Itoa(s.Number)
	}
	return "s" + strconv.Itoa(s.Number)
}

func loadExercise(root, path string) (*Exercise, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := corpus.ParseFile[corpus.ExerciseFrontMatter](raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	return &Exercise{Number: f.Meta.Exercise, Label: f.Meta.Label, Body: f.Body,
		Path: filepath.ToSlash(rel), Head: corpus.BodyStart(raw),
		Starred: f.Meta.Starred, Hint: f.Meta.HasHint}, nil
}
