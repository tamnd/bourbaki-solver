// Package share reads a public ChatGPT share page.
//
// A share link is the cheapest source of a transcription this project has. The
// OCR path drives a browser on a server, waits about 150 seconds a page and
// then argues with what comes back; a share link is a conversation somebody
// already had, already answered, and made public. Reading one is an HTTP GET.
//
// It is also the only path here that touches nothing else. It does not use the
// fleet, it does not sign in, it does not need a profile, and it writes to a
// tree of its own. That is deliberate: an import is raw material and not
// corpus, and it should not be able to reach the corpus by accident.
//
// The page carries the whole conversation in its HTML. The DOM does not, since
// the client only mounts the last few turns, but the server ships the loader
// data for the route inline, and that has every message in it. This file is
// about getting it out.
package share

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The format is React Router's turbo-stream, which is a flat array with the
// values interned and referred to by index. The root is element 0, an object is
// written as {"_<key index>": <value index>}, an array as a list of indices, a
// negative index is one of a handful of constants, and a list whose first
// element is a string is a tagged value rather than an array.
//
// Interning is why it cannot simply be unmarshalled. The same string appears
// once however many messages use it, and every reference to it is a number, so
// json.Unmarshal into a struct gets an array of unrelated fragments. Resolving
// the references is the whole of the work here.
//
// Measured on the four Theory of Sets share pages of 11 August 2026: 425 to
// about 2,000 elements each, keys always of the form _N, values always
// integers, one constant in use (-5) and one tag (P). The decoder handles more
// than that, because the shape is not ours and a page that uses a second
// constant should not come back as a wrong answer.
const (
	constHole  = -1
	constNaN   = -2
	constNegI  = -3
	constNegZ  = -4
	constNull  = -5
	constPosI  = -6
	constUndef = -7
)

// enqueue is how the payload is delivered: one or more calls, each with a
// JavaScript string literal that is a piece of the stream. They are
// concatenated before anything is parsed, because a value is allowed to
// straddle the join.
var enqueue = regexp.MustCompile(`streamController\.enqueue\("((?:[^"\\]|\\.)*)"\)`)

// stream is a decoded payload: the base array and whatever arrived later
// against a promise.
type stream struct {
	base     []json.RawMessage
	deferred map[int][]json.RawMessage
	memo     map[int]any
}

// parseStream pulls the turbo-stream out of a share page's HTML.
func parseStream(html string) (*stream, error) {
	chunks := enqueue.FindAllStringSubmatch(html, -1)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no react router stream in the page, so either the page is not a share page or the format has moved")
	}
	var b strings.Builder
	for _, c := range chunks {
		// The capture is the body of a JavaScript string literal, and a JSON
		// string literal is the same thing for everything that appears here.
		var s string
		if err := json.Unmarshal([]byte(`"`+c[1]+`"`), &s); err != nil {
			return nil, fmt.Errorf("a stream chunk is not a string literal: %w", err)
		}
		b.WriteString(s)
	}
	st := &stream{deferred: map[int][]json.RawMessage{}, memo: map[int]any{}}
	for i, line := range strings.Split(b.String(), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if i == 0 {
			if err := json.Unmarshal([]byte(line), &st.base); err != nil {
				return nil, fmt.Errorf("the stream base is not an array: %w", err)
			}
			continue
		}
		// A later line resolves a promise: "P1022:[{}]".
		colon := strings.Index(line, ":")
		if colon < 2 || line[0] != 'P' {
			continue
		}
		id, err := strconv.Atoi(line[1:colon])
		if err != nil {
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(line[colon+1:]), &arr); err != nil {
			continue
		}
		st.deferred[id] = arr
	}
	if len(st.base) == 0 {
		return nil, fmt.Errorf("the stream is empty")
	}
	return st, nil
}

// root decodes the whole payload into ordinary Go values.
func (s *stream) root() (any, error) { return s.at(0) }

// at decodes one element, resolving what it refers to.
//
// The memo is not an optimisation, or not only one. The graph is a graph: the
// same object is referred to from several places, and a share page of a long
// conversation refers to the same author object once per message. Without the
// memo a page with a cycle in it would not return at all.
func (s *stream) at(i int) (any, error) {
	if i < 0 {
		switch i {
		case constNull, constUndef, constHole:
			return nil, nil
		case constNaN, constNegI, constPosI, constNegZ:
			return nil, nil // numbers this reader has no use for
		}
		return nil, nil
	}
	if i >= len(s.base) {
		return nil, fmt.Errorf("index %d is past the end of a %d element stream", i, len(s.base))
	}
	if v, ok := s.memo[i]; ok {
		return v, nil
	}
	raw := s.base[i]
	switch first(raw) {
	case '{':
		var fields map[string]int
		if err := json.Unmarshal(raw, &fields); err != nil {
			// An object whose values are not all indices is not something this
			// format produces, and guessing at it would be worse than saying so.
			return nil, fmt.Errorf("element %d is an object this reader does not understand: %w", i, err)
		}
		out := map[string]any{}
		s.memo[i] = out
		for k, vi := range fields {
			key := k
			if strings.HasPrefix(k, "_") {
				ki, err := strconv.Atoi(k[1:])
				if err != nil {
					return nil, fmt.Errorf("element %d has key %q, which is neither a name nor an index", i, k)
				}
				kv, err := s.at(ki)
				if err != nil {
					return nil, err
				}
				ks, ok := kv.(string)
				if !ok {
					return nil, fmt.Errorf("element %d has a key at %d that is not a string", i, ki)
				}
				key = ks
			}
			v, err := s.at(vi)
			if err != nil {
				return nil, err
			}
			out[key] = v
		}
		return out, nil
	case '[':
		var head []json.RawMessage
		if err := json.Unmarshal(raw, &head); err != nil {
			return nil, fmt.Errorf("element %d is a malformed array: %w", i, err)
		}
		if len(head) > 0 && first(head[0]) == '"' {
			return s.tagged(head)
		}
		out := make([]any, 0, len(head))
		s.memo[i] = &out
		for _, e := range head {
			var idx int
			if err := json.Unmarshal(e, &idx); err != nil {
				return nil, fmt.Errorf("element %d holds %s, which is not an index", i, e)
			}
			v, err := s.at(idx)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		s.memo[i] = out
		return out, nil
	default:
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("element %d is not a value: %w", i, err)
		}
		s.memo[i] = v
		return v, nil
	}
}

// tagged decodes a value the format marks with a leading string. Only P, a
// promise, carries anything this reader wants, and an unresolved one is nil
// rather than an error: the page is complete without it.
func (s *stream) tagged(head []json.RawMessage) (any, error) {
	var tag string
	if err := json.Unmarshal(head[0], &tag); err != nil {
		return nil, err
	}
	if tag != "P" || len(head) < 2 {
		return nil, nil
	}
	var id int
	if err := json.Unmarshal(head[1], &id); err != nil {
		return nil, nil
	}
	arr, ok := s.deferred[id]
	if !ok || len(arr) == 0 {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal(arr[0], &v); err != nil {
		return nil, nil
	}
	return v, nil
}

// first is the first non-space byte of a raw JSON value, which is enough to
// tell an object from an array from a scalar.
func first(raw json.RawMessage) byte {
	for _, b := range raw {
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return b
		}
	}
	return 0
}
