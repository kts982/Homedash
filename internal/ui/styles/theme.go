package styles

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// ── Theme architecture ─────────────────────────────────────────────
//
// A Palette holds every semantic color the UI uses. ApplyTheme picks the
// dark or light variant of a named Theme based on the terminal background,
// rebinds the package-level color vars below, and calls rebuildDerivedStyles.
//
// The ~200 inline call sites across the UI read these vars every frame
// (lipgloss.NewStyle().Foreground(styles.Primary)…) and so pick up changes
// automatically. Anything that *caches* a style or a pre-rendered string must
// instead be re-derived in rebuildDerivedStyles, or it will silently keep
// rendering in the previous theme's colors.
//
// Styles held inside Bubbles models (text inputs, viewports) are not package
// state and cannot be reached from here at all — the UI layer re-applies
// those itself when the theme changes. See ui.applyThemedTextInputStyles.
//
// Palette roles:
//
//	BgBase / BgPanel / BgFocus  — surface tiers, back to front
//	TextPrimary/Secondary/Muted — text tiers, most to least prominent
//	TextInverse                 — text drawn on top of a Primary fill
//	Primary                     — focus, selection, structure
//	Secondary                   — accents and section headings
//	Success / Warning / Error   — state semantics; also gauge thresholds
//	Info                        — neutral emphasis, distinct from Primary
//	Border / BorderFocus        — panel edges, unfocused and focused

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

// Theme bundles the light and dark variants of a named palette. Light may be
// the zero value, in which case Dark is used regardless of terminal
// background.
type Theme struct {
	ID    string
	Label string
	Dark  Palette
	Light Palette
}

// DefaultThemeID is used when no theme is configured, and as the fallback for
// an unrecognised one.
const DefaultThemeID = "tokyo-night"

// ── Tokyo Night ────────────────────────────────────────────────────
// Dark values from the Tokyo Night spec; light from its Day variant.

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

var tokyoNightDay = Palette{
	Name:          "tokyo-night",
	BgBase:        lipgloss.Color("#e1e2e7"),
	BgPanel:       lipgloss.Color("#d0d5e3"),
	BgFocus:       lipgloss.Color("#b7c1e3"),
	TextPrimary:   lipgloss.Color("#3760bf"),
	TextSecondary: lipgloss.Color("#6172b0"),
	TextMuted:     lipgloss.Color("#848cb5"),
	TextInverse:   lipgloss.Color("#e1e2e7"),
	Primary:       lipgloss.Color("#2e7de9"),
	Secondary:     lipgloss.Color("#9854f1"),
	Success:       lipgloss.Color("#587539"),
	Warning:       lipgloss.Color("#8c6c3e"),
	Error:         lipgloss.Color("#f52a65"),
	Info:          lipgloss.Color("#007197"),
	Border:        lipgloss.Color("#b7c1e3"),
	BorderFocus:   lipgloss.Color("#2e7de9"),
}

// ── Catppuccin ─────────────────────────────────────────────────────
// Mocha and Latte, from the published Catppuccin spec.

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

var catppuccinLatte = Palette{
	Name:          "catppuccin",
	BgBase:        lipgloss.Color("#eff1f5"),
	BgPanel:       lipgloss.Color("#e6e9ef"),
	BgFocus:       lipgloss.Color("#ccd0da"),
	TextPrimary:   lipgloss.Color("#4c4f69"),
	TextSecondary: lipgloss.Color("#5c5f77"),
	TextMuted:     lipgloss.Color("#8c8fa1"),
	TextInverse:   lipgloss.Color("#eff1f5"),
	Primary:       lipgloss.Color("#1e66f5"),
	Secondary:     lipgloss.Color("#8839ef"),
	Success:       lipgloss.Color("#40a02b"),
	Warning:       lipgloss.Color("#df8e1d"),
	Error:         lipgloss.Color("#d20f39"),
	Info:          lipgloss.Color("#179299"),
	Border:        lipgloss.Color("#ccd0da"),
	BorderFocus:   lipgloss.Color("#1e66f5"),
}

// ── Dracula ────────────────────────────────────────────────────────
// Dark from the published Dracula spec. Dracula has no official light
// variant, so the light palette is a contrast-matched companion using the
// same hue family, darkened enough to read on a pale surface.

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

var draculaLight = Palette{
	Name:          "dracula",
	BgBase:        lipgloss.Color("#f8f8f2"),
	BgPanel:       lipgloss.Color("#e8e8e0"),
	BgFocus:       lipgloss.Color("#d2d4e0"),
	TextPrimary:   lipgloss.Color("#282a36"),
	TextSecondary: lipgloss.Color("#44475a"),
	TextMuted:     lipgloss.Color("#6272a4"),
	TextInverse:   lipgloss.Color("#f8f8f2"),
	Primary:       lipgloss.Color("#6d42b2"),
	Secondary:     lipgloss.Color("#c2187a"),
	Success:       lipgloss.Color("#168a35"),
	Warning:       lipgloss.Color("#b55f00"),
	Error:         lipgloss.Color("#c92c2c"),
	Info:          lipgloss.Color("#007c91"),
	Border:        lipgloss.Color("#d2d4e0"),
	BorderFocus:   lipgloss.Color("#6d42b2"),
}

// ── Nord ───────────────────────────────────────────────────────────
// Polar Night / Snow Storm / Frost / Aurora, from the Nord spec. The light
// variant inverts the surface tiers and darkens the Aurora accents.

var nord = Palette{
	Name:          "nord",
	BgBase:        lipgloss.Color("#2e3440"),
	BgPanel:       lipgloss.Color("#3b4252"),
	BgFocus:       lipgloss.Color("#434c5e"),
	TextPrimary:   lipgloss.Color("#eceff4"),
	TextSecondary: lipgloss.Color("#d8dee9"),
	TextMuted:     lipgloss.Color("#7b88a1"),
	TextInverse:   lipgloss.Color("#2e3440"),
	Primary:       lipgloss.Color("#88c0d0"),
	Secondary:     lipgloss.Color("#b48ead"),
	Success:       lipgloss.Color("#a3be8c"),
	Warning:       lipgloss.Color("#ebcb8b"),
	Error:         lipgloss.Color("#bf616a"),
	Info:          lipgloss.Color("#8fbcbb"),
	Border:        lipgloss.Color("#4c566a"),
	BorderFocus:   lipgloss.Color("#88c0d0"),
}

var nordLight = Palette{
	Name:          "nord",
	BgBase:        lipgloss.Color("#eceff4"),
	BgPanel:       lipgloss.Color("#e5e9f0"),
	BgFocus:       lipgloss.Color("#d8dee9"),
	TextPrimary:   lipgloss.Color("#2e3440"),
	TextSecondary: lipgloss.Color("#434c5e"),
	TextMuted:     lipgloss.Color("#7b88a1"),
	TextInverse:   lipgloss.Color("#eceff4"),
	Primary:       lipgloss.Color("#5e81ac"),
	Secondary:     lipgloss.Color("#8a5d80"),
	Success:       lipgloss.Color("#5f7f45"),
	Warning:       lipgloss.Color("#9a6f20"),
	Error:         lipgloss.Color("#a9444f"),
	Info:          lipgloss.Color("#4f8f8b"),
	Border:        lipgloss.Color("#c2ccd9"),
	BorderFocus:   lipgloss.Color("#5e81ac"),
}

// ── Ember ──────────────────────────────────────────────────────────
// Warm earth tones — the lane the other palettes all lean away from, since
// Tokyo Night, Catppuccin, Dracula and Nord are cool or magenta-leaning and
// Mono is achromatic. Success stays sage rather than cool green so it holds
// against the structural amber.

var emberDusk = Palette{
	Name:          "ember",
	BgBase:        lipgloss.Color("#1a1410"),
	BgPanel:       lipgloss.Color("#2a211a"),
	BgFocus:       lipgloss.Color("#3d2f24"),
	TextPrimary:   lipgloss.Color("#f5e6d3"),
	TextSecondary: lipgloss.Color("#d4bda4"),
	TextMuted:     lipgloss.Color("#8a7460"),
	TextInverse:   lipgloss.Color("#1a1410"),
	Primary:       lipgloss.Color("#e89a4f"),
	Secondary:     lipgloss.Color("#c4956c"),
	Success:       lipgloss.Color("#88a86b"),
	Warning:       lipgloss.Color("#e8b86e"),
	Error:         lipgloss.Color("#d9603f"),
	Info:          lipgloss.Color("#8fa66e"),
	Border:        lipgloss.Color("#3d2f24"),
	BorderFocus:   lipgloss.Color("#e89a4f"),
}

var emberDawn = Palette{
	Name:          "ember",
	BgBase:        lipgloss.Color("#f4ead8"),
	BgPanel:       lipgloss.Color("#e9dcc5"),
	BgFocus:       lipgloss.Color("#d8c7ab"),
	TextPrimary:   lipgloss.Color("#2c1f17"),
	TextSecondary: lipgloss.Color("#5a4536"),
	TextMuted:     lipgloss.Color("#8a7460"),
	TextInverse:   lipgloss.Color("#f4ead8"),
	Primary:       lipgloss.Color("#a85419"),
	Secondary:     lipgloss.Color("#8a6a3f"),
	Success:       lipgloss.Color("#4f6b3f"),
	Warning:       lipgloss.Color("#9a6418"),
	Error:         lipgloss.Color("#983827"),
	Info:          lipgloss.Color("#4f6b3f"),
	Border:        lipgloss.Color("#d8c7ab"),
	BorderFocus:   lipgloss.Color("#a85419"),
}

// ── Monochrome ─────────────────────────────────────────────────────
// High-contrast greyscale for accessibility. Success, Warning and Error keep
// a small hue cue — pure greyscale would erase the gauge thresholds and the
// container-state distinction entirely, which is a correctness loss, not a
// stylistic one.

var monoDark = Palette{
	Name:          "mono",
	BgBase:        lipgloss.Color("#000000"),
	BgPanel:       lipgloss.Color("#141414"),
	BgFocus:       lipgloss.Color("#333333"),
	TextPrimary:   lipgloss.Color("#ffffff"),
	TextSecondary: lipgloss.Color("#cccccc"),
	TextMuted:     lipgloss.Color("#8c8c8c"),
	TextInverse:   lipgloss.Color("#000000"),
	Primary:       lipgloss.Color("#ffffff"),
	Secondary:     lipgloss.Color("#cccccc"),
	Success:       lipgloss.Color("#a3be8c"),
	Warning:       lipgloss.Color("#ebcb8b"),
	Error:         lipgloss.Color("#bf616a"),
	Info:          lipgloss.Color("#cccccc"),
	Border:        lipgloss.Color("#666666"),
	BorderFocus:   lipgloss.Color("#ffffff"),
}

var monoLight = Palette{
	Name:          "mono",
	BgBase:        lipgloss.Color("#ffffff"),
	BgPanel:       lipgloss.Color("#f0f0f0"),
	BgFocus:       lipgloss.Color("#d6d6d6"),
	TextPrimary:   lipgloss.Color("#000000"),
	TextSecondary: lipgloss.Color("#333333"),
	TextMuted:     lipgloss.Color("#707070"),
	TextInverse:   lipgloss.Color("#ffffff"),
	Primary:       lipgloss.Color("#000000"),
	Secondary:     lipgloss.Color("#333333"),
	Success:       lipgloss.Color("#28702b"),
	Warning:       lipgloss.Color("#a06914"),
	Error:         lipgloss.Color("#a91527"),
	Info:          lipgloss.Color("#333333"),
	Border:        lipgloss.Color("#b0b0b0"),
	BorderFocus:   lipgloss.Color("#000000"),
}

// ── Registry ───────────────────────────────────────────────────────

var themes = map[string]Theme{
	"tokyo-night": {ID: "tokyo-night", Label: "Tokyo Night", Dark: TokyoNight, Light: tokyoNightDay},
	"catppuccin":  {ID: "catppuccin", Label: "Catppuccin", Dark: CatppuccinMocha, Light: catppuccinLatte},
	"dracula":     {ID: "dracula", Label: "Dracula", Dark: Dracula, Light: draculaLight},
	"nord":        {ID: "nord", Label: "Nord", Dark: nord, Light: nordLight},
	"ember":       {ID: "ember", Label: "Ember", Dark: emberDusk, Light: emberDawn},
	"mono":        {ID: "mono", Label: "Monochrome", Dark: monoDark, Light: monoLight},
}

// themeOrder is the user-facing cycle order. Kept separate from the map
// because Go map iteration is randomised and the settings picker needs a
// deterministic next/prev.
var themeOrder = []string{
	"tokyo-night",
	"catppuccin",
	"dracula",
	"nord",
	"ember",
	"mono",
}

// ThemeIDs returns the theme IDs in cycle order.
func ThemeIDs() []string {
	return append([]string(nil), themeOrder...)
}

// IsKnownTheme reports whether id names a built-in theme.
func IsKnownTheme(id string) bool {
	_, ok := themes[normalizeID(id)]
	return ok
}

// ThemeLabel returns the display label for id, or the default theme's label
// if id is unknown.
func ThemeLabel(id string) string {
	return LookupTheme(id).Label
}

// LookupTheme returns the named theme, falling back to the default when the
// name is unknown. It deliberately does not return an error: a hand-edited
// config with a typo must not be able to prevent startup.
func LookupTheme(id string) Theme {
	if t, ok := themes[normalizeID(id)]; ok {
		return t
	}
	return themes[DefaultThemeID]
}

func normalizeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return DefaultThemeID
	}
	return id
}

// hasLightVariant reports whether p was populated. The zero value means "no
// light variant, use Dark for both"; BgBase is probed because every real
// palette sets it.
func hasLightVariant(p Palette) bool {
	return p.BgBase != nil
}

// ── Active palette + bound color vars ──────────────────────────────

// Active palette colors — referenced by all UI code. Rebound by Apply.
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

// activeTheme records which theme ApplyTheme last resolved, so the same theme
// can be re-applied against a newly detected background. The background itself
// is not stored here — the UI model owns that and passes it in.
var activeTheme = DefaultThemeID

// ActiveTheme returns the ID of the currently applied theme.
func ActiveTheme() string { return activeTheme }

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

	rebuildDerivedStyles()
}

// ApplyTheme resolves id against the registry, selects the variant matching
// the terminal background, and applies it. It reports the ID actually applied
// and whether id was recognised — an unknown id is applied as the default
// rather than rejected, so callers can warn without failing.
func ApplyTheme(id string, darkBackground bool) (applied string, known bool) {
	known = IsKnownTheme(id)
	t := LookupTheme(id)

	p := t.Dark
	if !darkBackground && hasLightVariant(t.Light) {
		p = t.Light
	}
	Apply(p)

	activeTheme = t.ID
	return t.ID, known
}

// ReapplyForBackground re-resolves the active theme against a newly detected
// terminal background. Called when tea.BackgroundColorMsg arrives.
func ReapplyForBackground(darkBackground bool) {
	ApplyTheme(activeTheme, darkBackground)
}

// rebuildDerivedStyles re-derives any package-level cached style or
// pre-rendered string from the current color vars.
//
// It is intentionally empty: nothing in this package caches a style today,
// and every consumer reads the color vars inline each frame. The hook exists
// so that invariant is enforced rather than merely true by luck — anything
// added here as `var x = lipgloss.NewStyle().Foreground(Primary)` captures
// the color at init and would silently render stale after a theme change.
// Assign such values here instead.
func rebuildDerivedStyles() {}

// ── Semantic helpers ───────────────────────────────────────────────

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
