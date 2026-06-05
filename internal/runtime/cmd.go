package runtime

import (
	"fmt"
	"strings"
)

// splitCmd parses a single Cmd string into the program name and its
// arguments. It honours single and double quotes so that paths with
// spaces work, e.g. code "/Users/me/My Code". The caller can still
// append extra Args afterwards; this only handles the case where the
// user packed everything into Cmd for convenience.
func splitCmd(cmd string) (string, []string, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", nil, fmt.Errorf("splitCmd: empty command")
	}
	var args []string
	var cur strings.Builder
	var quote byte
	flush := func() {
		args = append(args, cur.String())
		cur.Reset()
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				flush()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if quote != 0 {
		return "", nil, fmt.Errorf("splitCmd: unterminated %q in %q", quote, cmd)
	}
	if cur.Len() > 0 {
		flush()
	}
	if len(args) == 0 {
		return "", nil, fmt.Errorf("splitCmd: no tokens in %q", cmd)
	}
	return args[0], args[1:], nil
}
