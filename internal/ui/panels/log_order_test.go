package panels

import (
	"errors"
	"strings"
	"testing"
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

func TestDetailLogLinesRespectsOrder(t *testing.T) {
	logs := []string{"oldest", "middle", "newest"}

	newest, state := detailLogLines(logs, nil, false, 80, LogOrderNewest)
	if state != detailLogStateLoaded {
		t.Fatalf("state = %v, want loaded", state)
	}
	if got := indexOfLine(newest, "newest"); got != 0 {
		t.Errorf("newest-first: %q at row %d, want row 0\nrendered: %v", "newest", got, newest)
	}
	if got := indexOfLine(newest, "oldest"); got != 2 {
		t.Errorf("newest-first: %q at row %d, want row 2", "oldest", got)
	}

	oldest, _ := detailLogLines(logs, nil, false, 80, LogOrderOldest)
	if got := indexOfLine(oldest, "oldest"); got != 0 {
		t.Errorf("oldest-first: %q at row %d, want row 0", "oldest", got)
	}
	if got := indexOfLine(oldest, "newest"); got != 2 {
		t.Errorf("oldest-first: %q at row %d, want row 2", "newest", got)
	}
}

func TestStackDetailLogLinesRespectsOrder(t *testing.T) {
	logs := []string{"first", "second", "third"}

	newest, state := stackDetailLogLines(logs, nil, false, 80, LogOrderNewest)
	if state != detailLogStateLoaded {
		t.Fatalf("state = %v, want loaded", state)
	}
	if got := indexOfLine(newest, "third"); got != 0 {
		t.Errorf("newest-first: %q at row %d, want row 0", "third", got)
	}

	oldest, _ := stackDetailLogLines(logs, nil, false, 80, LogOrderOldest)
	if got := indexOfLine(oldest, "first"); got != 0 {
		t.Errorf("oldest-first: %q at row %d, want row 0", "first", got)
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
// to be mapped back or it lands on the wrong row.
func TestHighlightSearchLinesMapsThroughOrder(t *testing.T) {
	const total = 3
	// Storage index 2 ("newest") matches. Under newest-first it renders at
	// row 0, so row 0 is what must be highlighted.
	search := LogSearch{
		Query:       "newest",
		MatchSet:    map[int]bool{2: true},
		CurrentLine: 2,
		Total:       1,
		Current:     1,
	}

	visible := []string{"row0", "row1", "row2"}
	highlightSearchLines(visible, 0, search, 40, LogOrderNewest, total)
	if visible[0] == "row0" {
		t.Error("newest-first: row 0 was not highlighted, but holds the matching line")
	}
	if visible[2] != "row2" {
		t.Error("newest-first: row 2 was highlighted, but does not hold the matching line")
	}

	visible = []string{"row0", "row1", "row2"}
	highlightSearchLines(visible, 0, search, 40, LogOrderOldest, total)
	if visible[2] == "row2" {
		t.Error("oldest-first: row 2 was not highlighted, but holds the matching line")
	}
	if visible[0] != "row0" {
		t.Error("oldest-first: row 0 was highlighted, but does not hold the matching line")
	}
}
