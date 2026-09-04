package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePositiveDuration(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		minimum time.Duration
		want    time.Duration
		wantErr bool
	}{
		{"valid above minimum", "refresh.system", "5s", 1 * time.Second, 5 * time.Second, false},
		{"valid at minimum", "refresh.docker", "3s", 3 * time.Second, 3 * time.Second, false},
		{"below minimum", "refresh.system", "500ms", 1 * time.Second, 0, true},
		{"zero", "refresh.docker", "0s", 1 * time.Second, 0, true},
		{"negative", "refresh.weather", "-5s", 1 * time.Minute, 0, true},
		{"invalid format", "refresh.system", "abc", 1 * time.Second, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePositiveDuration(tt.field, tt.value, tt.minimum)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePositiveDuration() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parsePositiveDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadUnknownFields(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "homedash")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			"valid fields accepted",
			"theme: dracula\nrefresh:\n  system: 2s\n",
			false,
		},
		{
			"unknown top-level field rejected",
			"theme: dracula\ntypo_field: something\n",
			true,
		},
		{
			"unknown nested field rejected",
			"refresh:\n  systme: 2s\n",
			true,
		},
		{
			"unknown docker field rejected",
			"docker:\n  hsot: tcp://localhost:2375\n",
			true,
		},
		{
			"empty file returns defaults",
			"",
			false,
		},
		{
			"comment-only file returns defaults",
			"# just a comment\n",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_CONFIG_HOME", tmpDir)

			_, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadMinimumEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "homedash")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			"system below minimum",
			"refresh:\n  system: 500ms\n",
			true,
		},
		{
			"docker below minimum",
			"refresh:\n  docker: 2s\n",
			true,
		},
		{
			"weather below minimum",
			"refresh:\n  weather: 30s\n",
			true,
		},
		{
			"all at minimums",
			"refresh:\n  system: 1s\n  docker: 3s\n  weather: 1m\n",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}

			// Override XDG_CONFIG_HOME to use our temp dir
			t.Setenv("XDG_CONFIG_HOME", tmpDir)

			_, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadTheme(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "homedash")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		yaml      string
		wantTheme string
		wantErr   bool
	}{
		{
			"valid theme",
			"theme: dracula\n",
			"dracula",
			false,
		},
		{
			"empty theme uses default",
			"theme: \"\"\n",
			"",
			false,
		},
		{
			"no theme field uses default",
			"system:\n  disks: []\n",
			"",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_CONFIG_HOME", tmpDir)

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && cfg.Theme != tt.wantTheme {
				t.Fatalf("Theme = %q, want %q", cfg.Theme, tt.wantTheme)
			}
		})
	}
}

func TestDiscoverDisksFromProcMounts(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		"/dev/nvme0n1p2 / ext4 rw,relatime 0 0",
		"tmpfs /run tmpfs rw,nosuid,nodev 0 0",
		"/dev/sdb1 /mnt/data xfs rw,relatime 0 0",
		"/dev/sdc1 /media/user/Backup ext4 rw,relatime 0 0",
		"tank/archive /mnt/archive zfs rw,relatime 0 0",
		"/dev/nvme0n1p1 /boot/efi vfat rw,relatime 0 0",
		"/dev/sdd1 /run/media/user/USB\\040Disk exfat rw,relatime 0 0",
	}, "\n"))

	disks, err := discoverDisksFromProcMounts(input)
	if err != nil {
		t.Fatalf("discoverDisksFromProcMounts() error = %v", err)
	}

	var got []string
	for _, disk := range disks {
		got = append(got, disk.Path)
	}
	want := []string{
		"/",
		"/media/user/Backup",
		"/mnt/archive",
		"/mnt/data",
		"/run/media/user/USB Disk",
	}
	if len(got) != len(want) {
		t.Fatalf("len(disks) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("disks[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}

	wantLabels := []string{"/", "Backup", "archive", "data", "USB Disk"}
	for i := range wantLabels {
		if disks[i].Label != wantLabels[i] {
			t.Fatalf("disks[%d].Label = %q, want %q", i, disks[i].Label, wantLabels[i])
		}
	}
}

func TestSaveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := Config{
		Theme: "dracula",
		System: SystemConfig{
			Disks: []Disk{
				{Path: "/", Label: "System"},
				{Path: "/mnt/archive", Label: "Archive"},
			},
		},
		Refresh: RefreshConfig{
			System:  3 * time.Second,
			Docker:  10 * time.Second,
			Weather: 15 * time.Minute,
		},
		Docker: DockerConfig{
			Host: "tcp://127.0.0.1:2375",
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	configPath := filepath.Join(tmpDir, "homedash", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Theme != cfg.Theme {
		t.Fatalf("Theme = %q, want %q", loaded.Theme, cfg.Theme)
	}
	if loaded.Refresh != cfg.Refresh {
		t.Fatalf("Refresh = %+v, want %+v", loaded.Refresh, cfg.Refresh)
	}
	if loaded.Docker.Host != cfg.Docker.Host {
		t.Fatalf("Docker.Host = %q, want %q", loaded.Docker.Host, cfg.Docker.Host)
	}
	if len(loaded.System.Disks) != len(cfg.System.Disks) {
		t.Fatalf("len(Disks) = %d, want %d", len(loaded.System.Disks), len(cfg.System.Disks))
	}
	for i := range cfg.System.Disks {
		if loaded.System.Disks[i] != cfg.System.Disks[i] {
			t.Fatalf("Disk[%d] = %+v, want %+v", i, loaded.System.Disks[i], cfg.System.Disks[i])
		}
	}
}

func TestNormalizeLogOrder(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		want        string
		wantWarning bool
	}{
		{"newest", "newest", LogOrderNewest, false},
		{"oldest", "oldest", LogOrderOldest, false},
		{"empty defaults to newest", "", LogOrderNewest, false},
		{"whitespace defaults to newest", "   ", LogOrderNewest, false},
		{"mixed case accepted", "Oldest", LogOrderOldest, false},
		{"padded value accepted", "  newest  ", LogOrderNewest, false},
		{"typo falls back and warns", "newst", LogOrderNewest, true},
		{"unrelated value falls back and warns", "reverse", LogOrderNewest, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warning := normalizeLogOrder(tt.value)
			if got != tt.want {
				t.Errorf("normalizeLogOrder(%q) = %q, want %q", tt.value, got, tt.want)
			}
			if (warning != "") != tt.wantWarning {
				t.Errorf("normalizeLogOrder(%q) warning = %q, wantWarning %v", tt.value, warning, tt.wantWarning)
			}
			if tt.wantWarning && !strings.Contains(warning, tt.value) {
				t.Errorf("warning %q should quote the offending value %q", warning, tt.value)
			}
		})
	}
}

// A bad logs.order must not fail the load — it is a display preference, and
// erroring would let a typo prevent startup entirely.
func TestLoadLogOrder(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "homedash")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		yaml         string
		wantOrder    string
		wantWarnings int
	}{
		{"explicit oldest", "logs:\n  order: oldest\n", LogOrderOldest, 0},
		{"explicit newest", "logs:\n  order: newest\n", LogOrderNewest, 0},
		{"absent defaults to newest", "theme: dracula\n", LogOrderNewest, 0},
		{"invalid falls back and warns", "logs:\n  order: sideways\n", LogOrderNewest, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_CONFIG_HOME", tmpDir)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v, want nil (bad log order must never fail the load)", err)
			}
			if cfg.Logs.Order != tt.wantOrder {
				t.Errorf("Logs.Order = %q, want %q", cfg.Logs.Order, tt.wantOrder)
			}
			if len(cfg.Warnings) != tt.wantWarnings {
				t.Errorf("Warnings = %v, want %d warning(s)", cfg.Warnings, tt.wantWarnings)
			}
		})
	}
}

func TestSaveRoundTripsLogOrder(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := Default()
	cfg.Logs.Order = LogOrderOldest
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Logs.Order != LogOrderOldest {
		t.Errorf("Logs.Order after round trip = %q, want %q", loaded.Logs.Order, LogOrderOldest)
	}
}

func TestSavePreservesExistingFileMode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	configPath := filepath.Join(tmpDir, "homedash", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("theme: nord\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := Default()
	cfg.Theme = "dracula"
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600 (a user's tightened permissions must survive a save)", got)
	}
	if _, err := os.Stat(configPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
}
