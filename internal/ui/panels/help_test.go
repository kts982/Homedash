package panels

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

var helpANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripHelpANSI(s string) string {
	return helpANSI.ReplaceAllString(s, "")
}

func TestRenderHelpShowsAlertBadge(t *testing.T) {
	view := RenderHelp(DefaultBindings, false, false, 120, "3 alerts")
	plain := stripHelpANSI(view)

	if !strings.Contains(plain, "3 alerts") {
		t.Fatalf("RenderHelp() = %q, want alert badge", plain)
	}
}

func TestRenderHelpCompactsBindingsWhenNarrow(t *testing.T) {
	view := RenderHelp(DefaultBindings, false, false, 60, "")
	plain := stripHelpANSI(view)

	if !strings.Contains(plain, "j/k nav") {
		t.Fatalf("RenderHelp() = %q, want compact navigation binding", plain)
	}
	if strings.Contains(plain, "open/toggle") {
		t.Fatalf("RenderHelp() = %q, want compact labels in narrow width", plain)
	}

	for _, line := range strings.Split(plain, "\n") {
		if lipgloss.Width(line) > 60 {
			t.Fatalf("RenderHelp() line width = %d, want <= 60", lipgloss.Width(line))
		}
	}
}
