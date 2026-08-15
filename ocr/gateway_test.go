package ocr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/api"
)

// stub is a gateway that answers whatever it was built with, and remembers what
// it was asked so a test can say the question went out whole.
type stub struct {
	answer api.Response
	err    error
	saw    api.Request
}

func (s *stub) Complete(_ context.Context, request api.Request) (api.Response, error) {
	s.saw = request
	if s.err != nil {
		return api.Response{}, s.err
	}
	return s.answer, nil
}

func TestGatewayAsks(t *testing.T) {
	client := &stub{answer: api.Response{Model: "nemotron-3-ultra-free", Text: "  la theorie des ensembles  "}}
	got, err := Gateway{Name: "zen", Client: client, Model: "nemotron-3-ultra-free",
		Prompt: "Translate: set theory", ID: "term-0001"}.Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Text != "la theorie des ensembles" {
		t.Errorf("Text = %q, want it trimmed", got.Text)
	}
	if got.Model != "nemotron-3-ultra-free" {
		t.Errorf("Model = %q, want what answered", got.Model)
	}
	// The prompt must arrive as one piece. The ssh path writes it to a file and
	// hands the file over, so anything split into a system message here would
	// make the same work read differently on the two transports.
	if client.saw.Input != "Translate: set theory" || client.saw.Instructions != "" {
		t.Errorf("the gateway was asked %+v", client.saw)
	}
}

// A model the route file names and the endpoint quietly swaps is a thing the
// caller has to be able to report, so the answer carries what answered. When the
// endpoint says nothing, the route's model is the best account there is.
func TestGatewayNamesTheModelWhenTheAnswerDoesNot(t *testing.T) {
	client := &stub{answer: api.Response{Text: "ok"}}
	got, err := Gateway{Name: "zen", Client: client, Model: "hy3-free", Prompt: "x", ID: "q"}.Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Model != "hy3-free" {
		t.Errorf("Model = %q, want the route's model", got.Model)
	}
}

// An empty answer and an error page are both failures, and neither may reach the
// caller as text. The first would be filed as a translation of nothing, and the
// second as a translation into the words of a rate limit notice.
func TestGatewayRefusesAnAnswerThatIsNotOne(t *testing.T) {
	for _, c := range []struct {
		name string
		text string
		want string
	}{
		{"nothing at all", "   ", "answered with nothing"},
		{"the endpoint's own error page", "Something went wrong while generating the response.", "its own error page"},
	} {
		_, err := Gateway{Name: "zen", Client: &stub{answer: api.Response{Text: c.text}},
			Model: "hy3-free", Prompt: "x", ID: "q"}.Do(context.Background())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

func TestGatewayReportsWhatFailed(t *testing.T) {
	_, err := Gateway{Name: "zen", Client: &stub{err: errors.New("429 too many requests")},
		Model: "hy3-free", Prompt: "x", ID: "term-0007"}.Do(context.Background())
	if err == nil {
		t.Fatal("a failed call came back as an answer")
	}
	// The question and the route both have to be in the line. A run puts out
	// thousands of these and the log is the only place to see which one broke.
	for _, want := range []string{"term-0007", "zen", "429"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to say %q", err, want)
		}
	}
	if _, err := (Gateway{Name: "zen", Model: "hy3-free", Prompt: "x", ID: "q"}).Do(context.Background()); err == nil {
		t.Error("a gateway with no client answered")
	}
	if _, err := (Gateway{Name: "zen", Client: &stub{}, Model: "hy3-free", ID: "q"}).Do(context.Background()); err == nil {
		t.Error("a gateway with no question answered")
	}
}

// The whole point of NewAsk is that the four commands stop deciding this. A host
// carrying a client is HTTP, and one carrying an ssh name is the fleet.
func TestNewAskPicksTheTransportFromTheHost(t *testing.T) {
	gateway := NewAsk(Host{Name: "zen", Client: &stub{}, Model: "hy3-free"},
		nil, nil, "question", "q1", false)
	if _, ok := gateway.(Gateway); !ok {
		t.Errorf("a host with a client got %T", gateway)
	}
	box := NewAsk(Host{Name: "server3", Tool: "/usr/local/bin/chatgpt-tool", Lanes: 4},
		nil, nil, "question", "q1", false)
	if _, ok := box.(Ask); !ok {
		t.Errorf("a host with no client got %T", box)
	}
}
