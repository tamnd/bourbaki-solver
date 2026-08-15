package glossary

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Mining the vocabulary out of the English corpus.
//
// Spec 06 §3 says to mine "every phrase that is bolded or italicised at first
// use", which is Bourbaki's own convention for introducing a term, and on the
// printed page it is exactly right. It does not survive the volume's text
// layer. The whole English corpus of chapter VIII, 344 files, carries five runs
// of star emphasis and two of underscore emphasis, and none of the seven is a
// defined term. Whatever the typesetter knew about which words were italic did
// not reach the file, so the strongest signal the spec names is not there to
// use and this mines the four that are.
//
//   - title. Every section and subsection title, which is Bourbaki's own
//     terminology chosen by Bourbaki: 146 of them in chapter VIII, and they are
//     the highest quality source here by a distance.
//   - defined. The phrase after "is called", "is said to be" or "we call". This
//     is thin on this volume, 30 or so hits, because most of chapter VIII's
//     definitions are stated in a Definition block rather than in a sentence,
//     but what it finds is always a term.
//   - prose. The recurring noun phrases of the running text, which is where
//     most of the working vocabulary is and where the noise is too.
//   - kind. The words for the statement kinds, which come off the labels rather
//     than out of the text. These are not mined so much as required: every
//     translation has to render Proposition and Theorem the same way every
//     time, and spec 06 §3 already fixes them per language.
//
// Nothing here decides anything. It produces a list for a person to look at,
// which is what manifests/glossary-candidates.yaml is, and the glossary proper
// holds what was decided.

// A Doc is one English content file, as the miner reads it.
type Doc struct {
	Path   string // relative to the corpus root
	Body   string
	Titles []string // the section title and the subsection titles
}

// A Candidate is one term the miner noticed.
type Candidate struct {
	EN    string `yaml:"en"`
	Count int    `yaml:"count"`
	// Sources are which of the four found it, in the order above.
	Sources []string `yaml:"sources"`
	// Where is the first place it occurs, as path:line, so that a curator can
	// read the sentence rather than guess at the phrase.
	Where string `yaml:"where,omitempty"`
}

// Candidates is manifests/glossary-candidates.yaml.
type Candidates struct {
	Version int         `yaml:"version"`
	From    string      `yaml:"from"`
	Terms   []Candidate `yaml:"terms"`
}

// Options tune the mining. The defaults are the ones measured against chapter
// VIII; ExtractCommand says what each of them was measured to do.
type Options struct {
	// MinCount is how often a phrase has to occur in the prose to be offered.
	MinCount int
	// MaxWords is the longest phrase mined. Four covers "central simple algebra
	// over a field" and above that the phrases are sentences.
	MaxWords int
	// Limit is how many candidates to write, longest first by count. Zero is
	// all of them.
	Limit int
}

// DefaultOptions are what the command uses when it is given no flags.
func DefaultOptions() Options { return Options{MinCount: 5, MaxWords: 4} }

// Extract mines the candidates from the English corpus.
func Extract(docs []Doc, opt Options) []Candidate {
	if opt.MinCount <= 0 {
		opt.MinCount = 1
	}
	if opt.MaxWords <= 0 {
		opt.MaxWords = 4
	}
	m := newMiner()
	for _, d := range docs {
		m.mineTitles(d)
		m.mineDefined(d)
		m.mineProse(d, opt.MaxWords)
	}
	m.addKinds()
	return m.candidates(opt)
}

// A miner accumulates counts and the first place each term was seen.
type miner struct {
	count   map[string]int
	sources map[string]map[string]bool
	where   map[string]string
	// display is the term as it was first written, so that "Artinian" keeps its
	// capital and "module" does not gain one.
	display map[string]string
	// whole is the phrases a title set off by itself: the title entire, and each
	// stretch of it that the stop words leave standing. "Segments of a
	// well-ordered set" puts in three, the title, "segments" and "well-ordered
	// set", and not "well-ordered" or "ordered set". See dropSubsumed.
	whole map[string]bool
}

func newMiner() *miner {
	return &miner{
		count:   map[string]int{},
		sources: map[string]map[string]bool{},
		where:   map[string]string{},
		display: map[string]string{},
		whole:   map[string]bool{},
	}
}

// The four sources, as they are written into the candidates file.
const (
	SourceTitle   = "title"
	SourceDefined = "defined"
	SourceProse   = "prose"
	SourceKind    = "kind"
)

func (m *miner) add(term, source, where string, n int) {
	key := Key(term)
	if key == "" {
		return
	}
	m.count[key] += n
	if m.sources[key] == nil {
		m.sources[key] = map[string]bool{}
	}
	m.sources[key][source] = true
	if _, ok := m.where[key]; !ok && where != "" {
		m.where[key] = where
	}
	// The first spelling wins, except that a lower case one beats a capitalised
	// one: a term met at the head of a sentence is capitalised for that reason
	// and not because it is a proper name, and Artin and Wedderburn are only
	// ever seen capitalised so they keep it either way.
	old, ok := m.display[key]
	if !ok || (startsUpper(old) && !startsUpper(term)) {
		m.display[key] = term
	}
}

func startsUpper(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

// mineTitles takes the titles whole and also the phrases inside them.
//
// Whole, because "Morita Equivalence of Modules and Algebras" is a thing to
// translate once and consistently, and in pieces because "Morita equivalence"
// is the term that will turn up in the running text.
func (m *miner) mineTitles(d Doc) {
	for _, t := range d.Titles {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		m.add(t, SourceTitle, d.Path, 1)
		m.whole[Key(t)] = true
		for _, run := range runs(strip(t)) {
			m.whole[Key(strings.Join(run, " "))] = true
			for _, ng := range ngrams(run, 4) {
				m.add(ng, SourceTitle, d.Path, 0)
			}
		}
	}
}

// definedRE is Bourbaki introducing a term in a sentence rather than in a
// Definition block. The trailing group stops at the punctuation or at the word
// that ends the noun phrase, and "if" is in there because "is called simple if
// it is nonzero" is the commonest shape of all.
var definedRE = regexp.MustCompile(`(?:is called|are called|is said to be|are said to be|we call|is termed) ((?:[a-z][a-z-]*)(?: [a-z][a-z-]*){0,3})`)

func (m *miner) mineDefined(d Doc) {
	for i, line := range strings.Split(d.Body, "\n") {
		for _, hit := range definedRE.FindAllStringSubmatch(strip(line), -1) {
			phrase := trimStops(strings.Fields(hit[1]))
			if len(phrase) == 0 {
				continue
			}
			m.add(strings.Join(phrase, " "), SourceDefined, fmt.Sprintf("%s:%d", d.Path, i+1), 1)
		}
	}
}

func (m *miner) mineProse(d Doc, maxWords int) {
	for i, line := range strings.Split(d.Body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue // the headings are the titles, counted already
		}
		for _, run := range runs(strip(line)) {
			for _, ng := range ngrams(run, maxWords) {
				m.add(ng, SourceProse, fmt.Sprintf("%s:%d", d.Path, i+1), 1)
			}
		}
	}
}

// kinds are the words for the statement kinds. They are not mined: they are the
// scaffolding of every § and they have to be rendered the same way in every
// file, so they go in whether the miner would have found them or not.
var kinds = []string{
	"theorem", "proposition", "lemma", "corollary", "definition",
	"remark", "example", "scholium", "exercise", "appendix",
	"chapter", "section", "historical note", "proof",
}

func (m *miner) addKinds() {
	for _, k := range kinds {
		m.add(k, SourceKind, "", 0)
	}
}

// singular is the phrase with the plural taken off its last word, and false
// when there is no plural to take off.
//
// The three endings are the ones English uses and the ones this corpus has:
// ideal/ideals, class/classes, category/categories. Nothing is guessed. The
// caller only folds when the singular it returns is itself a mined term, which
// is what keeps "basis" from becoming "basi" and "Weierstrass" from losing its
// tail: neither of those has a singular in the corpus to fold onto.
func singular(phrase string) (string, bool) {
	fields := strings.Fields(phrase)
	if len(fields) == 0 {
		return "", false
	}
	last := fields[len(fields)-1]
	var stem string
	switch {
	case strings.HasSuffix(last, "ies") && len(last) > 4:
		stem = last[:len(last)-3] + "y"
	case strings.HasSuffix(last, "sses") || strings.HasSuffix(last, "xes") ||
		strings.HasSuffix(last, "zes") || strings.HasSuffix(last, "ches") ||
		strings.HasSuffix(last, "shes"):
		stem = last[:len(last)-2]
	case strings.HasSuffix(last, "s") && !strings.HasSuffix(last, "ss") &&
		!strings.HasSuffix(last, "us") && !strings.HasSuffix(last, "is") && len(last) > 3:
		stem = last[:len(last)-1]
	default:
		return "", false
	}
	fields[len(fields)-1] = stem
	return strings.Join(fields, " "), true
}

// foldPlurals merges a plural into its singular when both were mined.
//
// "left ideal" and "left ideals" are one term and one glossary row, and a
// candidates file that offers both makes a curator decide the same thing twice.
// The count of the merged row is the two added, which is the number a curator
// wants: how often the corpus talks about left ideals at all.
func (m *miner) foldPlurals() {
	for key := range m.count {
		one, ok := singular(key)
		if !ok || one == key {
			continue
		}
		if _, ok := m.count[one]; !ok {
			continue
		}
		m.count[one] += m.count[key]
		for s := range m.sources[key] {
			m.sources[one][s] = true
		}
		if m.where[one] == "" {
			m.where[one] = m.where[key]
		}
		delete(m.count, key)
		delete(m.sources, key)
		delete(m.where, key)
		delete(m.display, key)
	}
}

// candidates is the counts turned into a list, filtered and sorted.
func (m *miner) candidates(opt Options) []Candidate {
	m.foldPlurals()
	var out []Candidate
	for key, n := range m.count {
		// A term seen only in a title, or only as a statement kind, has a prose
		// count of zero and is kept regardless: those two sources are curated
		// and the count is not what says they are worth translating.
		if n < opt.MinCount && !m.sources[key][SourceTitle] &&
			!m.sources[key][SourceKind] && !m.sources[key][SourceDefined] {
			continue
		}
		out = append(out, Candidate{
			EN:      m.display[key],
			Count:   n,
			Sources: sourceList(m.sources[key]),
			Where:   m.where[key],
		})
	}
	out = dropSubsumed(out, m.whole)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return Key(out[i].EN) < Key(out[j].EN)
	})
	if opt.Limit > 0 && len(out) > opt.Limit {
		out = out[:opt.Limit]
	}
	return out
}

func hasSource(c Candidate, want string) bool {
	return slices.Contains(c.Sources, want)
}

func sourceList(set map[string]bool) []string {
	var out []string
	for _, s := range []string{SourceTitle, SourceDefined, SourceProse, SourceKind} {
		if set[s] {
			out = append(out, s)
		}
	}
	return out
}

// dropSubsumed takes out a phrase that never occurs on its own.
//
// "artinian" and "artinian ring" both come out of the miner, and if every
// occurrence of the first is inside the second then the first is not a term the
// corpus uses, it is a fragment of one. The test is equal counts: a phrase that
// does occur alone somewhere has a count strictly greater than any phrase
// containing it.
//
// The test is a test on the prose, and a phrase the book itself set off is
// exempt from it. Three things set a phrase off: a title that is the phrase, a
// stretch of a title that the stop words leave standing on its own, and the
// definition and statement kind sources, both of which capture a whole noun
// phrase and never a fragment. Everything else is judged on the count.
//
// The counts of the curated sources cannot decide this on their own, which is
// why the exemption is needed. A title is counted once for the file it heads
// and the phrases inside it are counted zero, so "axiom" and "The axiom of
// extent" come out equal and the word loses to the sentence it sits in. Theory
// of Sets is where that shows worst, because its terms are being read off its
// table of contents while its pages are still being scanned, and axiom, empty
// set, ordered pair, well-ordered set, inverse limit and quantified theories
// were all going out this way. It is not only the unread volumes: segment
// heads a no. of chapter III, the corpus says the word once in prose, and that
// one sentence was enough to drop it against the phrase it occurred in.
//
// The exemption is the whole run and not any phrase a title contains, which is
// the difference between keeping "Morita equivalence" and keeping "connected
// commutative real lie". The first is a run of its title entire and the second
// is four words cut out of the middle of a longer one.
func dropSubsumed(in []Candidate, whole map[string]bool) []Candidate {
	count := map[string]int{}
	for _, c := range in {
		count[Key(c.EN)] = c.Count
	}
	var out []Candidate
	for _, c := range in {
		if whole[Key(c.EN)] || hasSource(c, SourceDefined) || hasSource(c, SourceKind) {
			out = append(out, c)
			continue
		}
		key := Key(c.EN)
		subsumed := false
		for longer, n := range count {
			if len(longer) <= len(key) || n != c.Count {
				continue
			}
			if containsPhrase(longer, key) {
				subsumed = true
				break
			}
		}
		if !subsumed {
			out = append(out, c)
		}
	}
	return out
}

// containsPhrase asks whether the short phrase occurs in the long one on word
// boundaries, so that "ring" is inside "artinian ring" and not inside "bringing".
func containsPhrase(long, short string) bool {
	l, s := strings.Fields(long), strings.Fields(short)
	for i := 0; i+len(s) <= len(l); i++ {
		if strings.Join(l[i:i+len(s)], " ") == short {
			return true
		}
	}
	return false
}

// strip takes out everything that is not prose: the mathematics, the heading
// attributes and the markup.
//
// The mathematics goes because it is not translated and its letters would join
// the words around it into phrases that were never written. It is replaced by a
// break rather than removed, so "the module $M$ is simple" does not mine "the
// module is simple" as one run.
func strip(s string) string {
	s = mathRE.ReplaceAllString(s, " | ")
	s = attrRE.ReplaceAllString(s, " | ")
	// What is left of a control sequence after the pairing above got it wrong.
	//
	// Pairing dollars is only right when a line has an even number of them, and
	// the volume has lines that do not: "the mapping cl(A$/\mathfrak{m})$ is a
	// bijection" opens a span in the middle of a token. Everything after the odd
	// dollar is then read as prose, and the prose it is read as is LaTeX.
	//
	// Measured by asking a model to translate the result. Of the 800 candidates
	// sent to it in the first Vietnamese run, it answered UNKNOWN to 26, and
	// eight of those were control sequences that got out this way: mathscr,
	// otimes, varepsilon, widetilde, sdet, and three more. It is a good detector
	// and a bad way to find out.
	s = commandRE.ReplaceAllString(s, " | ")
	s = markupRE.ReplaceAllString(s, " ")
	return s
}

var (
	mathRE    = regexp.MustCompile(`\$\$[^$]*\$\$|\$[^$]*\$`)
	attrRE    = regexp.MustCompile(`\{#[^}]*\}`)
	commandRE = regexp.MustCompile(`\\[a-zA-Z]+`)
	markupRE  = regexp.MustCompile(`[*_` + "`" + `]`)
)

// runs cuts a line into the stretches of ordinary words, breaking at anything
// that is not a word and at every stop word.
//
// Breaking at stop words is what keeps the phrases to noun phrases. "the length
// of the module" yields "length" and "module" and never "length of the", which
// is the fragment that dominates any frequency list built without it.
func runs(line string) [][]string {
	var out [][]string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur = nil
		}
	}
	for _, w := range words(line) {
		// A single letter breaks a phrase as surely as a stop word does. It is
		// never a term, and dropping it silently instead of breaking on it is
		// what made the first run of this mine "viii proposition" 149 times:
		// that is "VIII, p. 31, Proposition 4", with the p thrown away and the
		// two words either side of it left standing next to each other.
		if len([]rune(w)) < 2 || stop[w] || romans[w] || Operators[w] {
			flush()
			continue
		}
		cur = append(cur, w)
	}
	flush()
	return out
}

// words cuts a line into lower case word tokens. A hyphen and an apostrophe are
// part of a word, because "A-module" and "Schur's lemma" are terms; a digit is
// not, because a number in Bourbaki is a cross reference and never a term.
//
// An apostrophe is kept as the apostrophe the volume prints, U+2019, and not
// turned into a hyphen. It was a hyphen, and the row it made was "schur-s
// lemma", a string that occurs nowhere in the corpus. Nine rows of the glossary
// went in spelled that way, six of them real terms: schur-s lemma,
// burnside-s theorem, nakayama-s lemma, maschke-s theorem,
// hilbert-s nullstellensatz, zermelo-s theorem. A term is found in a section by
// literal search, so none of the six could ever be found, none could reach a
// translation prompt, and the adherence check could never hold a file to one.
func words(line string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			// Every token is emitted, including the one-letter ones. They are
			// dropped in runs, where dropping them breaks the phrase around
			// them rather than closing it up over the gap.
			if w := strings.Trim(b.String(), "-'’"); w != "" {
				out = append(out, w)
			}
			b.Reset()
		}
	}
	for _, r := range strings.ToLower(line) {
		switch {
		case unicode.IsLetter(r):
			b.WriteRune(r)
		case r == '-':
			if b.Len() > 0 {
				b.WriteRune('-')
			}
		case r == '\'' || r == '’':
			if b.Len() > 0 {
				b.WriteRune('’')
			}
		default:
			flush()
		}
	}
	flush()
	return out
}

// romans are the chapter numerals of a cross reference. They are words by every
// test above and terminology by none: "VIII" occurs a thousand times in chapter
// VIII and every one of them is a citation.
var romans = set("i ii iii iv v vi vii viii ix x xi xii")

// ngrams is every phrase of one to n words inside a run.
func ngrams(run []string, n int) []string {
	var out []string
	for size := 1; size <= n && size <= len(run); size++ {
		for i := 0; i+size <= len(run); i++ {
			out = append(out, strings.Join(run[i:i+size], " "))
		}
	}
	return out
}

func trimStops(fields []string) []string {
	for len(fields) > 0 && stop[strings.ToLower(fields[0])] {
		fields = fields[1:]
	}
	for len(fields) > 0 && stop[strings.ToLower(fields[len(fields)-1])] {
		fields = fields[:len(fields)-1]
	}
	return fields
}

// stop is the words a phrase does not cross.
//
// The first three groups are ordinary English function words. The fourth is the
// vocabulary of mathematical prose that is not terminology: "let", "hence",
// "such", "therefore". The fifth is the machinery of a Bourbaki cross
// reference, "p", "no", "cf", "loc", "cit", which occur thousands of times and
// would otherwise sit at the head of the frequency list.
//
// "set" was in the fourth group and came out of it. It is a verb in "we set x =
// 0" and that is what put it there, but the English corpus says "the set" 855
// times and "we set" 17, so the list was breaking phrases at the commonest noun
// in mathematics to catch one use in fifty. It cost ordered set, finite set,
// quotient set, empty set, inductive set, directed set and set theory, none of
// which could be mined while the word in the middle of them was a wall, and it
// cost the row for set itself, which Theory of Sets needs before anything of it
// can be translated.
var stop = set(`
a an the this that these those there here it its it's their his her our your my
and or but nor so yet if then else when while whenever whereas because since
of in on at by for with from to into onto over under between among through
across against about above below within without upon per via
is are was were be been being am do does did done have has had having
not no nor only also just even still already again ever never always
we us you they them he she who whom whose which what where why how
one two three four five six seven eight nine ten first second third
all any both each either every few many more most much neither none other same several some such
as than too very own less least more
let put take taken takes taking give given gives giving call called calls
say says said see seen show shown shows suppose supposed assume assumed
thus hence therefore moreover furthermore however conversely indeed namely
follows following follow followed denote denoted denotes write written writes
obtain obtained obtains consider considered consists consist
respectively particular general case cases sense way ways
p no cf loc cit ibid ff pp vol
`)

// notTerms are ordinary English words that are not vocabulary.
//
// Kept apart from stop, and used only when a row is judged and never when a
// phrase is cut. That is the whole point of it being a second list: "fixed
// point" and "character table" and "historical note" are terms, so a miner that
// broke a phrase at fixed, table or note would lose all three, while a glossary
// holding the bare words fix, table and note holds three rows that are not
// terminology and that tell a model how to render an ordinary word.
//
// Every word here was measured. It is what the second Vietnamese run put into
// manifests/glossary.yaml as a single-word row on 11 August 2026, read one by
// one, minus the ones that are terms. The renderings were mostly right, which
// is the point: "prove" came back "chứng minh" and that is correct Vietnamese
// for the word and no part of the terminology of algebra.
//
// It is not a rule that generalises and it does not pretend to. The real fix is
// upstream, where a frequency list of prose n-grams offers its tail as
// vocabulary, and until the miner is better than that this is the list.
//
// "respect" was added later and it is the one word here that was found rather
// than read. L06 reported it against the first translated section: the row
// rendered it bảo toàn, which is the verb, a map respecting a structure. The
// corpus uses the word 33 times and every one of the 33 is "with respect to",
// which is a preposition. So the row went out in the prompt for every chunk
// holding that phrase, inviting a mistake the translator did not make, and the
// rule that found it is the reason it is here.
var notTerms = set(`
abuse act adapt admit apply begin can come endow exist fact fix fixed hold
idea imply large last later lie make may mean must next now out prove respect
roles run send side study treat use used using well work
cor de resp th
`)

// Operators are Bourbaki's own abbreviations, the ones that are set upright
// inside a formula: Ker, Im, End, Aut, Hom, tr, det, and the rest.
//
// They are here because they came out of the second Vietnamese run as glossary
// rows, twenty-two of them, with renderings that are not wrong: end became "kết
// thúc", ker became "hạt nhân", inf became "cận dưới lớn nhất". Every one of
// those is the right Vietnamese for the notion and every one of them is a
// disaster in a translation prompt, because the prompt puts the glossary in
// front of a model and tells it to use the right hand column wherever the left
// hand one appears, and the left hand one appears inside $\operatorname{Ker} f$.
// A model that obeyed would break invariant 1 on every line it touched.
//
// They are in the corpus as bare words rather than as TeX because the printed
// page sets them upright and a transcription that read them off the page wrote
// them as words. So the miner sees them in prose and there is nothing about
// them, as words, that says they are notation. This list is what says it.
//
// A phrase breaks at one of these the same way it breaks at a stop word, which
// is right: "reduced trace trd" is not a term and neither is "trd nrd".
// pc, pcrd and coind were added after the list was written, off the rows they
// let through. The glossary was rendering pcrd as "ước chung phải lớn nhất",
// greatest right common divisor, where the book means the reduced
// characteristic polynomial and prints it upright in a formula, and the corpus
// says Pcrd 27 times, Pc 42 and Coind 30.
var Operators = set(`
ann aut card cl coind coker coim coker deg det diag dim end ex gal hom id im
ind inf int inv ker lcm gcd max min mod nrd ord pc pcrd pr proj rad rk sdet
sgn sup supp th tr trd
`)

func set(text string) map[string]bool {
	out := map[string]bool{}
	for w := range strings.FieldsSeq(text) {
		out[w] = true
	}
	return out
}

// WriteCandidates renders the mined list.
func WriteCandidates(path string, c Candidates) error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadCandidates reads the mined list back, for the curation pass.
func LoadCandidates(path string) (*Candidates, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Candidates
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}
