package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kts982/homedash/internal/collector"
	"github.com/kts982/homedash/internal/config"
	"github.com/kts982/homedash/internal/ui"
	"github.com/kts982/homedash/internal/ui/styles"
)

func main() {
	testMode := flag.Bool("test-mode", false, "Enable deterministic test mode (disables live refresh)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Assume a dark terminal until the real background is reported via
	// tea.BackgroundColorMsg, which arrives through Update shortly after
	// start. An unrecognised theme is a warning, never a startup failure.
	applied, known := styles.ApplyTheme(cfg.Theme, true)
	if !known {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"config: theme %q is not recognised — using %s (available: %s)",
			cfg.Theme, applied, strings.Join(styles.ThemeIDs(), ", ")))
	}

	dockerHost := cfg.EffectiveDockerHost()
	collector.SetDockerHost(dockerHost)

	p := tea.NewProgram(
		ui.NewModel(ui.ModelOptions{
			Theme:                  applied,
			Disks:                  cfg.System.Disks,
			DockerHost:             dockerHost,
			SystemRefreshInterval:  cfg.Refresh.System,
			DockerRefreshInterval:  cfg.Refresh.Docker,
			WeatherRefreshInterval: cfg.Refresh.Weather,
			LogOrder:               cfg.Logs.Order,
			ConfigWarnings:         cfg.Warnings,
			TestMode:               *testMode,
		}),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
