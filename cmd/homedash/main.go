package main

import (
	"flag"
	"fmt"
	"os"

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

	if err := styles.ApplyNamed(cfg.Theme); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dockerHost := cfg.EffectiveDockerHost()
	collector.SetDockerHost(dockerHost)

	p := tea.NewProgram(
		ui.NewModel(ui.ModelOptions{
			Theme:                  cfg.Theme,
			Disks:                  cfg.System.Disks,
			DockerHost:             dockerHost,
			SystemRefreshInterval:  cfg.Refresh.System,
			DockerRefreshInterval:  cfg.Refresh.Docker,
			WeatherRefreshInterval: cfg.Refresh.Weather,
			TestMode:               *testMode,
		}),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
