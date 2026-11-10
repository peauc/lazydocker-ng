package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// we only need these two methods from our OSCommand struct, for killing commands
type CmdKiller interface {
	Kill(cmd *exec.Cmd) error
	PrepareForChildren(cmd *exec.Cmd)
}

type SSHHandler struct {
	oSCommand CmdKiller

	dialContext func(ctx context.Context, network, addr string) (io.Closer, error)
	startCmd    func(*exec.Cmd) error
	tempDir     func(dir string, pattern string) (name string, err error)

	// socketTimeout is how long to wait for the tunneled socket to accept
	// connections before giving up.
	socketTimeout time.Duration
}

func NewSSHHandler(oSCommand CmdKiller) *SSHHandler {
	return &SSHHandler{
		oSCommand: oSCommand,

		dialContext: func(ctx context.Context, network, addr string) (io.Closer, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		startCmd:      func(cmd *exec.Cmd) error { return cmd.Start() },
		tempDir:       os.MkdirTemp,
		socketTimeout: 8 * time.Second,
	}
}

type TunnelResult struct {
	Closer     io.Closer
	SocketPath string
	Created    bool
}

// HandleSSHDockerHost overrides the DOCKER_HOST environment variable
// to point towards a local unix socket tunneled over SSH to the specified ssh host.
func (h *SSHHandler) HandleSSHDockerHost(dockerHost string) (TunnelResult, error) {
	u, err := url.Parse(dockerHost)
	if err != nil {
		// if no or an invalid docker host is specified, continue nominally
		return TunnelResult{Closer: noopCloser{}}, nil
	}

	// if the docker host scheme is "ssh", forward the docker socket before creating the client
	if u.Scheme == "ssh" {
		ctx := context.Background()
		tunnel, err := h.createDockerHostTunnel(ctx, u.String())
		if err != nil {
			return TunnelResult{Closer: noopCloser{}}, fmt.Errorf("tunnel ssh docker host: %w", err)
		}

		return TunnelResult{
			Closer:     tunnel,
			SocketPath: tunnel.socketPath,
			Created:    true,
		}, nil
	}
	return TunnelResult{Closer: noopCloser{}}, nil
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// processExitTimeout bounds how long Close waits for a killed ssh to exit.
const processExitTimeout = 5 * time.Second

// process is a started command whose exit is observed by one goroutine, so
// several callers can wait for it.
type process struct {
	cmd     *exec.Cmd
	done    chan struct{} // closed once cmd.Wait has returned
	waitErr error         // cmd.Wait's result; read only after done is closed
}

func watchProcess(cmd *exec.Cmd) *process {
	p := &process{cmd: cmd, done: make(chan struct{})}
	go func() {
		p.waitErr = cmd.Wait()
		close(p.done)
	}()
	return p
}

func (p *process) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

type tunneledDockerHost struct {
	socketPath string
	socketDir  string
	proc       *process
	oSCommand  CmdKiller

	closeOnce sync.Once
	closeErr  error
}

var _ io.Closer = (*tunneledDockerHost)(nil)

// Close stops ssh, waits for it to exit and removes the socket's temp dir. It
// is safe to call more than once.
func (t *tunneledDockerHost) Close() error {
	t.closeOnce.Do(func() {
		t.closeErr = stopTunnel(t.oSCommand, t.proc, t.socketDir)
	})
	return t.closeErr
}

// stopTunnel kills ssh unless it already exited, waits for it to be reaped and
// removes socketDir.
func stopTunnel(killer CmdKiller, proc *process, socketDir string) error {
	var errs []error
	if proc != nil {
		if !proc.exited() {
			if err := killer.Kill(proc.cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
				errs = append(errs, fmt.Errorf("kill ssh: %w", err))
			}
		}
		select {
		case <-proc.done:
		case <-time.After(processExitTimeout):
			errs = append(errs, errors.New("ssh did not exit after being killed"))
		}
	}
	if err := os.RemoveAll(socketDir); err != nil {
		errs = append(errs, fmt.Errorf("remove ssh tunnel dir: %w", err))
	}
	return errors.Join(errs...)
}

func (h *SSHHandler) createDockerHostTunnel(ctx context.Context, remoteHost string) (*tunneledDockerHost, error) {
	socketDir, err := h.tempDir("", "ld-ssh-")
	if err != nil {
		return nil, fmt.Errorf("create ssh tunnel tmp dir: %w", err)
	}
	localSocket := filepath.Join(socketDir, "docker.sock")

	cmd, err := h.tunnelSSH(ctx, remoteHost, localSocket)
	if err != nil {
		_ = os.RemoveAll(socketDir)
		return nil, fmt.Errorf("tunnel docker host over ssh: %w", err)
	}
	proc := watchProcess(cmd)

	// wait for the socket to dial successfully before attempting to create a
	// new docker client
	dialCtx, cancel := context.WithTimeout(ctx, h.socketTimeout)
	defer cancel()

	if err := h.retrySocketDial(dialCtx, localSocket); err != nil {
		_ = stopTunnel(h.oSCommand, proc, socketDir)
		return nil, fmt.Errorf("ssh tunneled socket %s never became available within %s: %w", localSocket, h.socketTimeout, err)
	}

	// construct the new DOCKER_HOST url with the proper scheme
	newDockerHostURL := url.URL{Scheme: "unix", Path: localSocket}
	return &tunneledDockerHost{
		socketPath: newDockerHostURL.String(),
		socketDir:  socketDir,
		proc:       proc,
		oSCommand:  h.oSCommand,
	}, nil
}

// Attempt to dial the socket until it becomes available.
// Uses exponential backoff starting at 100ms, doubling up to 5s.
// The retry loop continues until ctx is done; the error then includes the
// last dial error so users can see why.
func (h *SSHHandler) retrySocketDial(ctx context.Context, socketPath string) error {
	const (
		initialBackoff = 100 * time.Millisecond
		maxBackoff     = 5 * time.Second
	)
	backoff := initialBackoff
	var lastErr error

	for {
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return fmt.Errorf("%w (last dial error: %w)", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-timer.C:
		}

		if lastErr = h.tryDial(ctx, socketPath); lastErr == nil {
			return nil
		}

		backoff = min(backoff*2, maxBackoff)
	}
}

// Try to dial the specified unix socket, immediately close the connection if successfully created.
func (h *SSHHandler) tryDial(ctx context.Context, socketPath string) error {
	conn, err := h.dialContext(ctx, "unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func (h *SSHHandler) tunnelSSH(ctx context.Context, host, localSocket string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "ssh", "-L", localSocket+":/var/run/docker.sock", host, "-N")
	h.oSCommand.PrepareForChildren(cmd)
	err := h.startCmd(cmd)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}
