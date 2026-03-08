package panels

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderPreviewShowsStackSummary(t *testing.T) {
	view := RenderPreview(nil, &StackPreview{
		Name:           "media",
		ContainerCount: 4,
		RunningCount:   3,
		UnhealthyCount: 1,
		StartingCount:  1,
		StoppedCount:   1,
		CPUPerc:        17.5,
		MemUsed:        512 * 1024 * 1024,
	}, "", "", "", 100)
	plain := stripANSI(view)

	for _, want := range []string{"stack media", "3/4 up", "1 unhealthy", "1 starting", "1 stopped", "cpu 17.5%", "mem 512M"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderPreview() = %q, want substring %q", plain, want)
		}
	}

	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 100 {
			t.Fatalf("RenderPreview() line width = %d, want <= 100", lipgloss.Width(line))
		}
	}
}
