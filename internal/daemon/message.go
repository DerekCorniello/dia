// Package daemon implements the dia session daemon. The daemon owns
// the workspace runtime and supervises app processes; the GUI and CLI
// are clients that talk to it over a unix socket (named pipe on
// Windows). Keeping one process in charge means workspaces survive the
// GUI closing: detaching is just disconnecting, and the daemon keeps
// running and supervising until `dia shutdown`.
package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const maxMessageSize = 4 << 20

const ProtocolVersion = 1

var errMessageTooLarge = errors.New("daemon message exceeds 4 MiB")

// request is one client request read from the socket. ID is echoed
// back in the response so a client can correlate out-of-order replies.
// Method is one of the verbs below; Params was decoded as
// json.RawMessage so handlers can unmarshal their own shapes.
type request struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Verbs. Stable names because both sides have to agree across
// versions; add verbs by appending, never rename.
const (
	MethodStart     = "start"
	MethodStop      = "stop"
	MethodRestart   = "restart"
	MethodStopAll   = "stop-all"
	MethodList      = "list"
	MethodStatus    = "status"
	MethodReconcile = "reconcile"
	MethodVersion   = "version"
	MethodShutdown  = "shutdown"
	MethodHello     = "hello"
)

type HelloParams struct {
	Protocol int `json:"protocol"`
}

type HelloReply struct {
	Protocol int `json:"protocol"`
}

// response is the reply to a single request. Exactly one of Result
// and Error is set. Result is a raw payload: an Instance for
// start/restart, a count for stop-all/reconcile, a list for list.
type response struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func writeRequest(w io.Writer, id int64, method string, params any) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		raw = b
	}
	return writeJSONLine(w, request{ID: id, Method: method, Params: raw})
}

func writeResponse(w io.Writer, r response) error {
	return writeJSONLine(w, r)
}

func writeJSONLine(w io.Writer, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(b)+1 > maxMessageSize {
		return errMessageTooLarge
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func readLine(br *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := br.ReadSlice('\n')
		line = append(line, part...)
		if len(line) > maxMessageSize {
			return nil, errMessageTooLarge
		}
		if err == nil {
			return line, nil
		}
		if err != bufio.ErrBufferFull {
			return nil, err
		}
	}
}
