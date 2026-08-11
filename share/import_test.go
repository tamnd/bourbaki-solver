package share

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetPathIsTheStandardLayout(t *testing.T) {
	cases := map[string]string{
		"intro": "imports/sets/intro.md",
		"1.1":   "imports/sets/chapter_1/1.1.md",
		"1.3":   "imports/sets/chapter_1/1.3.md",
		"2.10":  "imports/sets/chapter_2/2.10.md",
		"11.4":  "imports/sets/chapter_11/11.4.md",
	}
	for label, want := range cases {
		got, err := ParseTarget("sets", label)
		if err != nil {
			t.Errorf("%s: %v", label, err)
			continue
		}
		if p := filepath.ToSlash(got.Path()); p != want {
			t.Errorf("%s went to %s, want %s", label, p, want)
		}
	}
}

func TestParseTargetRefusesWhatIsNotAPlaceInABook(t *testing.T) {
	for _, label := range []string{"", "one.one", "1", "1.1.1", "0.1", "1.0", "chapter 1", "§1"} {
		if _, err := ParseTarget("sets", label); err == nil {
			t.Errorf("%q was accepted", label)
		}
	}
	if _, err := ParseTarget("", "1.1"); err == nil {
		t.Error("an import with no book was accepted")
	}
}

func TestTargetFromTitleReadsTheFourTitlesThatExist(t *testing.T) {
	cases := map[string]string{
		"Bourbaki Sets - Intro": "imports/sets/intro.md",
		"Bourbaki Sets - 1.1":   "imports/sets/chapter_1/1.1.md",
		"Bourbaki Sets - 1.2":   "imports/sets/chapter_1/1.2.md",
		"Bourbaki Sets - 1.3":   "imports/sets/chapter_1/1.3.md",
	}
	for title, want := range cases {
		got, err := TargetFromTitle("sets", title)
		if err != nil {
			t.Errorf("%q: %v", title, err)
			continue
		}
		if p := filepath.ToSlash(got.Path()); p != want {
			t.Errorf("%q went to %s, want %s", title, p, want)
		}
	}
	// A title that does not say where it goes is an error and never a guess.
	// The label is one word on the command line and a wrong guess is a file in
	// the wrong place with the right name, which is the hardest kind to notice.
	for _, title := range []string{"Bourbaki Sets", "", "New chat", "Bourbaki Sets - chapter one"} {
		if _, err := TargetFromTitle("sets", title); err == nil {
			t.Errorf("%q was guessed at", title)
		}
	}
}

func TestImportWritesTheProvenance(t *testing.T) {
	conv := &Conversation{
		ID:    "6a7af1eb-3f74-83ec-9612-45e6992e80d6",
		Title: "Bourbaki Sets - 1.1",
		Asks:  3,
		Turns: []Turn{
			{Text: `Let \(A\) be an assembly.`, Model: "gpt-5-6-thinking"},
			{Text: `It is of the first species.`, Model: "gpt-5-6-thinking"},
		},
	}
	page, err := Markdown(conv)
	if err != nil {
		t.Fatal(err)
	}
	target, err := ParseTarget("sets", "1.1")
	if err != nil {
		t.Fatal(err)
	}
	im := &Import{Target: target, Page: page, Conv: conv,
		URL: "https://chatgpt.com/share/6a7af1eb-3f74-83ec-9612-45e6992e80d6"}

	root := t.TempDir()
	rel, err := im.Write(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.ToSlash(rel) != "imports/sets/chapter_1/1.1.md" {
		t.Errorf("wrote %s", rel)
	}
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		"book: sets",
		"chapter: 1",
		"section: 1",
		"extraction: share",
		"share_url: https://chatgpt.com/share/6a7af1eb-3f74-83ec-9612-45e6992e80d6",
		"share_title: Bourbaki Sets - 1.1",
		"asks: 3",
		"answers: 2",
		"models: [gpt-5-6-thinking]",
		"joined: 0 of 1",
		"Let $A$ be an assembly.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the file does not say %q:\n%s", want, got)
		}
	}

	// The digest is of the answers as they arrived, so a change to the
	// rendering here still says it is the same conversation, and a fetch that
	// came back different says so.
	was := im.Sum()
	im.Page.Body = "something else entirely"
	if im.Sum() != was {
		t.Error("the digest moved when the rendering changed")
	}
	im.Conv.Turns[0].Text = "a different page"
	if im.Sum() == was {
		t.Error("the digest did not move when the answers changed")
	}
}
