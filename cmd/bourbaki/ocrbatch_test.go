package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/ocr"
)

// fakeCLI is a Claude Code that is a shell script.
//
// It reads the prompt on standard input the way the real one does, so a test
// can assert that the image path reached it, and it answers from the script
// body, so a test can make it refuse, hang or return nothing.
func fakeCLI(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nask=$(cat)\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// images is a directory of page pictures, which for this test need not be
// pictures: nothing here opens them, the path is handed to the CLI.
func images(t *testing.T, pages ...int) string {
	t.Helper()
	dir := t.TempDir()
	for _, page := range pages {
		name := filepath.Join(dir, fmt.Sprintf("%04d.png", page))
		if err := os.WriteFile(name, []byte("a page"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestEachPageIsWrittenUnderTheNameOfItsImage(t *testing.T) {
	in := images(t, 22, 23)
	out := t.TempDir()
	reader := localReader{
		Binary: fakeCLI(t, `echo "read $ask" | tail -c 40`), Model: "opus",
		Prompt: "transcribe the page", Timeout: 30 * time.Second,
	}
	if err := reader.all(context.Background(), []string{
		filepath.Join(in, "0022.png"), filepath.Join(in, "0023.png"),
	}, out, 2, 0, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"0022.md", "0023.md"} {
		body, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(body) == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

// The prompt is thousands of bytes of LaTeX and the image is named in it. Both
// go in on standard input, and a page read without its own image is the one
// failure that would not look like one.
func TestThePromptAndTheImagePathReachTheCLI(t *testing.T) {
	in := images(t, 22)
	out := t.TempDir()
	reader := localReader{
		Binary: fakeCLI(t, `printf '%s' "$ask"`), Model: "opus",
		Prompt: "write $\\mathscr{T}$ and not \\mathcal", Timeout: 30 * time.Second,
	}
	if err := reader.all(context.Background(), []string{filepath.Join(in, "0022.png")}, out, 1, 0, false); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(out, "0022.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `\mathscr{T}`) {
		t.Errorf("the prompt did not arrive whole:\n%s", body)
	}
	if !strings.Contains(string(body), filepath.Join(in, "0022.png")) {
		t.Errorf("the image was not named to the CLI:\n%s", body)
	}
}

// The account running out is not one page failing. Every page after it fails in
// a second, and each of those costs the page an attempt out of the four it has
// before the queue gives up on it for good.
func TestTheBatchStopsWhenTheModelIsOutOfTurns(t *testing.T) {
	in := images(t, 22, 23, 24, 25, 26, 27, 28, 29)
	out := t.TempDir()
	reader := localReader{
		Binary: fakeCLI(t, `echo "You've hit your session limit · resets 6:40pm" >&2; exit 1`),
		Model:  "opus", Prompt: "transcribe the page", Timeout: 30 * time.Second,
	}
	var all []string
	for page := 22; page <= 29; page++ {
		all = append(all, filepath.Join(in, fmt.Sprintf("%04d.png", page)))
	}
	err := reader.all(context.Background(), all, out, 1, 0, false)
	if err == nil {
		t.Fatal("the batch reported success after the model refused every page")
	}
	if !strings.Contains(err.Error(), "session limit") {
		t.Errorf("the batch failed with %v, want it to say what the model said", err)
	}
	written, readErr := os.ReadDir(out)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(written) != 0 {
		t.Errorf("%d answers were written by a model that refused", len(written))
	}
}

// One page failing is one page to read again. It must not take the rest of the
// batch with it.
func TestOnePageFailingLeavesTheRestOfTheBatchAlone(t *testing.T) {
	in := images(t, 22, 23)
	out := t.TempDir()
	reader := localReader{
		Binary: fakeCLI(t, `case "$ask" in *0022.png*) echo "the page is torn" >&2; exit 1 ;; *) echo "SIGNS AND ASSEMBLIES" ;; esac`),
		Model:  "opus", Prompt: "transcribe the page", Timeout: 30 * time.Second,
	}
	if err := reader.all(context.Background(), []string{
		filepath.Join(in, "0022.png"), filepath.Join(in, "0023.png"),
	}, out, 1, 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "0022.md")); err == nil {
		t.Error("the page that failed was written anyway")
	}
	if _, err := os.Stat(filepath.Join(out, "0023.md")); err != nil {
		t.Errorf("the page after the failure was not read: %v", err)
	}
}

// An empty answer is a page that was not read, and writing it would put an
// empty page file in the corpus and mark the job done.
func TestAnEmptyAnswerIsNotAPage(t *testing.T) {
	in := images(t, 22)
	out := t.TempDir()
	reader := localReader{
		Binary: fakeCLI(t, `true`), Model: "opus",
		Prompt: "transcribe the page", Timeout: 30 * time.Second,
	}
	if err := reader.all(context.Background(), []string{filepath.Join(in, "0022.png")}, out, 1, 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "0022.md")); err == nil {
		t.Error("an empty answer was written as a page")
	}
}

// -skip-existing is what makes a batch restartable, and the run leans on it: a
// batch that died two thirds through is started again over the same directory.
func TestAPageAlreadyAnsweredIsNotReadAgain(t *testing.T) {
	in := images(t, 22)
	out := t.TempDir()
	answer := filepath.Join(out, "0022.md")
	if err := os.WriteFile(answer, []byte("the reading it already has\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reader := localReader{
		Binary: fakeCLI(t, `echo "a second reading"`), Model: "opus",
		Prompt: "transcribe the page", Timeout: 30 * time.Second,
	}
	if err := reader.all(context.Background(), []string{filepath.Join(in, "0022.png")}, out, 1, 0, true); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(answer)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "second") {
		t.Errorf("the page was read again over its own answer:\n%s", body)
	}
}

// A page that hangs holds a lane, and a batch of six lanes has six of them to
// lose. The timeout is per page for that reason.
func TestAPageThatHangsIsGivenUpOn(t *testing.T) {
	in := images(t, 22)
	out := t.TempDir()
	reader := localReader{
		Binary: fakeCLI(t, `sleep 30`), Model: "opus",
		Prompt: "transcribe the page", Timeout: 200 * time.Millisecond,
	}
	started := time.Now()
	if err := reader.all(context.Background(), []string{filepath.Join(in, "0022.png")}, out, 1, 0, false); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Errorf("the batch waited %s on a page it had given up on", elapsed.Round(time.Second))
	}
	if _, err := os.Stat(filepath.Join(out, "0022.md")); err == nil {
		t.Error("a page that never answered was written")
	}
}

func TestTheImagesAreReadInPageOrder(t *testing.T) {
	dir := images(t, 24, 22, 23)
	got, err := pageImages(dir, "png", false)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, path := range got {
		names = append(names, filepath.Base(path))
	}
	want := []string{"0022.png", "0023.png", "0024.png"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Errorf("the pages are in the order %v, want %v", names, want)
	}
}

func TestOnlyTheImagesAreRead(t *testing.T) {
	dir := images(t, 22)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a page"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := pageImages(dir, "png", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("the batch took %v, want the one png", got)
	}
}

// Markdown asked for as an answer comes back in a fence often enough that a
// page file would start with one and the section built from it would carry it.
func TestAFenceAroundTheWholeAnswerComesOff(t *testing.T) {
	cases := map[string]string{
		"```markdown\nSIGNS AND ASSEMBLIES\n```":     "SIGNS AND ASSEMBLIES",
		"```\nSIGNS AND ASSEMBLIES\n```":             "SIGNS AND ASSEMBLIES",
		"SIGNS AND ASSEMBLIES":                       "SIGNS AND ASSEMBLIES",
		"a page with a ```fenced``` word in it":      "a page with a ```fenced``` word in it",
		"```markdown\nan answer with no end to it\n": "```markdown\nan answer with no end to it",
	}
	for in, want := range cases {
		if got := fence(strings.TrimSpace(in)); got != want {
			t.Errorf("fence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestARefusalIsToldApartFromAPageThatFailed(t *testing.T) {
	if !isLimit("You've hit your session limit · resets 6:40pm (Asia/Saigon)") {
		t.Error("the session limit was not recognised")
	}
	if !isLimit("Claude AI usage limit reached") {
		t.Error("the usage limit was not recognised")
	}
	if isLimit("the page is torn") {
		t.Error("a page that failed was taken for a refusal")
	}
}

// -hosts local is the whole switch between the two engines, and the run reads
// it three times: to pick the hosts, to decide whether a failed page is worth a
// question in its thread, and to say what model read the page.
func TestLocalIsAskedForByNameAndOnItsOwn(t *testing.T) {
	for _, names := range []string{"local", " local ", "local\n"} {
		if !hereOnly(names) {
			t.Errorf("-hosts %q did not select this machine", names)
		}
	}
	for _, names := range []string{"", "server2", "local,server2", "server2,local"} {
		if hereOnly(names) {
			t.Errorf("-hosts %q was taken for a local run", names)
		}
	}
}

// A page says in its front matter what read it, and that has to be true.
func TestAPageReadHereSaysSo(t *testing.T) {
	here, err := localHosts(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(here) != 1 || !here[0].Local() {
		t.Fatalf("the local engine is %v, want one host named local", here)
	}
	if here[0].Tool == "" {
		t.Error("the local host has no reader")
	}
	if here[0].Lanes != localLanes {
		t.Errorf("the local host reads %d pages at once, want %d", here[0].Lanes, localLanes)
	}
	if got := runModel(here); got != localModelName {
		t.Errorf("a page read here would record %q, want %q", got, localModelName)
	}
	if got := runModel([]ocr.Host{{Name: "server2"}}); got == localModelName {
		t.Error("a page read on the fleet would record the model of this machine")
	}
}

func TestTheLanesCanBeTurnedDown(t *testing.T) {
	here, err := localHosts(2)
	if err != nil {
		t.Fatal(err)
	}
	if here[0].Lanes != 2 {
		t.Errorf("-lanes 2 gave %d lanes", here[0].Lanes)
	}
}

// The run polls by counting the Markdown files in the answers directory, so a
// name that ends in .md must mean a page that is finished.
func TestAHalfWrittenPageIsNotCountedAsOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0022.md")
	if err := writeAnswer(path, "SIGNS AND ASSEMBLIES\n"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != "0022.md" {
			t.Errorf("%s is counted as a finished page", entry.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("writing one page left %d files behind", len(entries))
	}
}
