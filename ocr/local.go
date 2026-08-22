package ocr

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// This machine is a host too.
//
// The fleet reads pages through browser accounts, and those accounts have a
// handful of uploads a day between them. Theory of Sets has 418 pages, so at
// that rate the volume is a read measured in weeks, and the pipeline behind it
// sits idle the whole time: nothing is assembled, no tag is minted, no
// translation starts. The laptop has a model on it as well, in the Claude Code
// CLI, and it reads a page in about fifteen seconds.
//
// So local is a host name like any other. Everything above it is unchanged: the
// same queue, the same content addressed job ids, the same nine rules deciding
// what is accepted, the same page files. What differs is only the transport,
// and that is what the two types below are. A batch on a rented box is ssh and
// rsync; a batch here is sh and cp, against the same directories under the same
// home, so the protocol in batch.go does not know which one it is talking to.

// LocalHost is the host name that means this machine.
const LocalHost = "local"

// Local says whether a host is this machine rather than a rented box.
func (h Host) Local() bool { return h.Name == LocalHost }

// home is where the scratch directories live. Every path in the batch protocol
// is relative to the login home, because that is where an ssh command starts,
// and the local transport keeps that rather than inventing a root of its own.
func home() (string, error) { return os.UserHomeDir() }

func resolve(name string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}
	dir, err := home()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// LocalShell runs a command here when the host is local, and hands every other
// host to the shell it wraps.
//
// One type rather than a choice made at the call site, because Batch takes a
// single Shell for the run and the host name is the only thing that says which
// machine a command is for. A run that reads some pages here and some on
// server2 therefore needs no branch anywhere above this line.
type LocalShell struct {
	// Remote is where anything that is not local goes. Nil is allowed, and a
	// run that is local only never reaches it.
	Remote Shell
}

func (s LocalShell) Run(ctx context.Context, host, command string) (string, error) {
	if host != LocalHost {
		if s.Remote == nil {
			return "", fmt.Errorf("no transport for host %s", host)
		}
		return s.Remote.Run(ctx, host, command)
	}
	dir, err := home()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("sh -c %q: %w: %s", condense(command), err, condense(string(output)))
	}
	return string(output), nil
}

// LocalCopy moves files into and out of the scratch directories on this
// machine, and hands every other host to the copier it wraps.
type LocalCopy struct {
	Remote Copier
}

func (c LocalCopy) Push(ctx context.Context, host string, local []string, remote string) error {
	if host != LocalHost {
		if c.Remote == nil {
			return fmt.Errorf("no transport for host %s", host)
		}
		return c.Remote.Push(ctx, host, local, remote)
	}
	dir, err := resolve(remote)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range local {
		if err := copyFile(name, filepath.Join(dir, filepath.Base(name))); err != nil {
			return err
		}
	}
	return nil
}

func (c LocalCopy) Pull(ctx context.Context, host, remote, local string) error {
	if host != LocalHost {
		if c.Remote == nil {
			return fmt.Errorf("no transport for host %s", host)
		}
		return c.Remote.Pull(ctx, host, remote, local)
	}
	// A trailing slash means the contents of the directory, which is rsync's
	// convention and what the batch asks for. Here the contents are all there
	// is to copy either way, so it is trimmed and the answers are copied.
	dir, err := resolve(strings.TrimSuffix(remote, "/"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(local, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(dir, entry.Name()), filepath.Join(local, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyFile writes through a temporary name and renames.
//
// The poller counts the Markdown files in the answers directory and calls the
// batch finished when the count reaches the page count, so a file that exists
// and is half written is a page the run will take as an answer. Renaming into
// place is what makes the count mean what it says.
func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(to), ".copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), to)
}
