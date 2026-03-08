package styles

import (
	"fmt"
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette defines a complete color theme.
type Palette struct {
	Name          string
	BgBase        color.Color
	BgPanel       color.Color
	BgFocus       color.Color
	TextPrimary   color.Color
	TextSecondary color.Color
	TextMuted     color.Color
	TextInverse   color.Color
	Primary       color.Color
	Secondary     color.Color
	Success       color.Color
	Warning       color.Color
	Error         color.Color
	Info          color.Color
	Border        color.Color
	BorderFocus   color.Color
}

// Built-in palettes.
var TokyoNight = Palette{
	Name:          "tokyo-night",
	BgBase:        lipgloss.Color("#1a1b26"),
	BgPanel:       lipgloss.Color("#24283b"),
	BgFocus:       lipgloss.Color("#414868"),
	TextPrimary:   lipgloss.Color("#c0caf5"),
	TextSecondary: lipgloss.Color("#a9b1d6"),
	TextMuted:     lipgloss.Color("#565f89"),
	TextInverse:   lipgloss.Color("#1a1b26"),
	Primary:       lipgloss.Color("#7aa2f7"),
	Secondary:     lipgloss.Color("#bb9af7"),
	Success:       lipgloss.Color("#9ece6a"),
	Warning:       lipgloss.Color("#e0af68"),
	Error:         lipgloss.Color("#f7768e"),
	Info:          lipgloss.Color("#7dcfff"),
	Border:        lipgloss.Color("#414868"),
	BorderFocus:   lipgloss.Color("#7aa2f7"),
}

var CatppuccinMocha = Palette{
	Name:          "catppuccin",
	BgBase:        lipgloss.Color("#1e1e2e"),
	BgPanel:       lipgloss.Color("#313244"),
	BgFocus:       lipgloss.Color("#45475a"),
	TextPrimary:   lipgloss.Color("#cdd6f4"),
	TextSecondary: lipgloss.Color("#bac2de"),
	TextMuted:     lipgloss.Color("#6c7086"),
	TextInverse:   lipgloss.Color("#1e1e2e"),
	Primary:       lipgloss.Color("#89b4fa"),
	Secondary:     lipgloss.Color("#cba6f7"),
	Success:       lipgloss.Color("#a6e3a1"),
	Warning:       lipgloss.Color("#f9e2af"),
	Error:         lipgloss.Color("#f38ba8"),
	Info:          lipgloss.Color("#94e2d5"),
	Border:        lipgloss.Color("#45475a"),
	BorderFocus:   lipgloss.Color("#89b4fa"),
}

var Dracula = Palette{
	Name:          "dracula",
	BgBase:        lipgloss.Color("#282a36"),
	BgPanel:       lipgloss.Color("#44475a"),
	BgFocus:       lipgloss.Color("#6272a4"),
	TextPrimary:   lipgloss.Color("#f8f8f2"),
	TextSecondary: lipgloss.Color("#e2e2dc"),
	TextMuted:     lipgloss.Color("#6272a4"),
	TextInverse:   lipgloss.Color("#282a36"),
	Primary:       lipgloss.Color("#bd93f9"),
	Secondary:     lipgloss.Color("#ff79c6"),
	Success:       lipgloss.Color("#50fa7b"),
	Warning:       lipgloss.Color("#ffb86c"),
	Error:         lipgloss.Color("#ff5555"),
	Info:          lipgloss.Color("#8be9fd"),
	Border:        lipgloss.Color("#6272a4"),
	BorderFocus:   lipgloss.Color("#bd93f9"),
}

// Active palette colors — referenced by all UI code.
var (
	BgBase  color.Color = lipgloss.Color("#1a1b26")
	BgPanel color.Color = lipgloss.Color("#24283b")
	BgFocus color.Color = lipgloss.Color("#414868")

	TextPrimary   color.Color = lipgloss.Color("#c0caf5")
	TextSecondary color.Color = lipgloss.Color("#a9b1d6")
	TextMuted     color.Color = lipgloss.Color("#565f89")
	TextInverse   color.Color = lipgloss.Color("#1a1b26")

	Primary   color.Color = lipgloss.Color("#7aa2f7")
	Secondary color.Color = lipgloss.Color("#bb9af7")
	Success   color.Color = lipgloss.Color("#9ece6a")
	Warning   color.Color = lipgloss.Color("#e0af68")
	Error     color.Color = lipgloss.Color("#f7768e")
	Info      color.Color = lipgloss.Color("#7dcfff")

	Border      color.Color = lipgloss.Color("#414868")
	BorderFocus color.Color = lipgloss.Color("#7aa2f7")
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
func GaugeColor(percent float64) color.Color {
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
func ContainerStateColor(state string) color.Color {
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
