// Package gtranslate drafts translations with the free Google Translate
// endpoints, the way the Python packages googletrans and deep-translator do,
// and wraps them in enough protection that the draft keeps the mathematics,
// the Markdown and the corpus's terminology intact.
//
// A draft from here is marked low quality in the file it is written to. It is
// there so a reader has something in their language today, and a translate run
// on a model replaces it later.
package gtranslate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MaxChars is the most one request carries. deep-translator holds the free
// endpoint to 5000 characters; a little under it leaves room for the
// percent-encoding of the placeholders.
const MaxChars = 4500

// An endpoint is one way of asking. Both answer the same question and fail
// differently, so a client rotates through them when one is refusing.
type endpoint struct {
	name  string
	build func(ctx context.Context, sl, tl, text string) (*http.Request, error)
	parse func([]byte) (string, error)
}

// gtx is translate_a/single with client=gtx, the endpoint googletrans and most
// of its descendants use. POST, so a long passage does not become a long URL.
var gtx = endpoint{
	name: "gtx",
	build: func(ctx context.Context, sl, tl, text string) (*http.Request, error) {
		q := url.Values{"client": {"gtx"}, "sl": {sl}, "tl": {tl}, "dt": {"t"}, "dj": {"1"}}
		body := url.Values{"q": {text}}.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://translate.googleapis.com/translate_a/single?"+q.Encode(), strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
		return req, nil
	},
	parse: func(b []byte) (string, error) {
		var r struct {
			Sentences []struct {
				Trans string `json:"trans"`
			} `json:"sentences"`
		}
		if err := json.Unmarshal(b, &r); err != nil {
			return "", fmt.Errorf("gtx answered something that is not its JSON: %w", err)
		}
		var out strings.Builder
		for _, s := range r.Sentences {
			out.WriteString(s.Trans)
		}
		return out.String(), nil
	},
}

// dict is the endpoint the Chrome dictionary extension uses. It answers a list
// with one translation per q.
var dict = endpoint{
	name: "dict",
	build: func(ctx context.Context, sl, tl, text string) (*http.Request, error) {
		q := url.Values{"client": {"dict-chrome-ex"}, "sl": {sl}, "tl": {tl}}
		body := url.Values{"q": {text}}.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://clients5.google.com/translate_a/t?"+q.Encode(), strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
		return req, nil
	},
	parse: func(b []byte) (string, error) {
		var flat []string
		if err := json.Unmarshal(b, &flat); err == nil && len(flat) > 0 {
			return flat[0], nil
		}
		// With a source language other than auto it sometimes answers pairs.
		var pairs [][]string
		if err := json.Unmarshal(b, &pairs); err == nil && len(pairs) > 0 && len(pairs[0]) > 0 {
			return pairs[0][0], nil
		}
		return "", fmt.Errorf("dict answered something that is not its JSON: %.80s", b)
	},
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0",
}

// errLimited is Google saying slow down, which a 429 or its "Sorry" page is.
var errLimited = errors.New("google is rate limiting this address")

// Client asks Google, politely and with a memory.
type Client struct {
	HTTP *http.Client
	// Gap is the least time between two requests. The free endpoint starts
	// answering 429 to an address that asks several times a second.
	Gap time.Duration
	// Retries is how many times one passage is asked before giving up.
	Retries int
	// Cache is a directory of answers keyed by the question, so a run that
	// stops costs nothing to start again. Empty means no cache.
	Cache string

	mu        sync.Mutex
	last      time.Time
	endpoints []endpoint
	next      int
	Requests  int // asks that went over the network
}

// New is a client with the defaults a long run wants.
func New(cache string) *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 60 * time.Second},
		Gap:       1200 * time.Millisecond,
		Retries:   6,
		Cache:     cache,
		endpoints: []endpoint{gtx, dict},
	}
}

// Translate puts text from sl into tl. Text longer than MaxChars goes in
// pieces cut at line breaks and comes back joined the same way.
func (c *Client) Translate(ctx context.Context, sl, tl, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}
	var out []string
	for _, piece := range Split(text, MaxChars) {
		got, err := c.one(ctx, sl, tl, piece)
		if err != nil {
			return "", err
		}
		out = append(out, got)
	}
	return strings.Join(out, "\n"), nil
}

// Split cuts text into pieces of at most max bytes, at line breaks where it
// can and at spaces where a single line is longer than that. Joining the
// pieces with "\n" gives back the text when every cut was at a line break.
func Split(text string, max int) []string {
	if len(text) <= max {
		return []string{text}
	}
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, line := range strings.Split(text, "\n") {
		for len(line) > max {
			cut := strings.LastIndex(line[:max], " ")
			if cut <= 0 {
				cut = max
			}
			flush()
			out = append(out, line[:cut])
			line = strings.TrimLeft(line[cut:], " ")
		}
		if cur.Len() > 0 && cur.Len()+1+len(line) > max {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
	}
	flush()
	return out
}

func (c *Client) cachePath(sl, tl, text string) string {
	if c.Cache == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(sl + "\x00" + tl + "\x00" + text))
	h := hex.EncodeToString(sum[:])
	return filepath.Join(c.Cache, h[:2], h+".txt")
}

func (c *Client) one(ctx context.Context, sl, tl, text string) (string, error) {
	path := c.cachePath(sl, tl, text)
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			return string(b), nil
		}
	}
	var last error
	tries := max(c.Retries, 1)
	for attempt := 0; attempt < tries; attempt++ {
		if attempt > 0 {
			// Exponential with jitter, longer when Google said slow down.
			wait := time.Duration(1<<attempt) * time.Second
			if errors.Is(last, errLimited) {
				wait *= 5
			}
			wait += time.Duration(rand.Int64N(int64(time.Second)))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
		}
		got, err := c.ask(ctx, c.endpoint(attempt), sl, tl, text)
		if err == nil {
			if path != "" {
				_ = os.MkdirAll(filepath.Dir(path), 0o755)
				_ = os.WriteFile(path, []byte(got), 0o644)
			}
			return got, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		last = err
	}
	return "", fmt.Errorf("google translate gave up after %d tries: %w", tries, last)
}

// endpoint rotates: the first try of a passage goes to the endpoint the last
// passage succeeded on, and each retry moves to the next one.
func (c *Client) endpoint(attempt int) endpoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.endpoints) == 0 {
		c.endpoints = []endpoint{gtx, dict}
	}
	if attempt > 0 {
		c.next = (c.next + 1) % len(c.endpoints)
	}
	return c.endpoints[c.next]
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	gap := c.Gap - time.Since(c.last)
	if gap < 0 {
		gap = 0
	}
	c.last = time.Now().Add(gap)
	c.Requests++
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(gap):
		return nil
	}
}

func (c *Client) ask(ctx context.Context, e endpoint, sl, tl, text string) (string, error) {
	if err := c.wait(ctx); err != nil {
		return "", err
	}
	req, err := e.build(ctx, sl, tl, text)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgents[rand.IntN(len(userAgents))])
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || strings.Contains(string(b[:min(len(b), 512)]), "<title>Sorry"):
		return "", fmt.Errorf("%s: %w", e.name, errLimited)
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("%s: HTTP %d", e.name, resp.StatusCode)
	}
	got, err := e.parse(b)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(got) == "" {
		return "", fmt.Errorf("%s: an empty translation", e.name)
	}
	return got, nil
}
