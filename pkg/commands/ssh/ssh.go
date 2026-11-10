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
	"strings"
	"sync"
	"time"
)

// we only need these two methods from our OSCommand struct, for killing commands
type CmdKiller interface {
	Kill(cmd *exec.Cmd) error
	PrepareForChildren(cmd *exec.Cmd)
}

type Handler struct {
	oSCommand CmdKiller

	dialContext func(ctx context.Context, network, addr string) (io.Closer, error)
	startCmd    func(*exec.Cmd) error
	tempDir     func(dir string, pattern string) (name string, err error)

	// socketTimeout is how long to wait for the tunneled socket to accept
	// connections before giving up.
	socketTimeout time.Duration
}

func NewHandler(oSCommand CmdKiller) *Handler {
	return &Handler{
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
func (h *Handler) HandleSSHDockerHost(dockerHost string) (TunnelResult, error) {
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

// maxUnixSocketPath is the longest unix socket path every supported OS
// accepts (macOS: 104 bytes including the terminating NUL; Linux allows 107).
const maxUnixSocketPath = 103

var errSSHExited = errors.New("ssh exited before the tunnel was ready")

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

// exitedCleanly reports whether ssh exited 0. Wait reports exec.ErrWaitDelay
// when a backgrounded child keeps stderr open after a clean exit; that still
// counts as clean.
func (p *process) exitedCleanly() bool {
	if p.waitErr == nil {
		return true
	}
	return errors.Is(p.waitErr, exec.ErrWaitDelay) && p.cmd.ProcessState != nil && p.cmd.ProcessState.Success()
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

func (h *Handler) createDockerHostTunnel(ctx context.Context, remoteHost string) (*tunneledDockerHost, error) {
	socketDir, err := h.tempDir("", "ld-ssh-")
	if err != nil {
		return nil, fmt.Errorf("create ssh tunnel tmp dir: %w", err)
	}
	localSocket := filepath.Join(socketDir, "docker.sock")
	if len(localSocket) > maxUnixSocketPath {
		_ = os.RemoveAll(socketDir)
		return nil, fmt.Errorf("ssh tunnel socket path %q is %d bytes, over the %d-byte limit for unix sockets; set TMPDIR to a shorter directory", localSocket, len(localSocket), maxUnixSocketPath)
	}

	stderr := &tailBuffer{max: 4096}
	cmd, err := h.tunnelSSH(ctx, remoteHost, localSocket, stderr)
	if err != nil {
		_ = os.RemoveAll(socketDir)
		return nil, fmt.Errorf("tunnel docker host over ssh: %w", err)
	}
	proc := watchProcess(cmd)

	// wait for the socket to dial successfully before attempting to create a
	// new docker client
	dialCtx, cancel := context.WithTimeout(ctx, h.socketTimeout)
	defer cancel()

	if err := h.retrySocketDial(dialCtx, localSocket, proc); err != nil {
		_ = stopTunnel(h.oSCommand, proc, socketDir)
		if errors.Is(err, errSSHExited) {
			return nil, fmt.Errorf("%w; ssh said: %s; lazydocker-ng runs ssh with BatchMode=yes, so `ssh %s` must work without a password or host-key prompt (use ssh-agent, and connect once to accept the host key)",
				err, strings.TrimSpace(stderr.String()), redactedHost(remoteHost))
		}
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

// redactedHost hides a password in an ssh:// URL before it is shown to users.
func redactedHost(host string) string {
	if u, err := url.Parse(host); err == nil {
		return u.Redacted()
	}
	return host
}

// Attempt to dial the socket until it becomes available.
// Uses exponential backoff starting at 100ms, doubling up to 5s.
// The retry loop continues until ctx is done (the error then includes the
// last dial error so users can see why) or until ssh fails. ssh exiting 0
// means it went to the background (e.g. ForkAfterAuthentication in the user's
// ssh config), so dialing goes on.
func (h *Handler) retrySocketDial(ctx context.Context, socketPath string, proc *process) error {
	const (
		initialBackoff = 100 * time.Millisecond
		maxBackoff     = 5 * time.Second
	)
	backoff := initialBackoff
	var lastErr error
	exited := proc.done

	for {
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return fmt.Errorf("%w (last dial error: %w)", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-exited:
			timer.Stop()
			if !proc.exitedCleanly() {
				return fmt.Errorf("%w: %w", errSSHExited, proc.waitErr)
			}
			exited = nil
			continue
		case <-timer.C:
		}

		if lastErr = h.tryDial(ctx, socketPath); lastErr == nil {
			return nil
		}

		backoff = min(backoff*2, maxBackoff)
	}
}

// Try to dial the specified unix socket, immediately close the connection if successfully created.
func (h *Handler) tryDial(ctx context.Context, socketPath string) error {
	conn, err := h.dialContext(ctx, "unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

// tunnelSSH starts ssh forwarding localSocket to the remote docker socket.
// BatchMode makes a password or host-key prompt fail instead of hanging,
// ExitOnForwardFailure makes ssh exit when the forward can't be set up,
// ControlPath=none keeps the tunnel out of the user's connection sharing (a
// ControlPersist master would move it to the background, out of our reach),
// and "--" keeps the host from being read as an option. -N must come before
// "--": anything after the destination would be the remote command.
func (h *Handler) tunnelSSH(ctx context.Context, host, localSocket string, stderr io.Writer) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "ssh",
		"-N",
		"-o", "BatchMode=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ControlPath=none",
		"-L", localSocket+":/var/run/docker.sock",
		"--", host,
	)
	cmd.Stderr = stderr
	// a backgrounded child may keep stderr open; don't let Wait hang on it
	cmd.WaitDelay = time.Second
	h.oSCommand.PrepareForChildren(cmd)
	err := h.startCmd(cmd)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

// tailBuffer is an io.Writer that keeps only the last max bytes written, so a
// long-running ssh's stderr can be quoted in an error without growing forever.
type tailBuffer struct {
	mu  sync.Mutex
	b   []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if over := len(t.b) - t.max; over > 0 {
		t.b = append(t.b[:0:0], t.b[over:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.b)
}
