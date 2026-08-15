package ocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// remote is a transport that records what it was asked for and never touches a
// network, which is how a test can tell local from not local.
type remote struct {
	shellHosts []string
	pushHosts  []string
	pullHosts  []string
	err        error
}

func (r *remote) Run(_ context.Context, host, _ string) (string, error) {
	r.shellHosts = append(r.shellHosts, host)
	return "ran on " + host, r.err
}

func (r *remote) Push(_ context.Context, host string, _ []string, _ string) error {
	r.pushHosts = append(r.pushHosts, host)
	return r.err
}

func (r *remote) Pull(_ context.Context, host, _, _ string) error {
	r.pullHosts = append(r.pullHosts, host)
	return r.err
}

func TestTheLocalShellRunsTheCommandHere(t *testing.T) {
	shell := LocalShell{Remote: &remote{}}
	out, err := shell.Run(context.Background(), LocalHost, "echo bourbaki")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "bourbaki" {
		t.Errorf("the command printed %q, want bourbaki", strings.TrimSpace(out))
	}
}

// A run that reads some pages here and some on a rented box hands both to the
// same Shell, and the host name is all there is to tell them apart.
func TestAnyOtherHostGoesToTheRemote(t *testing.T) {
	upstream := &remote{}
	shell := LocalShell{Remote: upstream}
	if _, err := shell.Run(context.Background(), "server2", "ls"); err != nil {
		t.Fatal(err)
	}
	if got := upstream.shellHosts; len(got) != 1 || got[0] != "server2" {
		t.Errorf("the remote was asked for %v, want one command for server2", got)
	}
}

// The command failing is not the transport failing, and the run above reads the
// output of a failed command: the poll parses what came back before it decides
// whether the batch is alive.
func TestAFailedCommandCarriesItsOutputBack(t *testing.T) {
	shell := LocalShell{}
	out, err := shell.Run(context.Background(), LocalHost, "echo said something; exit 3")
	if err == nil {
		t.Fatal("a command that exited 3 was reported as having worked")
	}
	if !strings.Contains(out, "said something") {
		t.Errorf("the output was %q, want what the command printed before it failed", out)
	}
}

func TestNoRemoteAndNotLocalIsAnErrorRatherThanAPanic(t *testing.T) {
	if _, err := (LocalShell{}).Run(context.Background(), "server2", "ls"); err == nil {
		t.Error("a run with no remote transport sent a command to server2")
	}
	if err := (LocalCopy{}).Push(context.Background(), "server2", nil, "dir"); err == nil {
		t.Error("a run with no remote transport pushed to server2")
	}
	if err := (LocalCopy{}).Pull(context.Background(), "server2", "dir", "here"); err == nil {
		t.Error("a run with no remote transport pulled from server2")
	}
}

// The batch protocol names its directories relative to the login home, because
// that is where an ssh command starts. Here that has to mean the same thing.
func TestTheScratchDirectoriesAreUnderTheHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)

	image := filepath.Join(t.TempDir(), "0022.png")
	if err := os.WriteFile(image, []byte("a page"), 0o644); err != nil {
		t.Fatal(err)
	}
	copier := LocalCopy{}
	if err := copier.Push(context.Background(), LocalHost, []string{image}, "bourbaki-ocr/in/batch-1"); err != nil {
		t.Fatal(err)
	}
	pushed := filepath.Join(root, "bourbaki-ocr", "in", "batch-1", "0022.png")
	if _, err := os.Stat(pushed); err != nil {
		t.Fatalf("the image is not at %s: %v", pushed, err)
	}

	answers := filepath.Join(root, "bourbaki-ocr", "out", "batch-1")
	if err := os.MkdirAll(answers, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(answers, "0022.md"), []byte("SIGNS AND ASSEMBLIES\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "answers")
	if err := copier.Pull(context.Background(), LocalHost, "bourbaki-ocr/out/batch-1/", dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "0022.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "SIGNS AND ASSEMBLIES\n" {
		t.Errorf("the answer came back as %q", got)
	}
}

// A batch whose host died before it wrote anything pulls an empty directory,
// and that is a batch with pages missing, not a transport error.
func TestPullingFromADirectoryThatIsNotThereIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dest := filepath.Join(t.TempDir(), "answers")
	if err := (LocalCopy{}).Pull(context.Background(), LocalHost, "bourbaki-ocr/out/gone/", dest); err != nil {
		t.Errorf("pulling nothing failed with %v", err)
	}
}

// Chrome needs a display and there is no Chrome here. The check for one asks
// pgrep for an Xvfb this machine has never run, so a local batch that asked
// would refuse to start every time.
func TestPrepareDoesNotAskThisMachineAboutADisplay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	shell := LocalShell{}
	host := Host{Name: LocalHost, Tool: "bourbaki", Lanes: 2}
	if err := prepare(context.Background(), shell, host, "", "bourbaki-ocr/in"); err != nil {
		t.Fatalf("prepare refused this machine: %v", err)
	}
}

func TestPrepareStillAsksARentedBox(t *testing.T) {
	upstream := &remote{err: errors.New("no")}
	err := prepare(context.Background(), upstream, Host{Name: "server2", Tool: "chatgpt-tool"}, "", "bourbaki-ocr/in")
	if err == nil {
		t.Fatal("prepare took a host that would not answer")
	}
	if len(upstream.shellHosts) != 1 {
		t.Errorf("the remote was asked %d times, want once", len(upstream.shellHosts))
	}
}

// setsid is not on macOS, and a launcher built with one starts nothing at all:
// the shell prints no pid and the run reads that as a batch that would not
// start.
func TestTheLocalLauncherHasNoSetsidAndNoDisplay(t *testing.T) {
	here := Batch{Host: Host{Name: LocalHost}}.launcher()
	if strings.Contains(here, "setsid") || strings.Contains(here, "DISPLAY") {
		t.Errorf("the local launcher is %q", here)
	}
	if !strings.Contains(here, "nohup") {
		t.Errorf("the local launcher is %q, want the batch to outlive the shell that started it", here)
	}
	away := Batch{Host: Host{Name: "server2"}}.launcher()
	if !strings.Contains(away, "setsid") || !strings.Contains(away, "DISPLAY") {
		t.Errorf("the remote launcher is %q, want a display and a process group", away)
	}
}

func TestLocalIsTheHostNamedLocal(t *testing.T) {
	if !(Host{Name: LocalHost}).Local() {
		t.Error("the host named local is not local")
	}
	if (Host{Name: "server3"}).Local() {
		t.Error("server3 is local")
	}
}
