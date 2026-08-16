package ocr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/api"
)

// Asking a question over HTTP rather than over ssh.
//
// Ask, above, is the transport the fleet needs: a scratch directory on a rented
// box, a prompt pushed as a file because it is full of dollars and backslashes
// that a remote shell would eat, a detached process polled to completion. All
// of that exists because a browser session is driven by a program running on
// the box and there is no other way to reach it.
//
// A gateway needs none of it. It is an HTTP endpoint that takes the question in
// the body and answers in the response, which this repo already speaks, so the
// whole of the machinery above collapses to one call.
//
// The reason to have both is uploads. Reading a page image spends an upload,
// and the accounts behind the fleet run out of them for hours at a time; both
// boxes reading Theory of Sets ran out on the same evening. Nothing downstream
// of the reading needs a browser at all. A translation, a solved exercise and
// the Vietnamese of a term are text going out and text coming back, and every
// one of them sent to a free gateway is an upload left for a page.

// Asker is one question put to a model, however it is reached. Ask and Gateway
// both satisfy it, and a caller that only wants an answer should hold this.
type Asker interface {
	Do(ctx context.Context) (Answer, error)
}

// Gateway is one question put to an OpenAI-compatible endpoint.
type Gateway struct {
	// Name is the route this went to, for the log line and for a report that
	// has to say which of several hosts answered.
	Name   string
	Client api.Completer
	Model  string

	// Prompt is the whole question, exactly as the ssh path would have written
	// it to a file. There is no system message: every prompt in this repo is
	// written as one piece and splitting it here would make the question the
	// gateway sees differ from the question the fleet sees, which is the one
	// thing that must not vary when the same work runs on both.
	Prompt string
	// ID names the question in the log, as it names the scratch directory on a
	// box, so the two transports read the same way.
	ID   string
	Logf func(string, ...any)
}

func (g Gateway) logf(format string, args ...any) {
	if g.Logf != nil {
		g.Logf(format, args...)
	}
}

// Do asks, and returns what came back.
func (g Gateway) Do(ctx context.Context) (Answer, error) {
	if g.Client == nil {
		return Answer{}, fmt.Errorf("question %s: no client for gateway %s", g.ID, g.Name)
	}
	if strings.TrimSpace(g.Prompt) == "" {
		return Answer{}, fmt.Errorf("question %s: nothing to ask", g.ID)
	}
	started := time.Now()
	g.logf("%s: asking %s", g.Name, g.ID)
	response, err := g.Client.Complete(ctx, api.Request{Model: g.Model, Input: g.Prompt})
	if err != nil {
		return Answer{}, fmt.Errorf("question %s on %s: %w", g.ID, g.Name, err)
	}
	text := strings.TrimSpace(response.Text)
	if text == "" {
		// An empty answer is not an answer, and the caller downstream would
		// read it as a model that had nothing to say about the corpus rather
		// than as a call that failed.
		return Answer{}, fmt.Errorf("question %s on %s: the gateway answered with nothing", g.ID, g.Name)
	}
	// The same check the ssh path makes, for the same reason: a service that
	// answers its own failures in the place it answers questions hands back a
	// judge with no verdict in it, and the caller files the work as bad for a
	// reason that has nothing to do with the work.
	if why := ProviderFailure(text); why != "" {
		return Answer{}, fmt.Errorf("question %s on %s: the service answered with its own error page: %s",
			g.ID, g.Name, why)
	}
	model := response.Model
	if model == "" {
		model = g.Model
	}
	out := Answer{Text: text, Model: model, Elapsed: time.Since(started)}
	g.logf("%s: %s answered in %s", g.Name, g.ID, out.Elapsed.Round(time.Second))
	return out, nil
}

// NewAsk builds the right transport for a host: HTTP when the host is a
// gateway, and the fleet's ssh and rsync when it is a box.
//
// It exists so the four commands that ask a question do not each decide this.
// They all want the same thing, which is an answer from whatever will answer,
// and the difference between a rented box and a public endpoint is not theirs
// to carry.
func NewAsk(host Host, shell Shell, copier Copier, prompt, id string, keep bool) Asker {
	return NewAskWithin(host, shell, copier, prompt, id, keep, 0)
}

// NewAskWithin is NewAsk with a bound on how long the question may take.
//
// A box question with no bound gets the page default, which is fifteen minutes
// times three because a § is minutes of generation. A chunk of six thousand
// characters is not that, it has measured between forty and seventy seconds,
// and the difference is not academic: the caller holds a queue lease while it
// waits, and a box that has stopped answering held three lanes of server3 for
// nine minutes on a three minute lease, so the chunks expired underneath the
// lanes still working on them and were free to be handed out twice.
//
// A deadline of zero is no bound, which is what the page path wants. A gateway
// takes its bound from the HTTP client its route built and this is not passed
// down to it, since a route already names the longest it will wait.
func NewAskWithin(host Host, shell Shell, copier Copier, prompt, id string, keep bool, deadline time.Duration) Asker {
	if host.Client != nil {
		return Gateway{Name: host.Name, Client: host.Client, Model: host.Model, Prompt: prompt, ID: id}
	}
	return Ask{Host: host, Shell: shell, Copy: copier, Prompt: prompt, ID: id, Keep: keep, Deadline: deadline}
}
