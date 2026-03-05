package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultDockerHost = "unix:///var/run/docker.sock"
)

type Config struct {
	Theme   string        `yaml:"theme"`
	System  SystemConfig  `yaml:"system"`
	Refresh RefreshConfig `yaml:"refresh"`
	Docker  DockerConfig  `yaml:"docker"`
}

type SystemConfig struct {
	Disks []Disk `yaml:"disks"`
}

type Disk struct {
	Path  string `yaml:"path"`
	Label string `yaml:"label,omitempty"`
}

type RefreshConfig struct {
	System  time.Duration `yaml:"system"`
	Docker  time.Duration `yaml:"docker"`
	Weather time.Duration `yaml:"weather"`
}

type DockerConfig struct {
	Host string `yaml:"host"`
}

type fileConfig struct {
	Theme string `yaml:"theme"`
	System struct {
		Disks []Disk `yaml:"disks"`
	} `yaml:"system"`
	Refresh struct {
		System  string `yaml:"system"`
		Docker  string `yaml:"docker"`
		Weather string `yaml:"weather"`
	} `yaml:"refresh"`
	Docker struct {
		Host string `yaml:"host"`
	} `yaml:"docker"`
}

func Default() Config {
	return Config{
		System: SystemConfig{
			Disks: []Disk{
				{Path: "/", Label: "/"},
				{Path: "/mnt/docker-data", Label: "/data"},
			},
		},
		Refresh: RefreshConfig{
			System:  2 * time.Second,
			Docker:  5 * time.Second,
			Weather: 5 * time.Minute,
		},
	}
}

func Load() (Config, error) {
	cfg := Default()

	configRoot, err := os.UserConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	configPath := filepath.Join(configRoot, "homedash", "config.yaml")

	raw, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config file %q: %w", configPath, err)
	}

	var parsed fileConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&parsed); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("parse config file %q: %w", configPath, err)
	}

	if parsed.System.Disks != nil {
		cfg.System.Disks = parsed.System.Disks
	}
	if host := strings.TrimSpace(parsed.Docker.Host); host != "" {
		cfg.Docker.Host = host
	}
	if parsed.Refresh.System != "" {
		duration, err := parsePositiveDuration("refresh.system", parsed.Refresh.System, 1*time.Second)
		if err != nil {
			return Config{}, err
		}
		cfg.Refresh.System = duration
	}
	if parsed.Refresh.Docker != "" {
		duration, err := parsePositiveDuration("refresh.docker", parsed.Refresh.Docker, 3*time.Second)
		if err != nil {
			return Config{}, err
		}
		cfg.Refresh.Docker = duration
	}
	if parsed.Refresh.Weather != "" {
		duration, err := parsePositiveDuration("refresh.weather", parsed.Refresh.Weather, 1*time.Minute)
		if err != nil {
			return Config{}, err
		}
		cfg.Refresh.Weather = duration
	}
	if theme := strings.TrimSpace(parsed.Theme); theme != "" {
		cfg.Theme = theme
	}

	for i, disk := range cfg.System.Disks {
		normalized, err := validateDisk(i, disk)
		if err != nil {
			return Config{}, err
		}
		cfg.System.Disks[i] = normalized
	}

	return cfg, nil
}

func (c Config) EffectiveDockerHost() string {
	if host := strings.TrimSpace(os.Getenv("DOCKER_HOST")); host != "" {
		return host
	}
	if host := strings.TrimSpace(c.Docker.Host); host != "" {
		return host
	}
	return defaultDockerHost
}

func parsePositiveDuration(field, value string, minimum time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid config: %s has invalid duration %q: %w", field, value, err)
	}
	if duration < minimum {
		return 0, fmt.Errorf("invalid config: %s must be >= %s (got %q)", field, minimum, value)
	}
	return duration, nil
}

func validateDisk(index int, disk Disk) (Disk, error) {
	path := filepath.Clean(strings.TrimSpace(disk.Path))
	if path == "" || path == "." {
		return Disk{}, fmt.Errorf("invalid config: system.disks[%d].path is required", index)
	}
	if !filepath.IsAbs(path) {
		return Disk{}, fmt.Errorf("invalid config: system.disks[%d].path must be absolute (got %q)", index, disk.Path)
	}

	label := strings.TrimSpace(disk.Label)
	if label == "" {
		label = path
	}

	return Disk{
		Path:  path,
		Label: label,
	}, nil
}
