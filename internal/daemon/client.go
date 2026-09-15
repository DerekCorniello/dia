package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DerekCorniello/dia/internal/platform"
)

// ErrNoDaemon is returned when the socket exists but no daemon is
// listening on the far end, or the daemon is not running at all.
var ErrNoDaemon = errors.New("no daemon is running")

// ErrProtocolMismatch means a live daemon speaks an incompatible protocol.
var ErrProtocolMismatch = errors.New("daemon protocol mismatch")

// Client is a connection to a running daemon. It is safe for
// concurrent use; each Do carries its own request ID.
type Client struct {
	conn   net.Conn
	nextID int64
	br     *bufio.Reader
	mu     sync.Mutex
}

// Dial connects to the daemon socket for the given state dir. It does
// not spawn a daemon; use Ensure for that.
func Dial(stateDir string) (*Client, error) {
	return dialPath(socketPath(stateDir))
}

func dialPath(path string) (*Client, error) {
	conn, err := dialSocket(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoDaemon, err)
	}
	c := &Client{
		conn:   conn,
		nextID: 1,
		br:     bufio.NewReader(conn),
	}
	var hello HelloReply
	if err := c.Do(MethodHello, HelloParams{Protocol: ProtocolVersion}, &hello); err != nil {
		_ = conn.Close()
		if strings.Contains(err.Error(), "unknown method") || strings.Contains(err.Error(), "incompatible daemon protocol") {
			return nil, fmt.Errorf("%w: %w", ErrProtocolMismatch, err)
		}
		return nil, fmt.Errorf("daemon handshake: %w", err)
	}
	if hello.Protocol != ProtocolVersion {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: protocol %d; want %d", ErrProtocolMismatch, hello.Protocol, ProtocolVersion)
	}
	return c, nil
}

// Close detaches from the daemon. The daemon keeps running.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Do sends one request and waits for its synchronous response. Each
// request carries a unique ID; the daemon replies in order.
func (c *Client) Do(method string, params, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return ErrNoDaemon
	}
	_ = c.conn.SetDeadline(time.Now().Add(requestTimeout))
	defer func() { _ = c.conn.SetDeadline(time.Time{}) }()
	id := atomic.AddInt64(&c.nextID, 1)
	if err := writeRequest(c.conn, id, method, params); err != nil {
		return fmt.Errorf("send %s: %w", method, err)
	}

	line, err := c.readLine()
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if resp.ID != id {
		return fmt.Errorf("response id %d; want %d", resp.ID, id)
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	if result != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("decode result: %w", err)
		}
	}
	return nil
}

func (c *Client) readLine() ([]byte, error) {
	return c.br.ReadBytes('\n')
}

// EnsureOpts controls Ensure's dial-and-maybe-spawn behavior.
type EnsureOpts struct {
	StateDir string
	// BinaryPath is the executable used to spawn the daemon.
	// Empty means os.Executable().
	BinaryPath string
	// NoSpawn prevents Ensure from spawning a daemon; it dials
	// only and returns ErrNoDaemon when absent.
	NoSpawn bool
}

// Ensure dials the daemon for stateDir, spawning one if it is not
// running: the current executable is relaunched with `serve`, detached
// via platform.Launch so it outlives the caller. It waits up to
// spawnTimeout for the daemon to accept connections.
const spawnTimeout = 10 * time.Second

const requestTimeout = 30 * time.Second

func Ensure(opts EnsureOpts) (*Client, error) {
	path := socketPath(opts.StateDir)
	c, err := dialPath(path)
	if err == nil {
		return c, nil
	}
	if errors.Is(err, ErrProtocolMismatch) {
		return nil, err
	}
	if opts.NoSpawn {
		return nil, err
	}

	bin := opts.BinaryPath
	if bin == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("find executable: %w", err)
		}
		// A Go test binary (`go test` appends `.test`) must never be
		// relaunched with the daemon args: it would re-run the suite,
		// recursively spawn itself, and fork-bomb the machine. Tests
		// that need a daemon start one in-process instead.
		if strings.HasSuffix(filepath.Base(exe), ".test") {
			return nil, fmt.Errorf("%w: refusing to spawn a test binary", ErrNoDaemon)
		}
		bin = exe
	}
	args := []string{"serve"}
	if opts.StateDir != "" {
		args = append(args, "--state-dir", opts.StateDir)
	}
	if _, err := platform.New().Launch(platform.LaunchOpts{
		Cmd:  bin,
		Args: args,
	}); err != nil {
		return nil, fmt.Errorf("launch daemon: %w", err)
	}

	deadline := time.Now().Add(spawnTimeout)
	for time.Now().Before(deadline) {
		c, err = dialPath(path)
		if err == nil {
			return c, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("daemon did not come up within %s", spawnTimeout)
}
