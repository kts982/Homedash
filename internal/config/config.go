package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultDockerHost = "unix:///var/run/docker.sock"
)

var localMountPrefixes = []string{
	"/mnt",
	"/media",
	"/run/media",
}

var localFilesystemTypes = map[string]struct{}{
	"btrfs":    {},
	"exfat":    {},
	"ext2":     {},
	"ext3":     {},
	"ext4":     {},
	"f2fs":     {},
	"jfs":      {},
	"nilfs2":   {},
	"ntfs":     {},
	"reiserfs": {},
	"udf":      {},
	"vfat":     {},
	"xfs":      {},
	"zfs":      {},
}

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
	Theme  string `yaml:"theme"`
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
			},
		},
		Refresh: RefreshConfig{
			System:  2 * time.Second,
			Docker:  5 * time.Second,
			Weather: 5 * time.Minute,
		},
	}
}

func Path() (string, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configRoot, "homedash", "config.yaml"), nil
}

func Load() (Config, error) {
	cfg := Default()

	configPath, err := Path()
	if err != nil {
		return Config{}, err
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.System.Disks = discoveredOrDefaultDisks()
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

func Save(cfg Config) error {
	configPath, err := Path()
	if err != nil {
		return err
	}

	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return err
	}

	file := fileConfig{
		Theme: normalized.Theme,
	}
	file.System.Disks = append([]Disk(nil), normalized.System.Disks...)
	file.Refresh.System = normalized.Refresh.System.String()
	file.Refresh.Docker = normalized.Refresh.Docker.String()
	file.Refresh.Weather = normalized.Refresh.Weather.String()
	file.Docker.Host = strings.TrimSpace(normalized.Docker.Host)

	raw, err := yaml.Marshal(&file)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("write temp config %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("replace config %q: %w", configPath, err)
	}
	return nil
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

func DiscoverDisks() ([]Disk, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("open /proc/mounts: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	disks, err := discoverDisksFromProcMounts(f)
	if err != nil {
		return nil, err
	}
	for i, disk := range disks {
		disks[i], err = validateDisk(i, disk)
		if err != nil {
			return nil, err
		}
	}
	return disks, nil
}

func discoveredOrDefaultDisks() []Disk {
	disks, err := DiscoverDisks()
	if err != nil || len(disks) == 0 {
		return append([]Disk(nil), Default().System.Disks...)
	}
	return disks
}

func discoverDisksFromProcMounts(r io.Reader) ([]Disk, error) {
	scanner := bufio.NewScanner(r)
	seen := map[string]struct{}{
		"/": {},
	}
	disks := []Disk{
		{Path: "/", Label: "/"},
	}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		mountPath := unescapeProcField(fields[1])
		fsType := fields[2]
		if !shouldAutoIncludeMount(mountPath, fsType) {
			continue
		}
		if _, ok := seen[mountPath]; ok {
			continue
		}
		seen[mountPath] = struct{}{}
		disks = append(disks, Disk{Path: mountPath, Label: autoDetectedDiskLabel(mountPath)})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan /proc/mounts: %w", err)
	}

	sort.Slice(disks[1:], func(i, j int) bool {
		return disks[1+i].Path < disks[1+j].Path
	})
	return disks, nil
}

func autoDetectedDiskLabel(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || path == "/" {
		return "/"
	}
	if base := filepath.Base(path); base != "" && base != "." && base != "/" {
		return base
	}
	return path
}

func shouldAutoIncludeMount(path, fsType string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "/" {
		return true
	}
	if path == "." || path == "" || !filepath.IsAbs(path) {
		return false
	}
	if _, ok := localFilesystemTypes[fsType]; !ok {
		return false
	}
	for _, prefix := range localMountPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func unescapeProcField(value string) string {
	if !strings.ContainsRune(value, '\\') {
		return value
	}

	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+3 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		code, err := strconv.ParseInt(value[i+1:i+4], 8, 32)
		if err != nil {
			b.WriteByte(value[i])
			continue
		}
		b.WriteByte(byte(code))
		i += 3
	}
	return b.String()
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.Refresh.System < 1*time.Second {
		return Config{}, fmt.Errorf("invalid config: refresh.system must be >= 1s")
	}
	if cfg.Refresh.Docker < 3*time.Second {
		return Config{}, fmt.Errorf("invalid config: refresh.docker must be >= 3s")
	}
	if cfg.Refresh.Weather < 1*time.Minute {
		return Config{}, fmt.Errorf("invalid config: refresh.weather must be >= 1m")
	}

	normalized := cfg
	for i, disk := range normalized.System.Disks {
		d, err := validateDisk(i, disk)
		if err != nil {
			return Config{}, err
		}
		normalized.System.Disks[i] = d
	}
	normalized.Theme = strings.TrimSpace(normalized.Theme)
	normalized.Docker.Host = strings.TrimSpace(normalized.Docker.Host)
	return normalized, nil
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
