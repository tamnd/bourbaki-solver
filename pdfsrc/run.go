// Package pdfsrc wraps the poppler command line tools.
//
// Everything the pipeline learns about a PDF comes through here: page counts,
// the native text layer, the embedded images, the fonts, and the page renders
// that feed vision OCR. Wrapping them behind one interface keeps the exec calls
// in a single place and lets the rest of the code be tested without poppler.
package pdfsrc

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs an external command and returns its standard output.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs commands for real. Dir, when set, is the working directory.
type ExecRunner struct {
	Dir string
}

// Run executes the command and returns stdout. A non-zero exit turns into an
// error carrying the first line of stderr, because poppler puts the useful part
// of a failure there and the rest is noise.
func (r ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.IndexByte(msg, '\n'); i > 0 {
			msg = msg[:i]
		}
		if msg == "" {
			return stdout.Bytes(), fmt.Errorf("%s: %w", name, err)
		}
		return stdout.Bytes(), fmt.Errorf("%s: %w: %s", name, err, msg)
	}
	return stdout.Bytes(), nil
}

// FakeRunner replays canned output, keyed by the whole command line joined with
// spaces. Tests register the exact invocations they expect.
type FakeRunner struct {
	Out  map[string]string
	Err  map[string]error
	Seen []string
}

// Run looks the command line up in Out. An unregistered command is an error, so
// a test that changes the flags of a poppler call finds out immediately.
func (f *FakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	f.Seen = append(f.Seen, key)
	if err, ok := f.Err[key]; ok {
		return nil, err
	}
	out, ok := f.Out[key]
	if !ok {
		return nil, fmt.Errorf("FakeRunner: no canned output for %q", key)
	}
	return []byte(out), nil
}
