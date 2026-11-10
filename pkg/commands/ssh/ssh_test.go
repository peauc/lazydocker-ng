package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	case "sleep-short":
		time.Sleep(5 * time.Second)
	case "fail":
		fmt.Fprintln(os.Stderr, "Permission denied (publickey).")
		os.Exit(255)
	case "exit0":
		// like ssh with ForkAfterAuthentication/ControlPersist: the started
		// process exits 0 while the forward lives on elsewhere
		os.Exit(0)
	case "fork0":
		// as exit0, but a child keeps the inherited stderr open; its PID goes
		// to HELPER_PIDFILE so the test can stop it (a running test binary
		// cannot be deleted on Windows)
		child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestHelperProcess", "--", "sleep-short") //nolint:gosec // re-executes this test binary
		child.Env = os.Environ()
		child.Stderr = os.Stderr
		if child.Start() == nil {
			_ = os.WriteFile(os.Getenv("HELPER_PIDFILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600) //nolint:gosec // path set by the test itself
		}
		os.Exit(0)
	}
	os.Exit(0)
}

// fakeSSH records the ssh command it is asked to start and runs the helper
// process instead.
type fakeSSH struct {
	mode    string
	err     error
	args    []string
	cmd     *exec.Cmd
	starts  int
	pidFile string // fork0: where the helper writes its child's PID
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
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HELPER_PIDFILE="+f.pidFile)
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

func newTestHandler(t *testing.T, ssh *fakeSSH, dial func(context.Context, string, string) (io.Closer, error), timeout time.Duration) (*Handler, *tempDirs) {
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
	return &Handler{
		oSCommand:     processKiller{},
		dialContext:   dial,
		startCmd:      ssh.start,
		tempDir:       dirs.make,
		socketTimeout: timeout,
	}, dirs
}

// newForkingSSH returns a fake ssh in fork0 mode whose background child is
// killed when the test ends.
func newForkingSSH(t *testing.T) *fakeSSH {
	t.Helper()
	ssh := &fakeSSH{mode: "fork0", pidFile: filepath.Join(t.TempDir(), "child.pid")}
	t.Cleanup(func() {
		raw, err := os.ReadFile(ssh.pidFile)
		if err != nil {
			return
		}
		pid, err := strconv.Atoi(string(raw))
		if err != nil {
			return
		}
		if child, err := os.FindProcess(pid); err == nil {
			_ = child.Kill()
			_, _ = child.Wait()
		}
	})
	return ssh
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
		socket := url.URL{Scheme: "unix", Path: filepath.Join(dirs.created[0], "docker.sock")}
		assert.Equal(t, socket.String(), result.SocketPath)
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

func TestTunnelSSHArgs(t *testing.T) {
	ssh := &fakeSSH{mode: "sleep"}
	handler, dirs := newTestHandler(t, ssh, dialAfter(0), 5*time.Second)

	result, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = result.Closer.Close() })

	if assert.Len(t, dirs.created, 1) {
		assert.Equal(t, []string{
			"ssh", "-N",
			"-o", "BatchMode=yes",
			"-o", "ExitOnForwardFailure=yes",
			"-o", "ControlPath=none",
			"-L", filepath.Join(dirs.created[0], "docker.sock") + ":/var/run/docker.sock",
			"--", "ssh://myhost@192.168.5.178",
		}, ssh.args)
	}
}

func TestTunnelSSHExitsEarly(t *testing.T) {
	ssh := &fakeSSH{mode: "fail"}
	handler, dirs := newTestHandler(t, ssh, dialAfter(1_000_000), 5*time.Second)

	started := time.Now()
	_, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")

	assert.Less(t, time.Since(started), 2*time.Second, "should not wait for the socket timeout")
	assert.ErrorContains(t, err, "Permission denied (publickey).")
	assert.ErrorContains(t, err, "BatchMode=yes")
	if assert.Len(t, dirs.created, 1) {
		assertGone(t, dirs.created[0])
	}
}

func TestTunnelSocketPathTooLong(t *testing.T) {
	ssh := &fakeSSH{mode: "sleep"}
	handler, dirs := newTestHandler(t, ssh, dialAfter(0), time.Second)
	handler.tempDir = func(string, string) (string, error) {
		dir := filepath.Join(os.TempDir(), strings.Repeat("x", 120))
		dirs.created = append(dirs.created, dir)
		return dir, os.MkdirAll(dir, 0o700)
	}

	_, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")

	assert.ErrorContains(t, err, "103")
	assert.Equal(t, 0, ssh.starts)
	assertGone(t, dirs.created[0])
}

func TestTailBufferKeepsLastBytes(t *testing.T) {
	b := &tailBuffer{max: 8}

	_, _ = b.Write([]byte("01234"))
	_, _ = b.Write([]byte("56789"))

	assert.Equal(t, "23456789", b.String())
}

// ssh exiting 0 before the socket is up means it went to the background
// (ForkAfterAuthentication, or a ControlPersist master); the tunnel must keep
// waiting for the socket instead of failing.
func TestTunnelSSHExitsZeroKeepsDialing(t *testing.T) {
	ssh := &fakeSSH{mode: "exit0"}
	handler, dirs := newTestHandler(t, ssh, dialAfter(3), 5*time.Second)

	result, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")

	assert.NoError(t, err)
	assert.True(t, result.Created)
	assert.NoError(t, result.Closer.Close())
	if assert.Len(t, dirs.created, 1) {
		assertGone(t, dirs.created[0])
	}
}

// A backgrounded child may keep ssh's stderr open; Close must not wait for it.
func TestTunnelCloseDoesNotWaitForChildrenHoldingStderr(t *testing.T) {
	ssh := newForkingSSH(t)
	handler, _ := newTestHandler(t, ssh, dialAfter(1), 5*time.Second)

	result, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")
	assert.NoError(t, err)

	started := time.Now()
	assert.NoError(t, result.Closer.Close())
	assert.Less(t, time.Since(started), 3*time.Second)
}

// ssh with ForkAfterAuthentication exits 0 while its background child keeps
// stderr open, so Wait reports exec.ErrWaitDelay; that is still a clean exit
// and the tunnel must keep waiting for a socket that is slow to come up.
func TestTunnelSSHForksKeepsDialing(t *testing.T) {
	ssh := newForkingSSH(t)
	handler, _ := newTestHandler(t, ssh, dialAfter(4), 6*time.Second)

	result, err := handler.HandleSSHDockerHost("ssh://myhost@192.168.5.178")

	assert.NoError(t, err)
	if err == nil {
		assert.NoError(t, result.Closer.Close())
	}
}
