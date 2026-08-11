package share

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Conversation is what a share page holds, reduced to what an import needs.
type Conversation struct {
	// ID is the share id out of the URL, which is what makes an import
	// traceable back to the page it came from.
	ID string
	// Title is what the conversation is called. It is typed by a person, so it
	// is a hint about where the import belongs and never more than that.
	Title string
	// Turns are the assistant's answers, in the order they were given. The
	// user's messages are not kept: they are the ask, and the ask is not the
	// book.
	Turns []Turn
	// Asks are how many messages the person sent, which is how many pages were
	// handed over. It is kept for the manifest, since a conversation with more
	// asks than answers is one that lost a page somewhere.
	Asks int
}

// Turn is one answer.
type Turn struct {
	// Text is the answer as it was given, before any normalising.
	Text string
	// Model is what answered, when the page says. L08 exists because a small
	// model translated a section and nobody noticed, and the same argument
	// applies to a transcription.
	Model string
}

// shareID pulls the id out of a share URL. Anything else is refused rather than
// fetched, because a URL that is not a share URL is either a mistake or a
// private conversation, and neither should be downloaded on a guess.
var shareID = regexp.MustCompile(`^https://chatgpt\.com/share/([0-9a-f-]{8,})/?$`)

// ID reads the share id out of a URL.
func ID(url string) (string, error) {
	m := shareID.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return "", fmt.Errorf("%q is not a chatgpt share URL, which is https://chatgpt.com/share/<id>", url)
	}
	return m[1], nil
}

// userAgent is a browser's.
//
// Not to pretend to be one. The page is public and asks nothing, but it is
// served by a front end that gives an empty shell to a client it does not
// recognise, and an empty shell has no conversation in it. This is the shortest
// path to the same bytes a person opening the link would get.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// Fetch downloads a share page.
func Fetch(ctx context.Context, url string) (string, error) {
	if _, err := ID(url); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Parse reads a share page into a conversation.
func Parse(url, html string) (*Conversation, error) {
	id, err := ID(url)
	if err != nil {
		return nil, err
	}
	st, err := parseStream(html)
	if err != nil {
		return nil, err
	}
	root, err := st.root()
	if err != nil {
		return nil, err
	}
	data, ok := findMap(root, "linear_conversation")
	if !ok {
		return nil, fmt.Errorf("the page has no linear_conversation in it, so it is not a share page this reader knows")
	}
	c := &Conversation{ID: id, Title: str(data["title"])}
	list, _ := data["linear_conversation"].([]any)
	for _, node := range list {
		m, _ := node.(map[string]any)
		msg, _ := m["message"].(map[string]any)
		if msg == nil {
			continue
		}
		author, _ := msg["author"].(map[string]any)
		role := str(author["role"])
		content, _ := msg["content"].(map[string]any)
		if role == "user" {
			c.Asks++
			continue
		}
		// Only text. An assistant node is also written for the reasoning
		// summary and for the editable context, and neither is a page.
		if role != "assistant" || str(content["content_type"]) != "text" {
			continue
		}
		text := strings.TrimSpace(parts(content))
		if text == "" {
			continue
		}
		c.Turns = append(c.Turns, Turn{Text: text, Model: modelOf(msg)})
	}
	if len(c.Turns) == 0 {
		return nil, fmt.Errorf("the page has no assistant answers in it")
	}
	return c, nil
}

// parts joins the pieces of one message. A text message is normally one piece,
// but the field is a list and a page that arrives in two is still one page.
func parts(content map[string]any) string {
	list, _ := content["parts"].([]any)
	var b strings.Builder
	for _, p := range list {
		if s, ok := p.(string); ok {
			b.WriteString(s)
		}
	}
	return b.String()
}

func modelOf(msg map[string]any) string {
	meta, _ := msg["metadata"].(map[string]any)
	return str(meta["model_slug"])
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// findMap walks the decoded payload for the object that carries a key.
//
// By search and not by path. The route the loader data hangs off is named for
// the file that serves it, "share.$shareId.($action)", and that is a detail of
// somebody else's router. Looking for the conversation itself survives a
// rename; a hard coded path does not.
func findMap(v any, key string) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if _, ok := t[key]; ok {
			return t, true
		}
		for _, child := range t {
			if m, ok := findMap(child, key); ok {
				return m, true
			}
		}
	case []any:
		for _, child := range t {
			if m, ok := findMap(child, key); ok {
				return m, true
			}
		}
	}
	return nil, false
}
