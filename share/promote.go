package share

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Promotion is the rule for moving an import into content/.
//
// The rule exists because imports/ and content/ are not two stages of one
// pipeline. An import is a transcription somebody made in a chat window, and a
// content file is a reading of the printed page with a page map behind it and
// 64 audit rules over it. Moving a file across that line is a claim about the
// text, so the rule is written down here, on its own, where it can be read and
// argued with, rather than living inside whatever function happens to copy the
// bytes.
//
// Four things have to hold, and the fourth is the one that surprised me.
//
// The file has to be a §. An introduction has no number, no place in the
// content layout, and nothing to hold it against, so it stays where it is.
//
// share audit has to pass: every no. the contents lists, every label the pages
// print, and every page of the § found somewhere in the text. That says the
// transcription is of the whole section rather than most of it, which is the
// one thing that cannot be seen by reading the file by itself.
//
// A person has to have read it against the printed volume and said so in
// manifests/imports.yaml. No machine check can stand in for this. share audit
// counts labels and looks for runs of words; it cannot tell a correct statement
// from a fluent wrong one, and a model transcribing mathematics produces
// fluent wrong ones. The review record is what the promotion rests on and the
// rest is a filter in front of the reader's time.
//
// And content/ must not already hold a reading of that § made from the pages.
// This is the case that turns out to be almost all of them: Theory of Sets was
// read by a model off the rendered pages long before the share links appeared,
// so content/en/ens/I already has all five §. A page-derived section carries
// pdf_pages, and every one of them can be turned back into the page files it
// came from and checked against the PDF. An import carries a URL to a
// conversation. Overwriting the first with the second would trade a reading
// with provenance for a reading without it, and it would do so silently,
// because both are Markdown and both look fine. So the rule is that an import
// is promoted into a gap and never over a page-derived file. Where the two
// overlap, the import's use is as a second opinion, which is a comparison and
// not a promotion.

// Refusal is why a section was not promoted, in the words that go in the
// report. Every import file gets one of these or gets promoted, and the pair
// of lists is the whole output: the exit criterion for M11 asks for every
// imported section to be either promoted or listed with the reason it was not,
// so a file that falls through both is a bug in this rule and not a quiet pass.
type Refusal string

const (
	// RefuseIntro is the book's introduction, which has no § of its own.
	RefuseIntro Refusal = "intro"
	// RefuseAudit is share audit finding something the printed book has and
	// the import does not.
	RefuseAudit Refusal = "audit"
	// RefuseUnreviewed is no person having recorded reading it against the
	// printed volume.
	RefuseUnreviewed Refusal = "unreviewed"
	// RefuseRejected is a person having read it and said it should not be
	// promoted.
	RefuseRejected Refusal = "rejected"
	// RefuseOccupied is content/ already holding a reading of that § made
	// from the pages.
	RefuseOccupied Refusal = "occupied"
	// RefuseStale is the file having changed since it was reviewed.
	RefuseStale Refusal = "stale"
)

// Decision is what the rule says about one import file.
type Decision struct {
	Target  Target
	Promote bool
	Refusal Refusal
	// Why is the sentence printed after the reason, with the particulars in
	// it: which no. was missing, who reviewed it, what occupies the path.
	Why string
}

// String is the line for the report.
func (d Decision) String() string {
	if d.Promote {
		return fmt.Sprintf("%s promoted: %s", d.Target, d.Why)
	}
	return fmt.Sprintf("%s not promoted, %s: %s", d.Target, d.Refusal, d.Why)
}

// Candidate is one import file with everything the rule needs to judge it,
// gathered by the caller so that the rule itself touches no disk and can be
// tested without a corpus.
type Candidate struct {
	Target Target
	// Audit is the result of share audit over this file, or nil if the file
	// is an introduction and was not audited.
	Audit *Result
	// ContentPath is where in content/ this § would go, relative to the
	// corpus root, and Occupant is what is there now.
	ContentPath string
	Occupant    *Occupant
	// SHA256 is the digest of the import's body as it is on disk now, which
	// is compared against what the reviewer signed off.
	SHA256 string
	Review *Review
}

// Occupant is what content/ already holds at the path an import would take.
//
// Extraction is the field that decides: a section assembled from page files
// says native or ocr, and a section that came from an earlier promotion says
// share. The first outranks any import. The second can be replaced, because
// replacing a promotion with a newer promotion loses nothing that was not
// already an import.
type Occupant struct {
	Extraction string
	PDFPages   string
}

// FromPages says whether the occupant was read off the printed page.
func (o *Occupant) FromPages() bool {
	if o == nil {
		return false
	}
	return o.Extraction != "" && o.Extraction != "share"
}

// Review is a person's record of reading an imported section against the
// printed volume, out of manifests/imports.yaml.
//
// Body is the digest of the import body the reader had in front of them. It is
// here so that editing an import after it was reviewed does not carry the
// review along with it: the review is of a text, not of a filename. A reviewer
// who cannot compute a digest can leave it empty and the promotion is allowed
// with a note, since a review with no digest is still a person's word and is
// worth more than no review, but it will not notice a later edit.
type Review struct {
	Import  string `yaml:"import"`
	Chapter int    `yaml:"chapter"`
	Section int    `yaml:"section"`
	By      string `yaml:"reviewed_by"`
	On      string `yaml:"reviewed_on"`
	Edition string `yaml:"printed_edition"`
	Body    string `yaml:"body_sha256,omitempty"`
	// Promote is the reader's verdict. It is a pointer so that a record with
	// no verdict in it is an error rather than a silent no: somebody who
	// writes a review and forgets the field means yes far more often than no,
	// and guessing either way would be wrong.
	Promote *bool `yaml:"promote"`
	// Findings is what the reader found wrong. It is required when they
	// promote as well as when they do not, because a reader who found nothing
	// should have to write "nothing", which is a different act from leaving a
	// field out.
	Findings string `yaml:"findings"`
}

// Reviews is manifests/imports.yaml.
type Reviews struct {
	Reviews []Review `yaml:"reviews"`
}

// ReviewsPath is where the review manifest lives in a checkout.
func ReviewsPath(root string) string {
	return filepath.Join(root, "manifests", "imports.yaml")
}

// LoadReviews reads the review manifest. A checkout with no manifest at all is
// not an error: it is a project where nobody has reviewed anything yet, and
// the answer to every promotion is then unreviewed, which is the right answer
// and not a crash.
func LoadReviews(root string) (*Reviews, error) {
	b, err := os.ReadFile(ReviewsPath(root))
	if os.IsNotExist(err) {
		return &Reviews{}, nil
	}
	if err != nil {
		return nil, err
	}
	var r Reviews
	if err := yaml.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", ReviewsPath(root), err)
	}
	for i, v := range r.Reviews {
		if v.Import == "" || v.Chapter == 0 || v.Section == 0 {
			return nil, fmt.Errorf("%s: review %d names no section, which needs import, chapter and section",
				ReviewsPath(root), i+1)
		}
		if v.By == "" {
			return nil, fmt.Errorf("%s: the review of %s %d.%d says nobody read it, and an unsigned review is not a review",
				ReviewsPath(root), v.Import, v.Chapter, v.Section)
		}
		if v.Promote == nil {
			return nil, fmt.Errorf("%s: the review of %s %d.%d has no promote field, and a verdict cannot be guessed from the rest",
				ReviewsPath(root), v.Import, v.Chapter, v.Section)
		}
		if strings.TrimSpace(v.Findings) == "" {
			return nil, fmt.Errorf("%s: the review of %s %d.%d records no findings, and a reader who found nothing has to write that down",
				ReviewsPath(root), v.Import, v.Chapter, v.Section)
		}
	}
	return &r, nil
}

// Find is the review of one section, or nil.
func (r *Reviews) Find(imp string, chapter, section int) *Review {
	if r == nil {
		return nil
	}
	for i := range r.Reviews {
		v := &r.Reviews[i]
		if v.Import == imp && v.Chapter == chapter && v.Section == section {
			return v
		}
	}
	return nil
}

// Decide applies the rule to one candidate.
func Decide(imp string, c Candidate) Decision {
	d := Decision{Target: c.Target}
	if c.Target.Intro {
		d.Refusal = RefuseIntro
		d.Why = "the introduction has no § of its own and no place in the content layout, so it stays an import"
		return d
	}
	if c.Audit == nil {
		d.Refusal = RefuseAudit
		d.Why = "it was never held against the printed volume, so there is nothing to promote on"
		return d
	}
	if !c.Audit.OK() {
		d.Refusal = RefuseAudit
		d.Why = fmt.Sprintf("share audit finds %d thing(s) the printed § has and this does not: %s",
			c.Audit.Hard(), firstHard(c.Audit))
		return d
	}
	if c.Occupant.FromPages() {
		d.Refusal = RefuseOccupied
		d.Why = fmt.Sprintf("%s is already a reading of this § from the pages (extraction: %s, pdf_pages: %s), which has provenance an import does not, so the import stays a second opinion",
			c.ContentPath, c.Occupant.Extraction, c.Occupant.PDFPages)
		return d
	}
	v := c.Review
	if v == nil {
		d.Refusal = RefuseUnreviewed
		d.Why = fmt.Sprintf("nobody has recorded reading it against the printed volume in %s, and no machine check stands in for that",
			filepath.Join("manifests", "imports.yaml"))
		return d
	}
	if v.Body != "" && c.SHA256 != "" && v.Body != c.SHA256 {
		d.Refusal = RefuseStale
		d.Why = fmt.Sprintf("%s read a body with digest %s and the file now has %s, so the review is of a text that is no longer there",
			v.By, short(v.Body), short(c.SHA256))
		return d
	}
	if v.Promote == nil || !*v.Promote {
		d.Refusal = RefuseRejected
		d.Why = fmt.Sprintf("%s read it against the printed volume and said not to promote it: %s", v.By, v.Findings)
		return d
	}
	d.Promote = true
	d.Why = fmt.Sprintf("share audit passes and %s read it against the %s printing on %s: %s",
		v.By, edition(v), v.On, v.Findings)
	if v.Body == "" {
		d.Why += " (the review records no body digest, so a later edit to the import would not be noticed)"
	}
	return d
}

func edition(v *Review) string {
	if v.Edition == "" {
		return "printed"
	}
	return v.Edition
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func firstHard(r *Result) string {
	for _, f := range r.Findings {
		if f.Hard {
			return f.Text
		}
	}
	return "something"
}

// Report is the summary of a whole promotion run.
type Report struct {
	Decisions []Decision
}

// Promoted is how many sections moved.
func (r *Report) Promoted() int {
	n := 0
	for _, d := range r.Decisions {
		if d.Promote {
			n++
		}
	}
	return n
}

// Reasons is how many were refused for each reason, in a stable order so the
// summary line does not shuffle between runs.
func (r *Report) Reasons() []string {
	count := map[Refusal]int{}
	for _, d := range r.Decisions {
		if !d.Promote {
			count[d.Refusal]++
		}
	}
	var out []string
	for k, v := range count {
		out = append(out, fmt.Sprintf("%d %s", v, k))
	}
	sort.Strings(out)
	return out
}
