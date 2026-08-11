package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The four files of tags/, and the log.
//
// tags is the record. new-tags is what the last assign run allocated, held back
// so that a person looks at it before it becomes permanent. inactive is where a
// tag goes when the statement it named leaves the corpus, and it is what stops
// the allocator from ever handing that tag out again. aliases says that a label
// was renamed, so that the tag follows the statement instead of being spent
// twice on it.
const (
	TagsFile     = "tags"
	NewTagsFile  = "new-tags"
	InactiveFile = "inactive"
	AliasesFile  = "aliases"
	LogFile      = "migrations.log"
)

// The header of each file if it has none yet. What is already at the top of a
// committed file is kept as it stands, since it is prose somebody wrote for the
// next reader and no writer of this program has anything to add to it.
var headers = map[string]string{
	TagsFile: `# List of current tags in the bourbaki corpus
# Each line is of the form
#	tag,full_label
`,
	NewTagsFile:  "",
	InactiveFile: "# Retired tags: TAG,full_label,reason,date\n# These tags are burned and must never be reassigned.\n",
	AliasesFile:  "# Label renames: OLD_LABEL,NEW_LABEL\n# The tag follows the label.\n",
}

// Dir is where the four files live in a checkout of the corpus.
func Dir(root string) string { return filepath.Join(root, "tags") }

// Path is one of them.
func Path(root, name string) string { return filepath.Join(Dir(root), name) }

// Entry is one line of tags or new-tags.
type Entry struct {
	Tag   Tag
	Label string
}

// Retired is one line of inactive: a tag that was spent and is now spent for
// good, with the reason somebody gave for taking the statement out.
type Retired struct {
	Tag    Tag
	Label  string
	Reason string
	Date   string
}

// Alias is one line of aliases, the old label and the label it became.
type Alias struct{ Old, New string }

// Set is the whole of tags/ in memory.
type Set struct {
	Tags     []Entry
	New      []Entry
	Inactive []Retired
	Aliases  []Alias

	// head is the comment block each file opens with, kept exactly as it was
	// read. tags is append-only and that is checked against its own history, so
	// a writer that reworded the header on every run would look like a file
	// that had lines taken out of it.
	head map[string]string
}

// Load reads tags/ out of a corpus checkout. A repository that has never had a
// tag assigned has no tags/ directory and loads empty, which is what lets the
// first assign run work at all.
func Load(root string) (*Set, error) {
	s := &Set{head: map[string]string{}}
	var err error
	for name, fallback := range headers {
		h, err := readHead(Path(root, name))
		if err != nil {
			return nil, err
		}
		if h == "" {
			h = fallback
		}
		s.head[name] = h
	}
	if s.Tags, err = readEntries(Path(root, TagsFile)); err != nil {
		return nil, err
	}
	if s.New, err = readEntries(Path(root, NewTagsFile)); err != nil {
		return nil, err
	}
	if s.Inactive, err = readInactive(Path(root, InactiveFile)); err != nil {
		return nil, err
	}
	if s.Aliases, err = readAliases(Path(root, AliasesFile)); err != nil {
		return nil, err
	}
	return s, nil
}

// Files renders the whole of tags/ as the bytes it goes to disk as, so that a
// caller can write it, diff it, or hold it next to what is committed.
//
// tags is sorted by tag, which for a sequentially allocated tag is the order it
// was assigned in, so an append to the corpus is an append to the file and the
// diff of a release is the list of what it added. new-tags keeps allocation
// order for the same reason and is not sorted again.
func (s *Set) Files(root string) map[string][]byte {
	sorted := append([]Entry(nil), s.Tags...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Tag < sorted[j].Tag })
	return map[string][]byte{
		Path(root, TagsFile):     entryBytes(s.header(TagsFile), sorted),
		Path(root, NewTagsFile):  entryBytes(s.header(NewTagsFile), s.New),
		Path(root, InactiveFile): inactiveBytes(s.header(InactiveFile), s.Inactive),
		Path(root, AliasesFile):  aliasBytes(s.header(AliasesFile), s.Aliases),
	}
}

func (s *Set) header(name string) string {
	if h, ok := s.head[name]; ok {
		return h
	}
	return headers[name]
}

// Save writes all four files, including the ones holding nothing but their
// header. They are part of the repository whether or not there is anything in
// them yet, and a reader who wants to know where a retired tag goes should find
// the file rather than have to be told it will appear one day.
func (s *Set) Save(root string) error {
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		return err
	}
	files := s.Files(root)
	for _, name := range []string{TagsFile, NewTagsFile, InactiveFile, AliasesFile} {
		if err := os.WriteFile(Path(root, name), files[Path(root, name)], 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Appendf adds a line to migrations.log, which is the one place a change to
// tags is explained in prose.
func Appendf(root, format string, args ...any) error {
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(Path(root, LogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, format+"\n", args...)
	return err
}

// Lookup is the label to tag mapping the rest of the corpus reads, over tags
// and new-tags together: a tag is real as soon as it is allocated, and merge
// only moves it from one file to the other.
//
// An alias resolves too, so that a renamed statement is found under its new
// label from the moment the alias is written and does not get a second tag
// spent on it while migrate waits for review.
func (s *Set) Lookup() map[string]Tag {
	out := make(map[string]Tag, len(s.Tags)+len(s.New))
	for _, e := range append(append([]Entry(nil), s.Tags...), s.New...) {
		out[e.Label] = e.Tag
	}
	for _, a := range s.Aliases {
		if t, ok := out[a.Old]; ok {
			if _, taken := out[a.New]; !taken {
				out[a.New] = t
			}
		}
	}
	return out
}

func entryBytes(head string, entries []Entry) []byte {
	var b strings.Builder
	b.WriteString(head)
	for _, e := range entries {
		fmt.Fprintf(&b, "%s,%s\n", e.Tag, e.Label)
	}
	return []byte(b.String())
}

func inactiveBytes(head string, retired []Retired) []byte {
	var b strings.Builder
	b.WriteString(head)
	for _, r := range retired {
		fmt.Fprintf(&b, "%s,%s,%s,%s\n", r.Tag, r.Label, r.Reason, r.Date)
	}
	return []byte(b.String())
}

func aliasBytes(head string, aliases []Alias) []byte {
	var b strings.Builder
	b.WriteString(head)
	for _, a := range aliases {
		fmt.Fprintf(&b, "%s,%s\n", a.Old, a.New)
	}
	return []byte(b.String())
}

// lines reads a file, drops comments and blank lines, and reports a missing
// file as no lines at all.
func lines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// readHead is the comment block a file opens with, which is kept as it stands.
// Anything after the first line that is not a comment is a record and is read
// by the parsers instead.
func readHead(path string) (string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "#") {
			break
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String(), nil
}

func readEntries(path string) ([]Entry, error) {
	ls, err := lines(path)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(ls))
	for i, line := range ls {
		tag, label, ok := strings.Cut(line, ",")
		if !ok {
			return nil, fmt.Errorf("%s line %d: %q is not tag,full_label", path, i+1, line)
		}
		t, err := Parse(tag)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		if label == "" {
			return nil, fmt.Errorf("%s line %d: tag %s has no label", path, i+1, t)
		}
		out = append(out, Entry{Tag: t, Label: label})
	}
	return out, nil
}

func readInactive(path string) ([]Retired, error) {
	ls, err := lines(path)
	if err != nil {
		return nil, err
	}
	out := make([]Retired, 0, len(ls))
	for i, line := range ls {
		f := strings.Split(line, ",")
		if len(f) != 4 {
			return nil, fmt.Errorf("%s line %d: %q is not tag,full_label,reason,date", path, i+1, line)
		}
		t, err := Parse(f[0])
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		out = append(out, Retired{Tag: t, Label: f[1], Reason: f[2], Date: f[3]})
	}
	return out, nil
}

func readAliases(path string) ([]Alias, error) {
	ls, err := lines(path)
	if err != nil {
		return nil, err
	}
	out := make([]Alias, 0, len(ls))
	for i, line := range ls {
		old, new, ok := strings.Cut(line, ",")
		if !ok || old == "" || new == "" {
			return nil, fmt.Errorf("%s line %d: %q is not old_label,new_label", path, i+1, line)
		}
		out = append(out, Alias{Old: old, New: new})
	}
	return out, nil
}
