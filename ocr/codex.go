package ocr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// The subscription on this machine, reached as a command rather than as a
// server.
//
// The fleet is browser sessions on rented boxes and the gateways are free
// models over HTTP, and between them they are what the text stages have run on.
// Both have the same shape of limit: the boxes run out of uploads and turns,
// the gateways answer 429 with fourteen hours on it, and a run spends most of
// its wall clock waiting for one or the other to come back. Neither is a
// question of money, and neither is fast.
//
// The ChatGPT subscription is already paid for and codex speaks to it from
// here, with no browser, no box and no key in an environment variable. So it is
// a third transport: the same prompt, the same rules deciding what is accepted,
// the same queue, reached by running a program on this machine and reading what
// it prints.
//
// Measured against the chunks the free gateway had already answered, it is both
// quicker and better. See the note on CodexModels for the numbers and for why
// the cheap model is tried first and is not trusted alone.

// CodexBin is the command. On PATH rather than an absolute path, because the CLI
// updates itself and a pinned path goes stale.
const CodexBin = "codex"

// codexTimeout is what one question is given before it is abandoned.
//
// Well over what a chunk takes. The full model has measured about forty seconds
// on a chunk of six thousand characters and the cheap one about seventeen, and
// the number here is for the case where the CLI is waiting on a login or on a
// network that is not there, which is a thing to fail out of rather than to
// hang on.
const codexTimeout = 5 * time.Minute

// Codex is one question put to the subscription through the codex CLI.
type Codex struct {
	// Name is the route this went to, for the log line.
	Name  string
	Model string
	// Prompt is the whole question, exactly as the other two transports send
	// it. Nothing is added to it here: a prompt that differs between transports
	// is an answer that cannot be compared with another transport's.
	Prompt string
	// ID names the question in the log, as it names the scratch directory on a
	// box.
	ID   string
	Logf func(string, ...any)

	// Deadline bounds the one call. Zero is codexTimeout.
	Deadline time.Duration
	// Exec runs the command and gives back what it wrote to standard output. It
	// is a field so a test can run this without the CLI, a subscription or a
	// network.
	Exec func(ctx context.Context, name string, args []string, stdin string) (string, error)
}

func (c Codex) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

func (c Codex) deadline() time.Duration {
	if c.Deadline > 0 {
		return c.Deadline
	}
	return codexTimeout
}

func (c Codex) run(ctx context.Context, name string, args []string, stdin string) (string, error) {
	if c.Exec != nil {
		return c.Exec(ctx, name, args, stdin)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	// The CLI reports a refusal, a rate limit and a model it does not serve on
	// standard output as a line of the stream, and standard error carries the
	// progress it prints for a person. So the output is worth reading even when
	// the exit status is not zero, and the parse below says what went wrong far
	// better than "exit status 1" does.
	if err != nil && len(out) == 0 {
		return "", err
	}
	return string(out), nil
}

// Do asks, and gives back the answer with what answered.
func (c Codex) Do(ctx context.Context) (Answer, error) {
	if strings.TrimSpace(c.Prompt) == "" {
		return Answer{}, fmt.Errorf("question %s on %s: there is no question to ask", c.ID, c.Name)
	}
	if strings.TrimSpace(c.Model) == "" {
		return Answer{}, fmt.Errorf("question %s on %s: no model was named", c.ID, c.Name)
	}
	ctx, cancel := context.WithTimeout(ctx, c.deadline())
	defer cancel()

	// exec, so nothing interactive is started. read-only, so a question about a
	// book cannot write to the disk of the machine that asked it. The git check
	// is skipped because the question has nothing to do with the directory it
	// is asked in.
	args := []string{"exec", "-m", c.Model, "--json", "--skip-git-repo-check", "-s", "read-only", "-"}
	started := time.Now()
	out, err := c.run(ctx, CodexBin, args, c.Prompt)
	if err != nil {
		return Answer{}, fmt.Errorf("question %s on %s: %w", c.ID, c.Name, err)
	}
	text, model, usage, why := readCodexStream(out)
	if why != "" {
		return Answer{}, fmt.Errorf("question %s on %s: %s", c.ID, c.Name, why)
	}
	if strings.TrimSpace(text) == "" {
		return Answer{}, fmt.Errorf("question %s on %s: answered with nothing", c.ID, c.Name)
	}
	if why := ProviderFailure(text); why != "" {
		return Answer{}, fmt.Errorf("question %s on %s: the service answered with its own error page: %s",
			c.ID, c.Name, why)
	}
	if model == "" {
		model = c.Model
	}
	elapsed := time.Since(started)
	c.logf("%s: %s answered in %s, %s", c.Name, c.ID, elapsed.Round(time.Second), usage)
	return Answer{Text: strings.TrimSpace(text), Model: model, Elapsed: elapsed}, nil
}

// readCodexStream picks the answer out of what the CLI printed.
//
// The CLI writes one JSON object a line: the thread starting, the turn
// starting, each item it completed, and the turn completing with what the turn
// cost. The answer is the last completed item of type agent_message, the last
// rather than the first because a turn that thinks aloud completes more than
// one and the last is the one addressed to whoever asked.
//
// A line that does not parse is passed over rather than being an error. The CLI
// prints its own notices on this stream from time to time, and a notice is not
// a reason to throw away an answer that is sitting three lines below it.
//
// The first reason is kept and not the last. A turn that fails prints what the
// endpoint said and then prints that the turn failed, and the first of those is
// the sentence somebody can act on: the model is not one this account serves,
// or the account is out of turns until Tuesday. The second is "nope".
func readCodexStream(out string) (text, model, usage, why string) {
	scanner := bufio.NewScanner(strings.NewReader(out))
	// A chunk of Bourbaki and its translation go on one line of this stream,
	// and the default is sixty four kilobytes.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var event struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Item    struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Message string `json:"message"`
			} `json:"item"`
			Usage struct {
				Input  int `json:"input_tokens"`
				Cached int `json:"cached_input_tokens"`
				Output int `json:"output_tokens"`
			} `json:"usage"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		switch {
		case event.Type == "item.completed" && event.Item.Type == "agent_message":
			text = event.Item.Text
		case event.Type == "turn.completed":
			usage = fmt.Sprintf("%d tokens in, %d cached, %d out",
				event.Usage.Input, event.Usage.Cached, event.Usage.Output)
		case event.Type == "error" && why == "":
			why = codexReason(event.Message)
		case event.Type == "turn.failed" && why == "":
			why = codexReason(event.Error.Message)
		}
	}
	// A turn that failed and then answered is not a failure. The CLI retries a
	// call it can retry, so an error line early in the stream can be followed
	// by a perfectly good answer, and only an error with nothing after it is
	// worth reporting.
	if text != "" {
		why = ""
	}
	return text, model, usage, why
}

// codexReason pulls the sentence out of the CLI's error, which is a JSON object
// printed inside the message field of another JSON object.
//
// The plain message is kept when it does not parse, since a sentence is what
// the caller is going to log either way and half a sentence is better than a
// blob of punctuation.
func codexReason(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "the run failed and said nothing"
	}
	var inner struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(message), &inner) == nil && inner.Error.Message != "" {
		return inner.Error.Message
	}
	return condense(message)
}
