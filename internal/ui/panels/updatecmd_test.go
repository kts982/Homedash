package panels

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/collector"
)

func TestUpdateCommand(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			"single compose file",
			map[string]string{
				labelComposeConfigFiles: "/home/kostas/docker/stacks/mcp/compose.yaml",
				labelComposeService:     "mcp-sap-docs-abap",
			},
			"docker compose -f /home/kostas/docker/stacks/mcp/compose.yaml up -d --pull always mcp-sap-docs-abap",
		},
		{
			"override files are preserved in order",
			map[string]string{
				labelComposeConfigFiles: "/srv/compose.yaml,/srv/compose.override.yaml",
				labelComposeService:     "web",
			},
			"docker compose -f /srv/compose.yaml -f /srv/compose.override.yaml up -d --pull always web",
		},
		{
			"path with a space is quoted",
			map[string]string{
				labelComposeConfigFiles: "/home/my stacks/compose.yaml",
				labelComposeService:     "web",
			},
			"docker compose -f '/home/my stacks/compose.yaml' up -d --pull always web",
		},
		{
			"whitespace around entries is trimmed",
			map[string]string{
				labelComposeConfigFiles: " /srv/a.yaml , /srv/b.yaml ",
				labelComposeService:     " web ",
			},
			"docker compose -f /srv/a.yaml -f /srv/b.yaml up -d --pull always web",
		},

		// Without compose labels there is no correct command to offer, and a
		// wrong one is worse than none.
		{"not a compose container", map[string]string{}, ""},
		{"nil labels", nil, ""},
		{
			"config files but no service",
			map[string]string{labelComposeConfigFiles: "/srv/compose.yaml"},
			"",
		},
		{
			"service but no config files",
			map[string]string{labelComposeService: "web"},
			"",
		},
		{
			"config files present but empty",
			map[string]string{labelComposeConfigFiles: " , ", labelComposeService: "web"},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UpdateCommand(tt.labels); got != tt.want {
				t.Errorf("UpdateCommand() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestQuoteIfNeeded(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/srv/compose.yaml", "/srv/compose.yaml"},
		{"simple-name", "simple-name"},
		{"/home/my stacks/c.yaml", "'/home/my stacks/c.yaml'"},
		{"has$dollar", "'has$dollar'"},
		{"semi;colon", "'semi;colon'"},
		{"it's", `'it'\''s'`},
	}
	for _, tt := range tests {
		if got := quoteIfNeeded(tt.in); got != tt.want {
			t.Errorf("quoteIfNeeded(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderContainersShowsUpdateMarker(t *testing.T) {
	input := textinput.New()
	items := []ContainerDisplayItem{
		{Container: &collector.Container{Name: "stale-one", State: "running"}, UpdateAvailable: true},
		{Container: &collector.Container{Name: "fresh-one", State: "running"}},
	}

	view := stripANSI(RenderContainers(items, 2, 2, 0, 0, 4, 110, true, input, false, false, "", 2, "", "1 updates"))

	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "stale-one") && !strings.Contains(line, "⬆") {
			t.Errorf("row with an update is missing the marker: %q", line)
		}
		if strings.Contains(line, "fresh-one") && strings.Contains(line, "⬆") {
			t.Errorf("up-to-date row should not carry the marker: %q", line)
		}
	}
	if !strings.Contains(view, "1 updates") {
		t.Error("header does not show the update summary")
	}
}

// The marker replaces the row indent, so flagging a row must not change the
// panel width or shift the columns.
func TestUpdateMarkerDoesNotChangePanelWidth(t *testing.T) {
	input := textinput.New()
	const width = 110

	widths := func(updated bool) []int {
		items := []ContainerDisplayItem{
			{Container: &collector.Container{Name: "svc", State: "running"}, UpdateAvailable: updated},
		}
		view := RenderContainers(items, 1, 1, 0, 0, 4, width, true, input, false, false, "", 1, "", "")
		var out []int
		for _, line := range strings.Split(view, "\n") {
			out = append(out, lipgloss.Width(line))
		}
		return out
	}

	plain, marked := widths(false), widths(true)
	if len(plain) != len(marked) {
		t.Fatalf("line count differs: %d vs %d", len(plain), len(marked))
	}
	for i := range plain {
		if plain[i] != marked[i] {
			t.Errorf("line %d width %d without marker, %d with it", i, plain[i], marked[i])
		}
		if marked[i] > width {
			t.Errorf("line %d is %d wide, exceeds panel width %d", i, marked[i], width)
		}
	}
}

func TestUpdateDetailLines(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	checked := now.Add(-5 * time.Minute)

	tests := []struct {
		name        string
		info        *UpdateInfo
		wantSubstrs []string
		wantAbsent  []string
	}{
		{"no check yet renders nothing", nil, nil, nil},
		{
			"available shows both digests and the command",
			&UpdateInfo{
				State:        "available",
				LocalDigest:  "sha256:1111111111111111",
				RemoteDigest: "sha256:2222222222222222",
				Command:      "docker compose -f /srv/c.yaml up -d --pull always web",
				CheckedAt:    checked,
			},
			[]string{"update available", "111111111111", "222222222222", "docker compose", "[c] copy", "checked 5m ago"},
			nil,
		},
		{
			"available without compose labels offers no command",
			&UpdateInfo{State: "available", LocalDigest: "sha256:aaaa", RemoteDigest: "sha256:bbbb", CheckedAt: checked},
			[]string{"update available"},
			[]string{"[c] copy", "docker compose"},
		},
		{
			"current",
			&UpdateInfo{State: "current", LocalDigest: "sha256:1111111111111111", CheckedAt: checked},
			[]string{"up to date", "111111111111"},
			[]string{"[c] copy"},
		},
		{
			"unwatchable explains itself and is not alarming",
			&UpdateInfo{State: "unwatchable", Reason: "not found in registry-1.docker.io", CheckedAt: checked},
			[]string{"not tracked", "not found in registry-1.docker.io"},
			[]string{"update available"},
		},
		{
			"error explains itself",
			&UpdateInfo{State: "error", Reason: "connection refused", CheckedAt: checked},
			[]string{"check failed", "connection refused"},
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := updateDetailLines(tt.info, 200, now)
			if tt.info == nil {
				if len(lines) != 0 {
					t.Fatalf("got %d lines for a nil update, want 0", len(lines))
				}
				return
			}
			joined := stripANSI(strings.Join(lines, "\n"))
			for _, want := range tt.wantSubstrs {
				if !strings.Contains(joined, want) {
					t.Errorf("output missing %q:\n%s", want, joined)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(joined, absent) {
					t.Errorf("output should not contain %q:\n%s", absent, joined)
				}
			}
		})
	}
}

func TestCheckAgeLabel(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"zero time", time.Time{}, ""},
		{"seconds", now.Add(-30 * time.Second), "just checked"},
		{"minutes", now.Add(-7 * time.Minute), "checked 7m ago"},
		{"hours", now.Add(-3 * time.Hour), "checked 3h ago"},
		{"days", now.Add(-50 * time.Hour), "checked 2d ago"},
	}
	for _, tt := range tests {
		if got := checkAgeLabel(tt.at, now); got != tt.want {
			t.Errorf("%s: checkAgeLabel() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestShortDigest(t *testing.T) {
	tests := []struct{ in, want string }{
		{"sha256:1234567890abcdef1234", "1234567890ab"},
		{"sha256:abcd", "abcd"},
		{"", "-"},
	}
	for _, tt := range tests {
		if got := shortDigest(tt.in); got != tt.want {
			t.Errorf("shortDigest(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
