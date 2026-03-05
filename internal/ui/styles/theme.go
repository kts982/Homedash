package styles

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Palette defines a complete color theme.
type Palette struct {
	Name          string
	BgBase        lipgloss.Color
	BgPanel       lipgloss.Color
	BgFocus       lipgloss.Color
	TextPrimary   lipgloss.Color
	TextSecondary lipgloss.Color
	TextMuted     lipgloss.Color
	TextInverse   lipgloss.Color
	Primary       lipgloss.Color
	Secondary     lipgloss.Color
	Success       lipgloss.Color
	Warning       lipgloss.Color
	Error         lipgloss.Color
	Info          lipgloss.Color
	Border        lipgloss.Color
	BorderFocus   lipgloss.Color
}

// Built-in palettes.
var TokyoNight = Palette{
	Name:          "tokyo-night",
	BgBase:        "#1a1b26",
	BgPanel:       "#24283b",
	BgFocus:       "#414868",
	TextPrimary:   "#c0caf5",
	TextSecondary: "#a9b1d6",
	TextMuted:     "#565f89",
	TextInverse:   "#1a1b26",
	Primary:       "#7aa2f7",
	Secondary:     "#bb9af7",
	Success:       "#9ece6a",
	Warning:       "#e0af68",
	Error:         "#f7768e",
	Info:          "#7dcfff",
	Border:        "#414868",
	BorderFocus:   "#7aa2f7",
}

var CatppuccinMocha = Palette{
	Name:          "catppuccin",
	BgBase:        "#1e1e2e",
	BgPanel:       "#313244",
	BgFocus:       "#45475a",
	TextPrimary:   "#cdd6f4",
	TextSecondary: "#bac2de",
	TextMuted:     "#6c7086",
	TextInverse:   "#1e1e2e",
	Primary:       "#89b4fa",
	Secondary:     "#cba6f7",
	Success:       "#a6e3a1",
	Warning:       "#f9e2af",
	Error:         "#f38ba8",
	Info:          "#94e2d5",
	Border:        "#45475a",
	BorderFocus:   "#89b4fa",
}

var Dracula = Palette{
	Name:          "dracula",
	BgBase:        "#282a36",
	BgPanel:       "#44475a",
	BgFocus:       "#6272a4",
	TextPrimary:   "#f8f8f2",
	TextSecondary: "#e2e2dc",
	TextMuted:     "#6272a4",
	TextInverse:   "#282a36",
	Primary:       "#bd93f9",
	Secondary:     "#ff79c6",
	Success:       "#50fa7b",
	Warning:       "#ffb86c",
	Error:         "#ff5555",
	Info:          "#8be9fd",
	Border:        "#6272a4",
	BorderFocus:   "#bd93f9",
}

// Active palette colors — referenced by all UI code.
// Initialized with Tokyo Night literal values (NOT referencing TokyoNight var).
var (
	BgBase  lipgloss.Color = "#1a1b26"
	BgPanel lipgloss.Color = "#24283b"
	BgFocus lipgloss.Color = "#414868"

	TextPrimary   lipgloss.Color = "#c0caf5"
	TextSecondary lipgloss.Color = "#a9b1d6"
	TextMuted     lipgloss.Color = "#565f89"
	TextInverse   lipgloss.Color = "#1a1b26"

	Primary   lipgloss.Color = "#7aa2f7"
	Secondary lipgloss.Color = "#bb9af7"
	Success   lipgloss.Color = "#9ece6a"
	Warning   lipgloss.Color = "#e0af68"
	Error     lipgloss.Color = "#f7768e"
	Info      lipgloss.Color = "#7dcfff"

	Border      lipgloss.Color = "#414868"
	BorderFocus lipgloss.Color = "#7aa2f7"
)

// Apply sets the active colors from a palette.
func Apply(p Palette) {
	BgBase = p.BgBase
	BgPanel = p.BgPanel
	BgFocus = p.BgFocus
	TextPrimary = p.TextPrimary
	TextSecondary = p.TextSecondary
	TextMuted = p.TextMuted
	TextInverse = p.TextInverse
	Primary = p.Primary
	Secondary = p.Secondary
	Success = p.Success
	Warning = p.Warning
	Error = p.Error
	Info = p.Info
	Border = p.Border
	BorderFocus = p.BorderFocus
}

// ApplyNamed applies a built-in palette by name.
// Empty string defaults to tokyo-night.
func ApplyNamed(name string) error {
	if name == "" {
		name = "tokyo-night"
	}
	switch name {
	case "tokyo-night":
		Apply(TokyoNight)
	case "catppuccin":
		Apply(CatppuccinMocha)
	case "dracula":
		Apply(Dracula)
	default:
		return fmt.Errorf("unknown theme %q (available: tokyo-night, catppuccin, dracula)", name)
	}
	return nil
}

// GaugeColor returns a color based on usage percentage thresholds.
func GaugeColor(percent float64) lipgloss.Color {
	switch {
	case percent >= 90:
		return Error
	case percent >= 70:
		return Warning
	default:
		return Success
	}
}

// ContainerStateColor returns the semantic color for a Docker container state.
func ContainerStateColor(state string) lipgloss.Color {
	switch state {
	case "running":
		return Success
	case "exited":
		return Error
	case "paused":
		return Warning
	default:
		return TextMuted
	}
}
