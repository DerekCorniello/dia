package registry

import (
	"fmt"
	"path/filepath"
	"strings"
)

// firefoxFamily lists browser binaries that share Firefox's command-line
// conventions (Gecko-based forks, including Zen). Passing several bare
// positional URLs to these only opens the first as a tab in the running
// instance and drops the rest, so each one needs its own "-new-tab" flag.
var firefoxFamily = map[string]bool{
	"firefox":     true,
	"firefox-esr": true,
	"zen":         true,
	"zen-browser": true,
	"librewolf":   true,
	"waterfox":    true,
	"floorp":      true,
	"icecat":      true,
}

// browserLaunch decides how to invoke bin so urls end up open as tabs,
// honoring newWindow. It returns the process to exec — which for one
// specific Firefox-family case is a shell wrapping the real browser,
// not the browser itself — and that process's arguments.
func browserLaunch(bin string, urls []string, newWindow bool) (cmd string, args []string) {
	if !firefoxFamily[filepath.Base(bin)] {
		// Chromium family / fallback: "--new-window" is prepended once,
		// ahead of the whole positional URL list, matching Chromium's
		// own documented "force a new window" semantics.
		args = append([]string(nil), urls...)
		if newWindow {
			args = append([]string{"--new-window"}, args...)
		}
		return bin, args
	}

	if !newWindow || len(urls) <= 1 {
		args = make([]string, 0, len(urls)*2)
		for i, u := range urls {
			flag := "-new-tab"
			if newWindow && i == 0 {
				flag = "-new-window"
			}
			args = append(args, flag, u)
		}
		return bin, args
	}

	// Firefox/Zen quirk, confirmed against a real zen-browser instance:
	// bundling "-new-window <a> -new-tab <b> -new-tab <c>" into one
	// remote command is racy. The already-running instance can dispatch
	// the "-new-tab" calls to whichever window currently holds focus
	// rather than the one "-new-window" just created moments earlier in
	// the same command, landing them in an unrelated pre-existing window
	// instead of the fresh one. Splitting into two remote calls with a
	// short pause between them — enough time for the new window to
	// actually take focus before the rest arrive — avoids the race in
	// practice. Needs a shell since it's two sequential invocations of
	// the same binary; args are shell-quoted since urls come from the
	// workspace's own YAML, not sanitized input, but still shouldn't be
	// interpolated unquoted into a shell string.
	var b strings.Builder
	fmt.Fprintf(&b, "%s -new-window %s >/dev/null 2>&1; sleep 0.6; %s", shQuote(bin), shQuote(urls[0]), shQuote(bin))
	for _, u := range urls[1:] {
		fmt.Fprintf(&b, " -new-tab %s", shQuote(u))
	}
	b.WriteString(" >/dev/null 2>&1")
	return "sh", []string{"-c", b.String()}
}

// shQuote wraps s in single quotes for use as one word in a POSIX shell
// command, escaping any single quotes already in s.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
