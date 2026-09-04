package panels

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/ui/styles"
)

func TestLogOrderRenderIndexIsSelfInverse(t *testing.T) {
	const n = 7
	for _, order := range []LogOrder{LogOrderNewest, LogOrderOldest} {
		for i := 0; i < n; i++ {
			got := order.RenderIndex(order.RenderIndex(i, n), n)
			if got != i {
				t.Errorf("order %v: RenderIndex applied twice to %d = %d, want %d", order, i, got, i)
			}
		}
	}
}

func TestLogOrderRenderIndexMapping(t *testing.T) {
	// Storage is always oldest-first, so under newest-first the last stored
	// line must render at row 0.
	if got := LogOrderNewest.RenderIndex(4, 5); got != 0 {
		t.Errorf("newest: RenderIndex(4, 5) = %d, want 0 (last stored line renders first)", got)
	}
	if got := LogOrderNewest.RenderIndex(0, 5); got != 4 {
		t.Errorf("newest: RenderIndex(0, 5) = %d, want 4 (first stored line renders last)", got)
	}
	if got := LogOrderOldest.RenderIndex(4, 5); got != 4 {
		t.Errorf("oldest: RenderIndex(4, 5) = %d, want 4 (identity)", got)
	}
}

// indexOfLine reports which rendered row contains the given text, or -1.
func indexOfLine(lines []string, want string) int {
	for i, l := range lines {
		if strings.Contains(l, want) {
			return i
		}
	}
	return -1
}

func TestRenderLogWindowRespectsOrder(t *testing.T) {
	logs := []string{"oldest", "middle", "newest"}

	newest := renderLogWindow(logs, 0, 10, 80, LogOrderNewest, LogSearch{}, false)
	if got := indexOfLine(newest, "newest"); got != 0 {
		t.Errorf("newest-first: %q at row %d, want row 0\nrendered: %v", "newest", got, newest)
	}
	if got := indexOfLine(newest, "oldest"); got != 2 {
		t.Errorf("newest-first: %q at row %d, want row 2", "oldest", got)
	}

	oldest := renderLogWindow(logs, 0, 10, 80, LogOrderOldest, LogSearch{}, false)
	if got := indexOfLine(oldest, "oldest"); got != 0 {
		t.Errorf("oldest-first: %q at row %d, want row 0", "oldest", got)
	}
	if got := indexOfLine(oldest, "newest"); got != 2 {
		t.Errorf("oldest-first: %q at row %d, want row 2", "newest", got)
	}
}

// The loaded state defers formatting to renderLogWindow so that only the
// visible rows are rendered per frame.
func TestLoadedStateFormatsOnlyTheVisibleWindow(t *testing.T) {
	logs := make([]string, 1000)
	for i := range logs {
		logs[i] = fmt.Sprintf("line-%d", i)
	}

	lines, state := detailLogLines(logs, nil, false, 80, LogOrderNewest)
	if state != detailLogStateLoaded || lines != nil {
		t.Fatalf("detailLogLines(loaded) = %d lines, state %v; want nil lines and loaded", len(lines), state)
	}
	lines, state = stackDetailLogLines(logs, nil, false, 80, LogOrderNewest)
	if state != detailLogStateLoaded || lines != nil {
		t.Fatalf("stackDetailLogLines(loaded) = %d lines, state %v; want nil lines and loaded", len(lines), state)
	}

	rows := renderLogWindow(logs, 5, 3, 80, LogOrderNewest, LogSearch{}, false)
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	// Render row 5 under newest-first is storage index 999-5.
	if got := indexOfLine(rows, "line-994"); got != 0 {
		t.Errorf("row 0 = %q, want line-994", rows[0])
	}
	if got := indexOfLine(rows, "line-992"); got != 2 {
		t.Errorf("row 2 = %q, want line-992", rows[2])
	}
}

// A "[LEVEL]" prefix is a log level for a single container, and only a
// source name in the merged stack view.
func TestSourcePrefixOnlySplitsInStackView(t *testing.T) {
	line := "2024-03-03T12:00:01Z [ERROR] db down"
	single := formatLogLine(line, 80, logLineStyle{})
	// Stack lines carry the container name first; the level follows it.
	stack := formatLogLine("2024-03-03T12:00:01Z [web] [ERROR] db down", 80, logLineStyle{withSource: true})
	plainError := formatLogLine("2024-03-03T12:00:01Z ERROR db down", 80, logLineStyle{})

	errorColour := lipgloss.NewStyle().Foreground(styles.Error).Render("x")
	errorSGR := errorColour[:strings.Index(errorColour, "m")+1]
	if !strings.Contains(plainError, errorSGR) {
		t.Skip("renderer emits no colour in this environment")
	}
	if !strings.Contains(single, errorSGR) {
		t.Errorf("single-container [ERROR] line lost its level colour: %q", single)
	}
	if !strings.Contains(stack, errorSGR) {
		t.Errorf("stack line with [ERROR] after the source lost its level colour: %q", stack)
	}
}

// Placeholder states (loading, error, empty) are prose, not log data, and must
// read top-to-bottom regardless of order.
func TestDetailLogLinesDoesNotReverseNonLoadedStates(t *testing.T) {
	cases := []struct {
		name      string
		logs      []string
		err       error
		following bool
		wantFirst string
	}{
		{"loading", nil, nil, false, "Loading logs"},
		{"waiting", nil, nil, true, "Waiting for live log output"},
		{"error", nil, errors.New("socket closed"), false, "Log refresh failed"},
		{"empty", []string{}, nil, false, "No logs available"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines, _ := detailLogLines(tc.logs, tc.err, tc.following, 80, LogOrderNewest)
			if len(lines) == 0 {
				t.Fatal("no lines rendered")
			}
			if !strings.Contains(lines[0], tc.wantFirst) {
				t.Errorf("first line = %q, want it to contain %q", lines[0], tc.wantFirst)
			}
		})
	}
}

func TestLogOrderStringAppearsInTitle(t *testing.T) {
	newest := renderLogTitle(detailLogStateLoaded, 0, 10, 3, 100, false, LogOrderNewest)
	if !strings.Contains(newest, "newest") {
		t.Errorf("title = %q, want it to mention newest", newest)
	}
	oldest := renderLogTitle(detailLogStateLoaded, 0, 10, 3, 100, false, LogOrderOldest)
	if !strings.Contains(oldest, "oldest") {
		t.Errorf("title = %q, want it to mention oldest", oldest)
	}
}

// MatchSet is keyed by storage index, so under newest-first the highlight has
// to be mapped back or it lands on the wrong row. And the highlight must sit
// on the matched text itself: wrapping a pre-styled row in a background
// style used to stop at the timestamp's reset and miss the message.
func TestSearchHighlightCoversMatchedText(t *testing.T) {
	logs := []string{
		"2024-03-03T12:00:01Z hello world",
		"2024-03-03T12:00:02Z other",
		"2024-03-03T12:00:03Z newest hello",
	}
	// Storage index 2 matches. Under newest-first it renders at row 0.
	search := LogSearch{
		Query:       "newest",
		MatchSet:    map[int]bool{2: true},
		CurrentLine: 2,
		Total:       1,
		Current:     1,
	}

	plain := renderLogWindow(logs, 0, 3, 60, LogOrderNewest, LogSearch{}, false)
	rows := renderLogWindow(logs, 0, 3, 60, LogOrderNewest, search, false)
	if rows[0] == plain[0] {
		t.Error("newest-first: row 0 holds the matching line but was not highlighted")
	}
	if rows[2] != plain[2] {
		t.Error("newest-first: row 2 was highlighted but does not hold the matching line")
	}

	idx := strings.Index(rows[0], "newest hello")
	if idx < 0 {
		t.Fatalf("match text missing from highlighted row: %q", rows[0])
	}
	segment := rows[0][:idx]
	if i := strings.LastIndex(segment, "\x1b[m"); i >= 0 {
		segment = segment[i+3:]
	}
	if !strings.Contains(segment, "48;") {
		t.Errorf("no background (SGR 48) set immediately before the matched text; row = %q", rows[0])
	}

	plainOld := renderLogWindow(logs, 0, 3, 60, LogOrderOldest, LogSearch{}, false)
	rowsOld := renderLogWindow(logs, 0, 3, 60, LogOrderOldest, search, false)
	if rowsOld[2] == plainOld[2] {
		t.Error("oldest-first: row 2 holds the matching line but was not highlighted")
	}
	if rowsOld[0] != plainOld[0] {
		t.Error("oldest-first: row 0 was highlighted but does not hold the matching line")
	}
}
