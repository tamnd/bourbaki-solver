package share

import (
	"strconv"
	"strings"
	"testing"
)

// page builds a share page around a turbo-stream payload, the way the real one
// arrives: one enqueue call carrying a JavaScript string literal whose content
// is the stream.
func page(lines ...string) string {
	payload := strconv.Quote(strings.Join(lines, "\n"))
	return `<!doctype html><html><body><script>window.__reactRouterContext.streamController.enqueue(` +
		payload + `);</script></body></html>`
}

// fixture is a conversation of one ask and one answer, written out by hand in
// the interned form the real payload uses. The indices are the point of it: the
// key "title" is element 1 and is referred to as "_1", and every value is an
// index too, which is why the payload cannot simply be unmarshalled.
const fixture = `[{"_1":2,"_3":4,"_21":22},` +
	`"title","Bourbaki Sets - 9.9",` +
	`"linear_conversation",[5,25],` +
	`{"_6":7},"message",{"_8":9,"_12":13,"_19":20},` +
	`"author",{"_10":11},"role","assistant",` +
	`"content",{"_14":15,"_16":17},"content_type","text","parts",[18],` +
	`"Let \\(A\\) be a ring.",` +
	`"metadata",{"_23":24},` +
	`"deferred",["P",100],` +
	`"model_slug","gpt-5-6-thinking",` +
	`{"_6":26},{"_8":27,"_12":29},{"_10":28},"user",{"_14":15,"_16":30},[31],"which page is this"]`

func TestStreamResolvesTheInternedReferences(t *testing.T) {
	conv, err := Parse("https://chatgpt.com/share/6a7af1eb-3f74-83ec-9612-45e6992e80d6",
		page(fixture, `P100:["resolved"]`))
	if err != nil {
		t.Fatal(err)
	}
	if conv.Title != "Bourbaki Sets - 9.9" {
		t.Errorf("title is %q", conv.Title)
	}
	if conv.ID != "6a7af1eb-3f74-83ec-9612-45e6992e80d6" {
		t.Errorf("id is %q", conv.ID)
	}
	if len(conv.Turns) != 1 {
		t.Fatalf("got %d answers, want 1: %v", len(conv.Turns), conv.Turns)
	}
	if conv.Turns[0].Text != `Let \(A\) be a ring.` {
		t.Errorf("the answer is %q", conv.Turns[0].Text)
	}
	if conv.Turns[0].Model != "gpt-5-6-thinking" {
		t.Errorf("the model is %q", conv.Turns[0].Model)
	}
	// The user's message is counted and not kept. A conversation with more asks
	// than answers lost a page, and that is worth knowing; the ask itself is
	// not the book.
	if conv.Asks != 1 {
		t.Errorf("got %d asks, want 1", conv.Asks)
	}
}

// The payload arrives in pieces, and a value is allowed to be cut in half by
// the boundary between two of them, which is why the chunks are concatenated
// before anything is parsed. The real Theory of Sets pages arrive in one piece
// each, so this is the case that has never happened and would be silent if it
// did: a value cut in half parses as two shorter values and the conversation
// comes back with a hole in it rather than an error.
func TestStreamJoinsTheChunksBeforeParsingThem(t *testing.T) {
	half := len(fixture) / 2
	split := `<!doctype html><html><body><script>` +
		`window.__reactRouterContext.streamController.enqueue(` + strconv.Quote(fixture[:half]) + `);` +
		`window.__reactRouterContext.streamController.enqueue(` + strconv.Quote(fixture[half:]) + `);` +
		`</script></body></html>`
	url := "https://chatgpt.com/share/6a7af1eb-3f74-83ec-9612-45e6992e80d6"
	got, err := Parse(url, split)
	if err != nil {
		t.Fatal(err)
	}
	want, err := Parse(url, page(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Turns) != len(want.Turns) || got.Turns[0].Text != want.Turns[0].Text {
		t.Errorf("split across two chunks gave %+v, want %+v", got.Turns, want.Turns)
	}
}

func TestStreamRefusesAPageThatIsNotOne(t *testing.T) {
	for name, html := range map[string]string{
		"no stream":   `<!doctype html><html><body>sign in to continue</body></html>`,
		"no messages": page(`[{"_1":2},"title","Bourbaki Sets - 9.9"]`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse("https://chatgpt.com/share/6a7af1eb-3f74-83ec-9612-45e6992e80d6", html); err == nil {
				t.Error("a page with nothing in it was accepted")
			}
		})
	}
}

func TestIDRefusesWhatIsNotAShareLink(t *testing.T) {
	for _, url := range []string{
		"https://chatgpt.com/c/6a7af1eb-3f74-83ec-9612-45e6992e80d6",
		"https://example.com/share/6a7af1eb",
		"http://chatgpt.com/share/6a7af1eb-3f74-83ec-9612-45e6992e80d6",
		"",
	} {
		if _, err := ID(url); err == nil {
			t.Errorf("%q was accepted as a share link", url)
		}
	}
	if got, err := ID(" https://chatgpt.com/share/6a7af1eb-3f74-83ec-9612-45e6992e80d6 "); err != nil || got != "6a7af1eb-3f74-83ec-9612-45e6992e80d6" {
		t.Errorf("got %q, %v", got, err)
	}
}
