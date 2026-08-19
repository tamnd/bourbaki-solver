package roundtrip

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The kinds of difference the judge may report.
//
// The list is closed and short on purpose. An open list of categories is a list
// the model writes rather than answers, and the point of naming them is to make
// the judge decide which of a small number of things went wrong instead of
// writing a paragraph about the passage. Anything it names that is not here
// becomes Other, which keeps the finding and loses only the label, because a
// real dropped hypothesis filed under a word this program has never heard of is
// still a real dropped hypothesis.
const (
	KindStatement  = "statement"  // the two say different things
	KindHypothesis = "hypothesis" // a condition on a result is gone or changed
	KindQuantifier = "quantifier" // every became some, or the order of two changed
	KindNumber     = "number"     // a number, an index or a dimension differs
	KindReference  = "reference"  // a cross reference points somewhere else
	KindOmission   = "omission"   // something the English says is not there
	KindAddition   = "addition"   // something is there that the English does not say
	KindOther      = "other"
)

var kinds = map[string]bool{
	KindStatement: true, KindHypothesis: true, KindQuantifier: true,
	KindNumber: true, KindReference: true, KindOmission: true,
	KindAddition: true, KindOther: true,
}

// A Difference is one place the judge says the two English texts do not say the
// same mathematics.
type Difference struct {
	Kind string `json:"kind"`
	// English is the passage as the corpus has it, quoted by the judge.
	English string `json:"english"`
	// Back is what came round the loop in its place.
	Back string `json:"back"`
	Why  string `json:"why"`
}

// A Verdict is one file's trip round the loop.
type Verdict struct {
	Path    string `json:"path"`
	English string `json:"english"`
	Lang    string `json:"lang"`
	// Digest is the translation body that went out, so that a verdict about a
	// file that has since been translated again can be seen for what it is.
	Digest     string `json:"digest"`
	BackModel  string `json:"back_model"`
	JudgeModel string `json:"judge_model"`
	On         string `json:"on"`
	// Same is the judge saying the two say the same mathematics, and it is
	// false whenever there is a difference under it whatever the judge said in
	// the summary field. See ParseJudgement.
	Same        bool         `json:"same"`
	Differences []Difference `json:"differences,omitempty"`
	// Back is the English that came back, kept so that a person reading a
	// reported difference can see the whole passage it came out of rather than
	// the judge's quotation of it.
	Back string `json:"back,omitempty"`
}

// Results is reports/roundtrip.json.
type Results struct {
	Rate     float64   `json:"rate"`
	Verdicts []Verdict `json:"verdicts"`
}

// ResultsPath is where a run's verdicts live in a checkout.
func ResultsPath(root string) string {
	return filepath.Join(root, "reports", "roundtrip.json")
}

// LoadResults reads the verdicts. A checkout with no file has run nothing yet,
// which is not an error and is answered by every file being stale.
func LoadResults(root string) (*Results, error) {
	b, err := os.ReadFile(ResultsPath(root))
	if os.IsNotExist(err) {
		return &Results{Rate: Rate}, nil
	}
	if err != nil {
		return nil, err
	}
	var r Results
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", ResultsPath(root), err)
	}
	return &r, nil
}

// Save writes the verdicts, sorted, so that two runs that judged the same files
// produce the same file and a diff is about what changed rather than about what
// order the hosts answered in.
func (r *Results) Save(root string) error {
	sort.Slice(r.Verdicts, func(i, j int) bool {
		if r.Verdicts[i].Lang != r.Verdicts[j].Lang {
			return r.Verdicts[i].Lang < r.Verdicts[j].Lang
		}
		return r.Verdicts[i].Path < r.Verdicts[j].Path
	})
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ResultsPath(root)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ResultsPath(root), append(b, '\n'), 0o644)
}

// Find is the verdict on one file, or nil.
func (r *Results) Find(lang, path string) *Verdict {
	if r == nil {
		return nil
	}
	for i := range r.Verdicts {
		if r.Verdicts[i].Lang == lang && r.Verdicts[i].Path == path {
			return &r.Verdicts[i]
		}
	}
	return nil
}

// Put files a verdict, replacing the one on the same file.
func (r *Results) Put(v Verdict) {
	if old := r.Find(v.Lang, v.Path); old != nil {
		*old = v
		return
	}
	r.Verdicts = append(r.Verdicts, v)
}

// Stale says whether an item has to go round the loop again: no verdict at all,
// or a verdict on a body that is no longer the body.
func (r *Results) Stale(it Item) bool {
	v := r.Find(it.Lang, it.Path)
	if v == nil {
		return true
	}
	return it.Digest != "" && v.Digest != "" && v.Digest != it.Digest
}

// A Count is one language's line in the report.
type Count struct {
	Lang        string
	Sampled     int
	Judged      int
	Stale       int
	Same        int
	Differing   int
	Differences int
}

// Tally is the sample held against the verdicts, one line a language.
//
// Sampled is what the draw picked, Judged is how many of those have a verdict
// on the body that is there now, and Stale is the rest. The three are reported
// separately rather than folded into a rate, because a run that judged two of
// forty and a run that judged forty of forty can print the same percentage and
// they are not the same claim.
func Tally(sample []Item, r *Results) []Count {
	at := map[string]int{}
	var out []Count
	for _, it := range sample {
		i, ok := at[it.Lang]
		if !ok {
			i = len(out)
			at[it.Lang] = i
			out = append(out, Count{Lang: it.Lang})
		}
		out[i].Sampled++
		if r.Stale(it) {
			out[i].Stale++
			continue
		}
		v := r.Find(it.Lang, it.Path)
		out[i].Judged++
		if v.Same {
			out[i].Same++
		} else {
			out[i].Differing++
			out[i].Differences += len(v.Differences)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lang < out[j].Lang })
	return out
}

// Line is the count as one line of the report.
func (c Count) Line() string {
	s := fmt.Sprintf("%s: %d sampled, %d judged", c.Lang, c.Sampled, c.Judged)
	if c.Stale > 0 {
		s += fmt.Sprintf(", %d waiting", c.Stale)
	}
	if c.Judged == 0 {
		return s + ", nothing measured yet"
	}
	s += fmt.Sprintf(", %d came back saying the same mathematics and %d did not", c.Same, c.Differing)
	if c.Differences > 0 {
		s += fmt.Sprintf(" over %d differences", c.Differences)
	}
	return s
}

// ParseJudgement reads the judge's answer.
//
// The answer is JSON because the alternative is reading prose for a verdict,
// and a judge that hedges in prose is a judge whose verdict is whatever the
// parser felt like that day. A fenced block is unwrapped, since a model asked
// for JSON hands back a fence about half the time and refusing that would throw
// away good answers over punctuation.
//
// A judge that lists a dropped hypothesis and then says the two are the same
// has answered the summary carelessly, and the list wins. Taking the summary
// would be discarding the evidence in favour of the sentence about it, which is
// the wrong way round: the differences are the finding and same is a
// convenience over them.
func ParseJudgement(answer string) (bool, []Difference, error) {
	text := unfence(answer)
	if strings.TrimSpace(text) == "" {
		return false, nil, fmt.Errorf("the judge said nothing")
	}
	var got struct {
		Same        *bool        `json:"same"`
		Differences []Difference `json:"differences"`
	}
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		return false, nil, fmt.Errorf("the judge did not answer in JSON: %w", err)
	}
	if got.Same == nil {
		return false, nil, fmt.Errorf("the judge's answer has no same field, so it holds no verdict")
	}
	var out []Difference
	for _, d := range got.Differences {
		d.Kind = strings.ToLower(strings.TrimSpace(d.Kind))
		if !kinds[d.Kind] {
			d.Kind = KindOther
		}
		if strings.TrimSpace(d.English) == "" && strings.TrimSpace(d.Back) == "" &&
			strings.TrimSpace(d.Why) == "" {
			// An entry with nothing in it is the model filling the shape of the
			// answer rather than reporting anything, and counting it would put
			// a file in the differing column over an empty object.
			continue
		}
		out = append(out, d)
	}
	return *got.Same && len(out) == 0, out, nil
}

// unfence strips a Markdown code fence from around an answer.
func unfence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[i+1:]
	}
	if i := strings.LastIndex(t, "```"); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}
