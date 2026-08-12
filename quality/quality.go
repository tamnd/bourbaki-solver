// Package quality is the corpus audit: every quality claim this project makes,
// written down as a rule that runs.
//
// The rules are the ones in spec 08 §2, and they keep the numbering they have
// there, because a CI log that says S07 has to mean the same thing as the
// checklist that says S07. Each is a named function of the committed Markdown
// and the manifests, and of nothing else. Nothing here opens a PDF, calls a
// model or reaches the network, which is what lets the whole audit run in CI,
// where the PDFs are not and cannot be, and what makes two runs of it on the
// same commit give the same answer.
//
// A hard rule failing means the corpus is wrong. A soft rule failing means
// somebody should look. The exit status is the hard rules alone, so the soft
// ones can be honest about how much they do not know without turning the build
// red for it.
//
// The audit is expected to be red for a while. taocp's README says its complete
// audit "intentionally exits nonzero until all reported hard failures are
// repaired", and the same is true here: a rule written so that today's corpus
// passes it is a rule that measures nothing.
package quality

import (
	"fmt"
	"sort"
	"strings"
)

// A Finding is one thing wrong, at a place somebody can open.
//
// File and Line are what make this worth having over a count. An audit that
// says seven files have stranded operators sends the reader back to grep; one
// that says which line does not.
type Finding struct {
	Check string `json:"check"`
	Group string `json:"group"`
	Hard  bool   `json:"hard"`
	File  string `json:"file,omitempty"` // relative to the corpus root
	Line  int    `json:"line,omitempty"`
	Msg   string `json:"message"`
}

// At is the finding's place, in the form an editor and a CI annotation both
// understand.
func (f Finding) At() string {
	switch {
	case f.File == "":
		return ""
	case f.Line > 0:
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	return f.File
}

func (f Finding) String() string {
	if at := f.At(); at != "" {
		return fmt.Sprintf("%s  %s  %s", f.Check, at, f.Msg)
	}
	return fmt.Sprintf("%s  %s", f.Check, f.Msg)
}

// A Check is one rule.
//
// Run returns what it found. It returns an error only when it could not look:
// a manifest that will not parse is an error, a manifest that says something
// wrong is a finding. Keeping those apart is what lets one broken rule fail
// loudly without the other forty going quiet.
type Check struct {
	ID    string
	Group string
	Hard  bool
	Title string
	Run   func(*Corpus) ([]Finding, error)

	// Need says why this rule cannot run against this corpus, and is empty
	// when it can. A shallow checkout has no history for T05 and a corpus with
	// no Vietnamese has nothing for L01 to compare; both are reported as not
	// run, because a rule that passes by having nothing to look at is the one
	// kind of green nobody should trust.
	Need func(*Corpus) string
}

// The groups, in the order the report prints them.
const (
	Structure   = "structure"
	Tags        = "tags"
	Mathematics = "mathematics"
	Figures     = "figures"
	References  = "references"
	Translation = "translation"
	Solutions   = "solutions"
	Publication = "publication"
	Hygiene     = "hygiene"
)

var groupOrder = []string{
	Structure, Tags, Mathematics, Figures,
	References, Translation, Solutions, Publication, Hygiene,
}

// checks is every rule, in the order spec 08 lists them. Registration is a
// plain slice rather than an init-time map because the order the report reads
// in is the order they are written down in, and a map has none.
var checks []Check

func register(c ...Check) { checks = append(checks, c...) }

// Checks is every rule the audit knows, for the command that lists them.
func Checks() []Check { return append([]Check(nil), checks...) }

// GroupOrder is the groups in the order spec 08 writes them, which is the order
// anything printing the rules should use.
func GroupOrder() []string { return append([]string(nil), groupOrder...) }

// An Outcome is one rule's run.
//
// Skipped carries why a rule did not run, and it is printed rather than
// swallowed. A shallow CI checkout cannot read the history of tags/tags and a
// corpus with no Vietnamese cannot check a Vietnamese translation; both are
// fine, and an audit that reported them as passes would be lying about its own
// coverage.
type Outcome struct {
	Check    Check
	Findings []Finding
	Skipped  string
}

// A Result is the whole run.
type Result struct {
	Outcomes []Outcome
	Corpus   *Corpus
}

// Run loads the corpus once and puts every selected rule over it.
func Run(opt Options) (*Result, error) {
	sel, err := opt.selection()
	if err != nil {
		return nil, err
	}
	c, err := Load(opt)
	if err != nil {
		return nil, err
	}
	res := &Result{Corpus: c}
	for _, ck := range checks {
		if !sel(ck) {
			continue
		}
		out := Outcome{Check: ck}
		if ck.Need != nil {
			if why := ck.Need(c); why != "" {
				out.Skipped = why
				res.Outcomes = append(res.Outcomes, out)
				continue
			}
		}
		found, err := ck.Run(c)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ck.ID, err)
		}
		for i := range found {
			found[i].Check, found[i].Group, found[i].Hard = ck.ID, ck.Group, ck.Hard
		}
		sort.SliceStable(found, func(i, j int) bool {
			if found[i].File != found[j].File {
				return found[i].File < found[j].File
			}
			return found[i].Line < found[j].Line
		})
		out.Findings = found
		res.Outcomes = append(res.Outcomes, out)
	}
	return res, nil
}

// Findings is every finding of the run, hard and soft together.
func (r *Result) Findings() []Finding {
	var out []Finding
	for _, o := range r.Outcomes {
		out = append(out, o.Findings...)
	}
	return out
}

// Hard is how many findings came from a rule the corpus is held to. It is the
// exit status: zero means the corpus is as good as this audit can tell, and
// anything else means it is not.
func (r *Result) Hard() int {
	n := 0
	for _, o := range r.Outcomes {
		if o.Check.Hard {
			n += len(o.Findings)
		}
	}
	return n
}

// Soft is how many findings came from a rule that only asks for a look.
func (r *Result) Soft() int {
	n := 0
	for _, o := range r.Outcomes {
		if !o.Check.Hard {
			n += len(o.Findings)
		}
	}
	return n
}

// Failing is the rules with findings, and Skipped the rules that could not run.
func (r *Result) Failing() []Outcome {
	var out []Outcome
	for _, o := range r.Outcomes {
		if len(o.Findings) > 0 {
			out = append(out, o)
		}
	}
	return out
}

func (r *Result) Skipped() []Outcome {
	var out []Outcome
	for _, o := range r.Outcomes {
		if o.Skipped != "" {
			out = append(out, o)
		}
	}
	return out
}

// selection turns -only and -skip into a predicate.
//
// Both take a rule's own name, S07, or the group it belongs to, structure. The
// group form is what makes the flags usable: fixing the mathematics of a
// chapter means running M over and over, and typing six identifiers each time
// is how people stop running the audit at all.
func (o Options) selection() (func(Check) bool, error) {
	only, err := parseSet(o.Only)
	if err != nil {
		return nil, err
	}
	skip, err := parseSet(o.Skip)
	if err != nil {
		return nil, err
	}
	return func(c Check) bool {
		if len(only) > 0 && !only[strings.ToLower(c.ID)] && !only[c.Group] {
			return false
		}
		return !skip[strings.ToLower(c.ID)] && !skip[c.Group]
	}, nil
}

// Selected answers whether -only and -skip leave this rule in. It is for a
// caller that has to do expensive work for one rule and should not do it when
// the rule is not going to run: the audit runs the whole assembler for S09, and
// running it for bourbaki audit -only hygiene would be a minute of nothing.
func (o Options) Selected(id string) bool {
	sel, err := o.selection()
	if err != nil {
		return true // the flags are wrong, and Run is where that is reported
	}
	for _, c := range checks {
		if strings.EqualFold(c.ID, id) {
			return sel(c)
		}
	}
	return false
}

func parseSet(names []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, raw := range names {
		for _, n := range strings.Split(raw, ",") {
			n = strings.ToLower(strings.TrimSpace(n))
			if n == "" {
				continue
			}
			if !known(n) {
				return nil, fmt.Errorf("no check or group called %q", n)
			}
			out[n] = true
		}
	}
	return out, nil
}

func known(n string) bool {
	for _, g := range groupOrder {
		if n == g {
			return true
		}
	}
	for _, c := range checks {
		if n == strings.ToLower(c.ID) {
			return true
		}
	}
	return false
}

// Groups counts what ran, per group, for the summary table.
func (r *Result) Groups() []GroupSummary {
	by := map[string]*GroupSummary{}
	for _, o := range r.Outcomes {
		g, ok := by[o.Check.Group]
		if !ok {
			g = &GroupSummary{Group: o.Check.Group}
			by[o.Check.Group] = g
		}
		switch {
		case o.Skipped != "":
			g.NotRun++
		case o.Check.Hard:
			g.Hard++
		default:
			g.Soft++
		}
		if len(o.Findings) > 0 {
			g.Failing = append(g.Failing, fmt.Sprintf("%s (%d)", o.Check.ID, len(o.Findings)))
		}
	}
	var out []GroupSummary
	for _, name := range groupOrder {
		if g, ok := by[name]; ok {
			out = append(out, *g)
		}
	}
	return out
}

// GroupSummary is one row of the summary table.
type GroupSummary struct {
	Group   string
	Hard    int
	Soft    int
	NotRun  int
	Failing []string
}
