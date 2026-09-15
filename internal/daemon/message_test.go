package daemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestReadLinePreservesBufferedRequests(t *testing.T) {
	br := bufio.NewReader(bytes.NewBufferString("one\ntwo\n"))
	first, err := readLine(br)
	if err != nil || string(first) != "one\n" {
		t.Fatalf("first line = %q, %v", first, err)
	}
	second, err := readLine(br)
	if err != nil || string(second) != "two\n" {
		t.Fatalf("second line = %q, %v", second, err)
	}
}

func TestReadLineRejectsOversizedMessage(t *testing.T) {
	br := bufio.NewReader(bytes.NewReader(bytes.Repeat([]byte{'x'}, maxMessageSize+1)))
	_, err := readLine(br)
	if !errors.Is(err, errMessageTooLarge) {
		t.Fatalf("readLine error = %v, want oversized error", err)
	}
}

func TestWriteJSONLineRejectsOversizedMessage(t *testing.T) {
	var out bytes.Buffer
	if err := writeJSONLine(&out, map[string]string{"value": string(bytes.Repeat([]byte{'x'}, maxMessageSize))}); !errors.Is(err, errMessageTooLarge) {
		t.Fatalf("writeJSONLine error = %v, want oversized error", err)
	}
}

func TestHelloRejectsUnsupportedProtocol(t *testing.T) {
	srv, err := NewServer(Options{StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.doHello(mustJSON(HelloParams{Protocol: ProtocolVersion + 1}))
	if err == nil {
		t.Fatal("unsupported protocol unexpectedly accepted")
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
