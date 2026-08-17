package ocr

import (
	"context"
	"strings"
	"testing"
	"time"
)

// stream is what the CLI prints for a turn that answered.
const stream = `{"type":"thread.started","thread_id":"01a00e1d"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"thinking about it"}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"  Một hợp thành.  "}}
{"type":"turn.completed","usage":{"input_tokens":13321,"cached_input_tokens":4992,"output_tokens":1752}}
`

func TestCodexAsksAndReadsTheAnswerOutOfTheStream(t *testing.T) {
	var sawName string
	var sawArgs []string
	var sawStdin string
	got, err := Codex{
		Name: "codex", Model: "gpt-5.4", Prompt: "Translate: set theory", ID: "tr-vi-abc123-001-1",
		Exec: func(_ context.Context, name string, args []string, stdin string) (string, error) {
			sawName, sawArgs, sawStdin = name, args, stdin
			return stream, nil
		},
	}.Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Text != "Một hợp thành." {
		t.Errorf("Text = %q, want the last message, trimmed", got.Text)
	}
	if got.Model != "gpt-5.4" {
		t.Errorf("Model = %q, want the route's model", got.Model)
	}
	if sawName != CodexBin {
		t.Errorf("ran %q, want %q", sawName, CodexBin)
	}
	// The prompt goes in whole and on standard input, as it goes to a file on
	// the ssh path and into the body on the HTTP one. A question that differs
	// between transports is an answer that cannot be compared with another
	// transport's.
	if sawStdin != "Translate: set theory" {
		t.Errorf("stdin = %q, want the whole question", sawStdin)
	}
	// exec so nothing interactive starts, read-only so a question about a book
	// cannot write to this machine, and the named model rather than whatever
	// the CLI's own config happens to say.
	line := strings.Join(sawArgs, " ")
	for _, want := range []string{"exec", "--json", "-s read-only", "-m gpt-5.4", "--skip-git-repo-check"} {
		if !strings.Contains(line, want) {
			t.Errorf("args = %v, want %q in them", sawArgs, want)
		}
	}
}

// The turn that answers is the last one. A model that thinks aloud completes
// more than one message and the last is the one addressed to whoever asked.
func TestCodexTakesTheLastMessage(t *testing.T) {
	two := `{"type":"item.completed","item":{"type":"agent_message","text":"first"}}
{"type":"item.completed","item":{"type":"agent_message","text":"second"}}
{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":2}}
`
	got, err := Codex{Name: "codex", Model: "gpt-5.4", Prompt: "x", ID: "q",
		Exec: func(context.Context, string, []string, string) (string, error) { return two, nil },
	}.Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Text != "second" {
		t.Errorf("Text = %q, want the last message", got.Text)
	}
}

func TestCodexReportsWhatTheCLISaidWentWrong(t *testing.T) {
	for _, c := range []struct {
		name   string
		stream string
		want   string
	}{
		{
			"a model the subscription does not serve",
			`{"type":"error","message":"{\"type\":\"error\",\"status\":400,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"The 'gpt-5.1-codex-mini' model is not supported when using Codex with a ChatGPT account.\"}}"}
{"type":"turn.failed","error":{"message":"nope"}}`,
			"is not supported when using Codex with a ChatGPT account",
		},
		{
			"a turn that failed and said why in plain words",
			`{"type":"turn.failed","error":{"message":"stream disconnected before completion"}}`,
			"stream disconnected",
		},
		{
			"a turn that printed nothing at all",
			``,
			"answered with nothing",
		},
	} {
		_, err := Codex{Name: "codex", Model: "gpt-5.4", Prompt: "x", ID: "tr-vi-abc123-001-1",
			Exec: func(context.Context, string, []string, string) (string, error) { return c.stream, nil },
		}.Do(context.Background())
		if err == nil {
			t.Errorf("%s: a failed run came back as an answer", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to say %q", c.name, err, c.want)
		}
		// The question and the route both belong in the line, as they do on the
		// other two transports. A run puts thousands of these and the log is
		// the only place to see which one broke.
		for _, want := range []string{"tr-vi-abc123-001-1", "codex"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: err = %v, want it to say %q", c.name, err, want)
			}
		}
	}
}

// The CLI retries a call it can retry, so an error line early in the stream can
// be followed by a perfectly good answer. Only an error with nothing after it is
// a failure, and reporting the other kind would throw away work that succeeded.
func TestCodexKeepsAnAnswerThatCameAfterAnError(t *testing.T) {
	late := `{"type":"error","message":"429 too many requests"}
{"type":"item.completed","item":{"type":"agent_message","text":"Một hợp thành."}}
{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":2}}
`
	got, err := Codex{Name: "codex", Model: "gpt-5.4", Prompt: "x", ID: "q",
		Exec: func(context.Context, string, []string, string) (string, error) { return late, nil },
	}.Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Text != "Một hợp thành." {
		t.Errorf("Text = %q, want the answer that came after the error", got.Text)
	}
}

// A chunk of Bourbaki and its translation both arrive on one line of this
// stream, and a scanner's default line is sixty four kilobytes.
func TestCodexReadsALineLongerThanAScannerDefault(t *testing.T) {
	long := strings.Repeat("Một hợp thành. ", 20000)
	one := `{"type":"item.completed","item":{"type":"agent_message","text":"` + long + `"}}`
	got, err := Codex{Name: "codex", Model: "gpt-5.4", Prompt: "x", ID: "q",
		Exec: func(context.Context, string, []string, string) (string, error) { return one, nil },
	}.Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got.Text != strings.TrimSpace(long) {
		t.Errorf("Text is %d bytes, want the whole %d", len(got.Text), len(strings.TrimSpace(long)))
	}
}

// The same choice NewAsk makes for the other two, made for this one.
func TestNewAskPicksTheCommandWhenTheHostNamesOne(t *testing.T) {
	asker := NewAskWithin(Host{Name: "codex", Command: "codex", Model: "gpt-5.4"},
		nil, nil, "question", "q1", false, 5*time.Minute)
	call, ok := asker.(Codex)
	if !ok {
		t.Fatalf("a host naming a command got %T", asker)
	}
	if call.Model != "gpt-5.4" || call.Deadline != 5*time.Minute {
		t.Errorf("built %+v, want the host's model and the caller's bound", call)
	}
}
