package collector

import "testing"

func TestSanitizeTerminalText(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain text untouched", "GET /health 200 3ms", "GET /health 200 3ms"},
		{"tabs are kept", "a\tb", "a\tb"},
		{"unicode is kept", "Ελλάδα → ok ✓", "Ελλάδα → ok ✓"},
		{"csi clear screen", "before \x1b[2J\x1b[H after", "before  after"},
		{"sgr colours", "\x1b[31mERROR\x1b[0m boom", "ERROR boom"},
		{"osc title and clipboard", "x\x1b]0;PWNED\x07y\x1b]52;c;aGk=\x1b\\z", "xyz"},
		{"nul, bell, delete", "a\x00b\x07c\x7fd", "abcd"},
		{"c1 controls", "a\u0085b\u009fc", "abc"},
		{"trailing cr from tty containers", "line one\r", "line one"},
		{"cr keeps the last overwrite", "10%\r20%\r30%", "30%"},
		{"cr then trailing cr", "10%\r100%\r\r", "100%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeTerminalText(tt.in); got != tt.want {
				t.Errorf("sanitizeTerminalText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeLabels(t *testing.T) {
	if sanitizeLabels(nil) != nil {
		t.Fatal("nil labels should stay nil")
	}
	got := sanitizeLabels(map[string]string{"com.docker.compose.service": "web\x1b[2J", "k\x07": "v"})
	if got["com.docker.compose.service"] != "web" || got["k"] != "v" {
		t.Fatalf("sanitizeLabels() = %#v", got)
	}
}
