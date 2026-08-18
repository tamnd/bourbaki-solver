package tags

import (
	"fmt"
	"sort"
	"strings"
)

// A run is one merge: the tags that became permanent together.
//
// The allocator walks the tag space from the bottom and hands out what nothing
// has taken, so the tags of one merge are the next block of the space and the
// block begins where the run before it left off. That makes a run a boundary
// and nothing more, and tags/runs records the boundaries rather than the five
// thousand lines they cut.
//
// The reason to keep them at all is T10. A file's tags climb on the run that
// assigned them, because the run reads the file from the top. They do not climb
// across runs: read a volume in August, add the exercise a later printing puts
// in the middle of § 3 in September, and it takes a tag larger than everything
// under it. Both readings are correct and only one of them is a file whose tags
// climb. Without the boundaries there is no way to tell that from a heading
// somebody pasted the wrong tag on to, which is what T10 is for, and the corpus
// had 236 findings of which every one was the first thing.
type Run struct {
	// First is the lowest tag the run allocated, and the run holds every tag
	// from there up to the First of the run after it.
	First Tag
	Date  string
	// Note is what the run read, in a few words, for somebody reading the file
	// a year from now.
	Note string
}

// Runs are held in order and RunAt is what reads them, so a file that is out of
// order is a file that answers wrongly rather than one that fails. The loader
// sorts, which costs nothing at this size and means a hand edit that appends a
// line in the wrong place still works.
func sortRuns(runs []Run) {
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].First < runs[j].First })
}

// RunAt is the run that assigned a tag, as an index into the runs given. A tag
// below the first boundary, which is what every tag is in a corpus that has
// never recorded one, belongs to run zero: with no boundaries at all this says
// every tag came from one run and T10 reads exactly as it did before this file
// existed.
func RunAt(runs []Run, t Tag) int {
	at := 0
	for i, r := range runs {
		if t < r.First {
			break
		}
		at = i
	}
	return at
}

// Open records a boundary. It is called by merge with the lowest tag of what it
// made permanent, and a merge whose tags fall inside a run already recorded is
// not a new run: that is a second merge of the same reading, and the two of them
// assigned one file from the top once.
func (s *Set) Open(first Tag, date, note string) {
	// The file is comma separated and the note is prose somebody types, so a
	// comma in it is taken out rather than written and read back as a field
	// boundary somewhere down the line.
	if commas(note) {
		note = strings.Join(strings.Fields(strings.ReplaceAll(note, ",", " ")), " ")
	}
	for _, r := range s.Runs {
		if r.First == first {
			return
		}
	}
	s.Runs = append(s.Runs, Run{First: first, Date: date, Note: note})
	sortRuns(s.Runs)
}

func runBytes(head string, runs []Run) []byte {
	var b strings.Builder
	b.WriteString(head)
	for _, r := range runs {
		fmt.Fprintf(&b, "%s,%s,%s\n", r.First, r.Date, r.Note)
	}
	return []byte(b.String())
}

func readRuns(path string) ([]Run, error) {
	ls, err := lines(path)
	if err != nil {
		return nil, err
	}
	out := make([]Run, 0, len(ls))
	for i, line := range ls {
		f := strings.SplitN(line, ",", 3)
		if len(f) != 3 {
			return nil, fmt.Errorf("%s line %d: %q is not first_tag,date,what the run read", path, i+1, line)
		}
		t, err := Parse(f[0])
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		out = append(out, Run{First: t, Date: f[1], Note: f[2]})
	}
	sortRuns(out)
	return out, nil
}
