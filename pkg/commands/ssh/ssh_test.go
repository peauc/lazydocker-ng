package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestHelperProcess is not a real test: the tunnel tests start the test binary
// in place of ssh, in one of two modes.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	switch os.Args[len(os.Args)-1] {
	case "sleep":
		time.Sleep(60 * time.Second)
	case "fail":
		fmt.Fprintln(os.Stderr, "Permission denied (publickey).")
		os.Exit(255)
	}
	os.Exit(0)
}

// fakeSSH records the ssh command it is asked to start and runs the helper
// process instead.
type fakeSSH struct {
	mode   string
	err    error
	args   []string
	cmd    *exec.Cmd
	starts int
}

func (f *fakeSSH) start(cmd *exec.Cmd) error {
	f.starts++
	f.args = append([]string(nil), cmd.Args...)
	f.cmd = cmd
	if f.err != nil {
		return f.err
	}
	cmd.Path = os.Args[0]
	cmd.Args = []string{os.Args[0], "-test.run=TestHelperProcess", "--", f.mode}
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	cmd.Err = nil // ssh may not be on PATH in CI
	return cmd.Start()
}

type processKiller struct{}

func (processKiller) Kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func (processKiller) PrepareForChildren(*exec.Cmd) {}

// tempDirs creates real, short temp dirs (t.TempDir paths are too long for a
// unix socket on macOS) and remembers them.
type tempDirs struct {
	created []string
}

func (d *tempDirs) make(dir, pattern string) (string, error) {
	path, err := os.MkdirTemp(dir, pattern)
	if err == nil {
		d.created = append(d.created, path)
	}
	return path, err
}

// dialAfter returns a dialer that fails `failures` times, then succeeds.
func dialAfter(failures int) func(context.Context, string, string) (io.Closer, error) {
	calls := 0
	return func(context.Context, string, string) (io.Closer, error) {
		calls++
		if calls <= failures {
			return nil, errors.New("connection refused")
		}
		return noopCloser{}, nil
	}
}

func newTestHandler(t *testing.T, ssh *fakeSSH, dial func(context.Context, string, string) (io.Closer, error), timeout time.Duration) (*SSHHandler, *tempDirs) {
	t.Helper()
	dirs := &tempDirs{}
	t.Cleanup(func() {
		for _, dir := range dirs.created {
			_ = os.RemoveAll(dir)
		}
		if ssh.cmd != nil && ssh.cmd.Process != nil {
			_ = ssh.cmd.Process.Kill()
		}
	})
	return &SSHHandler{
		oSCommand:     processKiller{},
		dialContext:   dial,
		startCmd:      ssh.start,
		tempDir:       dirs.make,
		socketTimeout: timeout,
	}, dirs
}

func assertGone(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.ErrorIs(t, err, os.ErrNotExist, "%s should be removed", path)
}

func TestTunnelReadyThenClose(t *testing.T) {
	ssh := &fakeSSH{mode: "sleep"}
	handler, dirs := newTestHandler(t, ssh, dialAfter(1), 5*time.Second)

	result, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")

	assert.NoError(t, err)
	assert.True(t, result.Created)
	if assert.Len(t, dirs.created, 1) {
		assert.Equal(t, "unix://"+dirs.created[0]+"/docker.sock", result.SocketPath)
	}

	assert.NoError(t, result.Closer.Close())
	assert.NotNil(t, ssh.cmd.ProcessState, "ssh should be reaped")
	assertGone(t, dirs.created[0])
	assert.NoError(t, result.Closer.Close(), "Close is idempotent")
}

func TestTunnelSocketNeverAvailable(t *testing.T) {
	ssh := &fakeSSH{mode: "sleep"}
	handler, dirs := newTestHandler(t, ssh, dialAfter(1_000_000), 300*time.Millisecond)

	_, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, "connection refused")
	if assert.NotNil(t, ssh.cmd) {
		assert.NotNil(t, ssh.cmd.ProcessState, "ssh should be killed and reaped")
	}
	if assert.Len(t, dirs.created, 1) {
		assertGone(t, dirs.created[0])
	}
}

func TestTunnelStartFails(t *testing.T) {
	ssh := &fakeSSH{err: errors.New("no ssh")}
	handler, dirs := newTestHandler(t, ssh, dialAfter(0), time.Second)

	_, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")

	assert.ErrorContains(t, err, "no ssh")
	if assert.Len(t, dirs.created, 1) {
		assertGone(t, dirs.created[0])
	}
}

func TestHandleSSHDockerHostIgnoresNonSSH(t *testing.T) {
	for _, host := range []string{"", "https://myhost.com", "unix:///var/run/docker.sock"} {
		ssh := &fakeSSH{mode: "sleep"}
		handler, _ := newTestHandler(t, ssh, dialAfter(0), time.Second)

		result, err := handler.HandleSSHDockerHost(host)

		assert.NoError(t, err, host)
		assert.False(t, result.Created, host)
		assert.Equal(t, 0, ssh.starts, host)
	}
}
