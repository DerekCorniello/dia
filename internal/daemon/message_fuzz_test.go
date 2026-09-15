package daemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzReadLineNeverPanics(f *testing.F) {
	f.Add([]byte("{\"id\":1}\n"))
	f.Add([]byte("partial"))
	f.Add(bytes.Repeat([]byte{'x'}, 1024))
	f.Fuzz(func(t *testing.T, input []byte) {
		br := bufio.NewReader(bytes.NewReader(input))
		_, _ = readLine(br)
	})
}

func FuzzRequestEnvelopeNeverPanics(f *testing.F) {
	f.Add([]byte(`{"id":1,"method":"list","params":null}`))
	f.Add([]byte(`{"id":"wrong","method":7}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		var req request
		_ = json.Unmarshal(input, &req)
	})
}
