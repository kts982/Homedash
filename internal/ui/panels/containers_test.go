package panels

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/ui/styles"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestFormatGroupHeaderIncludesStackSummaryCounts(t *testing.T) {
	got := formatGroupHeader("media", 3, 4, 1, 1, 1, false, 120)
	plain := stripANSI(got)

	for _, want := range []string{"▼ media", "3/4 up", "1 unhealthy", "1 starting", "1 stopped"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("formatGroupHeader() = %q, want substring %q", plain, want)
		}
	}
	if strings.Contains(plain, "\n") {
		t.Fatalf("formatGroupHeader() wrapped: %q", plain)
	}
	if lipgloss.Width(got) > 120 {
		t.Fatalf("formatGroupHeader() width = %d, want <= 120", lipgloss.Width(got))
	}
}

func TestFormatGroupHeaderClampsLongNamesWithoutWrapping(t *testing.T) {
	got := formatGroupHeader("very-long-stack-name-that-should-not-wrap-in-the-container-panel", 2, 5, 1, 0, 3, false, 40)
	plain := stripANSI(got)

	if strings.Contains(plain, "\n") {
		t.Fatalf("formatGroupHeader() wrapped: %q", plain)
	}
	if !strings.Contains(plain, "2/5") {
		t.Fatalf("formatGroupHeader() = %q, want running summary to remain visible", plain)
	}
	if lipgloss.Width(got) > 40 {
		t.Fatalf("formatGroupHeader() width = %d, want <= 40", lipgloss.Width(got))
	}
}

func TestRenderContainersShowsStackSummaryRow(t *testing.T) {
	input := textinput.New()
	view := RenderContainers([]ContainerDisplayItem{
		{
			IsGroup:        true,
			StackName:      "media",
			ContainerCount: 4,
			RunningCount:   3,
			UnhealthyCount: 1,
			StartingCount:  1,
			StoppedCount:   1,
		},
	}, 3, 4, 0, 0, 1, 90, true, input, false, false, "", 1, "")
	plain := stripANSI(view)

	for _, want := range []string{"3/4 up", "1 unhealthy", "1 starting", "1 stopped"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderContainers() = %q, want substring %q", plain, want)
		}
	}

	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 90 {
			t.Fatalf("RenderContainers() line width = %d, want <= 90", lipgloss.Width(line))
		}
	}
}

func TestRenderContainersShowsSortAndShownCounts(t *testing.T) {
	input := textinput.New()
	input.SetValue("state:running")
	view := RenderContainers(nil, 2, 3, 0, 0, 1, 100, true, input, false, false, "cpu", 2, "2s ago")
	plain := stripANSI(view)

	for _, want := range []string{"2 shown", "sort:cpu", "2s ago", "No containers match current filter"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("RenderContainers() = %q, want substring %q", plain, want)
		}
	}
}

func TestRenderContainerSortSummaryUsesSeparateStyles(t *testing.T) {
	if err := styles.ApplyNamed("tokyo-night"); err != nil {
		t.Fatalf("ApplyNamed() error = %v", err)
	}

	rendered := renderContainerSortSummary("cpu")
	plain := stripANSI(rendered)

	if plain != "sort:cpu" {
		t.Fatalf("renderContainerSortSummary() = %q, want %q", plain, "sort:cpu")
	}
	if len(ansiPattern.FindAllString(rendered, -1)) < 2 {
		t.Fatalf("renderContainerSortSummary() = %q, want multiple ANSI style segments", rendered)
	}
	if !strings.Contains(rendered, "sort:") || !strings.Contains(rendered, "cpu") {
		t.Fatalf("renderContainerSortSummary() = %q, want sort text and value", rendered)
	}
}
