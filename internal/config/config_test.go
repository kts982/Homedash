package config

import (
	"os"
	"path/filepath"
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
