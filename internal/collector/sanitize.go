package collector

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// sanitizeTerminalText makes untrusted text safe to hand to the terminal.
//
// Container logs and image metadata are written by whoever controls the
// container. An application, or anyone able to write to its stdout, can emit
// sequences that clear the screen, retitle the window or, via OSC 52, write to
// the user's clipboard, and the renderer measures around escape sequences
// rather than removing them. The collector is the trust boundary, so the
// cleanup happens here once and search, rendering and copy all see clean text.
//
// Every ESC-initiated sequence and every control character other than tab is
// dropped. A carriage return keeps only what follows it, which is what the
// terminal would have shown for a progress bar; the trailing \r that
// `tty: true` containers put on every line goes with it.
func sanitizeTerminalText(s string) string {
	if !needsSanitizing(s) {
		return s
	}
	s = strings.TrimRight(s, "\r")
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	s = ansi.Strip(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteRune(r)
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			// C0 and C1 control characters.
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// needsSanitizing is the fast path: almost every log line is plain text, and
// the full pass allocates.
func needsSanitizing(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 0x20 && c != '\t') || c == 0x7f {
			return true
		}
		// C1 controls are encoded as 0xC2 0x80..0x9F in UTF-8.
		if c == 0xc2 && i+1 < len(s) && s[i+1] >= 0x80 && s[i+1] <= 0x9f {
			return true
		}
	}
	return false
}

// sanitizeLabels returns a copy of labels with keys and values sanitized.
// Labels are rendered in the detail panel and feed the compose command, and
// they are set by the image publisher, not the user.
func sanitizeLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		out[sanitizeTerminalText(k)] = sanitizeTerminalText(v)
	}
	return out
}
