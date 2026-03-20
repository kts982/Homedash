package ui

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/config"
	"github.com/kts982/homedash/internal/ui/styles"
)

const (
	settingsThemeField = iota
	settingsDockerHostField
	settingsSystemRefreshField
	settingsDockerRefreshField
	settingsWeatherRefreshField
	settingsDiskFieldStart
)

var settingsThemeOptions = []string{"tokyo-night", "catppuccin", "dracula"}

type settingsDiskRow struct {
	label textinput.Model
	path  textinput.Model
}

type settingsForm struct {
	themeIndex int
	focus      int
	saving     bool
	errorText  string

	dockerHost     textinput.Model
	systemRefresh  textinput.Model
	dockerRefresh  textinput.Model
	weatherRefresh textinput.Model
	disks          []settingsDiskRow
}

func normalizeThemeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "tokyo-night"
	}
	return name
}

func newThemedTextInput(value, placeholder, prompt string, virtualCursor bool) textinput.Model {
	input := textinput.New()
	input.Prompt = prompt
	input.Placeholder = placeholder
	input.SetValue(value)
	input.SetVirtualCursor(virtualCursor)
	applyThemedTextInputStyles(&input)
	return input
}

func applyThemedTextInputStyles(input *textinput.Model) {
	s := textinput.DefaultDarkStyles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	s.Focused.Text = lipgloss.NewStyle().Foreground(styles.TextPrimary)
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(styles.TextSecondary)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(styles.TextPrimary)
	s.Cursor.Color = styles.Primary
	input.SetStyles(s)
}

func newSettingsForm(cfg config.Config, themeName string) settingsForm {
	form := settingsForm{
		dockerHost:     newThemedTextInput(cfg.Docker.Host, "unix:///var/run/docker.sock", "", true),
		systemRefresh:  newThemedTextInput(formatSettingsDuration(cfg.Refresh.System), "2s", "", true),
		dockerRefresh:  newThemedTextInput(formatSettingsDuration(cfg.Refresh.Docker), "5s", "", true),
		weatherRefresh: newThemedTextInput(formatSettingsDuration(cfg.Refresh.Weather), "5m", "", true),
	}

	themeName = normalizeThemeName(themeName)
	for i, option := range settingsThemeOptions {
		if option == themeName {
			form.themeIndex = i
			break
		}
	}

	disks := cfg.System.Disks
	if len(disks) == 0 {
		disks = config.Default().System.Disks
	}
	for _, disk := range disks {
		form.disks = append(form.disks, newSettingsDiskRow(disk))
	}
	if len(form.disks) == 0 {
		form.disks = append(form.disks, newSettingsDiskRow(config.Disk{Path: "/", Label: "/"}))
	}
	_ = form.focusCurrent()
	return form
}

func newSettingsDiskRow(disk config.Disk) settingsDiskRow {
	return settingsDiskRow{
		label: newThemedTextInput(disk.Label, "Label", "", true),
		path:  newThemedTextInput(disk.Path, "/mnt/data", "", true),
	}
}

func (f *settingsForm) applyTheme() {
	applyThemedTextInputStyles(&f.dockerHost)
	applyThemedTextInputStyles(&f.systemRefresh)
	applyThemedTextInputStyles(&f.dockerRefresh)
	applyThemedTextInputStyles(&f.weatherRefresh)
	for i := range f.disks {
		applyThemedTextInputStyles(&f.disks[i].label)
		applyThemedTextInputStyles(&f.disks[i].path)
	}
}

func (f *settingsForm) fieldCount() int {
	return settingsDiskFieldStart + len(f.disks)*2
}

func (f *settingsForm) blurAll() {
	f.dockerHost.Blur()
	f.systemRefresh.Blur()
	f.dockerRefresh.Blur()
	f.weatherRefresh.Blur()
	for i := range f.disks {
		f.disks[i].label.Blur()
		f.disks[i].path.Blur()
	}
}

func (f *settingsForm) focusCurrent() tea.Cmd {
	if f.fieldCount() == 0 {
		f.focus = 0
		return nil
	}
	if f.focus < 0 {
		f.focus = 0
	}
	if f.focus >= f.fieldCount() {
		f.focus = f.fieldCount() - 1
	}

	f.blurAll()
	if input := f.currentInput(); input != nil {
		return input.Focus()
	}
	return nil
}

func (f *settingsForm) moveFocus(delta int) tea.Cmd {
	if f.fieldCount() == 0 {
		return nil
	}
	f.focus += delta
	if f.focus < 0 {
		f.focus = f.fieldCount() - 1
	}
	if f.focus >= f.fieldCount() {
		f.focus = 0
	}
	return f.focusCurrent()
}

func (f *settingsForm) currentInput() *textinput.Model {
	switch f.focus {
	case settingsDockerHostField:
		return &f.dockerHost
	case settingsSystemRefreshField:
		return &f.systemRefresh
	case settingsDockerRefreshField:
		return &f.dockerRefresh
	case settingsWeatherRefreshField:
		return &f.weatherRefresh
	}
	if f.focus < settingsDiskFieldStart {
		return nil
	}
	diskIndex := (f.focus - settingsDiskFieldStart) / 2
	if diskIndex < 0 || diskIndex >= len(f.disks) {
		return nil
	}
	if (f.focus-settingsDiskFieldStart)%2 == 0 {
		return &f.disks[diskIndex].label
	}
	return &f.disks[diskIndex].path
}

func (f *settingsForm) currentDiskIndex() int {
	if f.focus < settingsDiskFieldStart {
		return -1
	}
	index := (f.focus - settingsDiskFieldStart) / 2
	if index < 0 || index >= len(f.disks) {
		return -1
	}
	return index
}

func (f *settingsForm) cycleTheme(delta int) {
	if len(settingsThemeOptions) == 0 {
		return
	}
	f.themeIndex = (f.themeIndex + delta + len(settingsThemeOptions)) % len(settingsThemeOptions)
	f.errorText = ""
}

func (f *settingsForm) addDiskRow() tea.Cmd {
	insertAt := len(f.disks)
	if current := f.currentDiskIndex(); current >= 0 {
		insertAt = current + 1
	}
	row := newSettingsDiskRow(config.Disk{})
	f.disks = append(f.disks, settingsDiskRow{})
	copy(f.disks[insertAt+1:], f.disks[insertAt:])
	f.disks[insertAt] = row
	f.focus = settingsDiskFieldStart + insertAt*2
	f.errorText = ""
	return f.focusCurrent()
}

func (f *settingsForm) removeDiskRow() tea.Cmd {
	if len(f.disks) <= 1 {
		f.errorText = "At least one disk path is required."
		return nil
	}
	index := f.currentDiskIndex()
	if index < 0 {
		index = len(f.disks) - 1
	}
	f.disks = append(f.disks[:index], f.disks[index+1:]...)
	if index >= len(f.disks) {
		index = len(f.disks) - 1
	}
	f.focus = settingsDiskFieldStart + index*2
	f.errorText = ""
	return f.focusCurrent()
}

func (f *settingsForm) detectDisks() tea.Cmd {
	disks, err := config.DiscoverDisks()
	if err != nil {
		f.errorText = err.Error()
		return nil
	}
	if len(disks) == 0 {
		f.errorText = "No mounted disks were detected."
		return nil
	}
	f.disks = f.disks[:0]
	for _, disk := range disks {
		f.disks = append(f.disks, newSettingsDiskRow(disk))
	}
	f.focus = settingsDiskFieldStart
	f.errorText = ""
	return f.focusCurrent()
}

func (f *settingsForm) selectedTheme() string {
	if len(settingsThemeOptions) == 0 {
		return "tokyo-night"
	}
	return settingsThemeOptions[f.themeIndex]
}

func (f *settingsForm) config() (config.Config, error) {
	cfg := config.Config{
		Theme: f.selectedTheme(),
		Refresh: config.RefreshConfig{
			System:  parseSettingsDuration("System refresh", f.systemRefresh.Value(), 1*time.Second),
			Docker:  parseSettingsDuration("Docker refresh", f.dockerRefresh.Value(), 3*time.Second),
			Weather: parseSettingsDuration("Weather refresh", f.weatherRefresh.Value(), 1*time.Minute),
		},
		Docker: config.DockerConfig{
			Host: strings.TrimSpace(f.dockerHost.Value()),
		},
	}

	disks := make([]config.Disk, 0, len(f.disks))
	for i, row := range f.disks {
		path := filepath.Clean(strings.TrimSpace(row.path.Value()))
		if path == "" || path == "." {
			return config.Config{}, fmt.Errorf("disk %d path is required", i+1)
		}
		if !filepath.IsAbs(path) {
			return config.Config{}, fmt.Errorf("disk %d path must be absolute", i+1)
		}
		label := strings.TrimSpace(row.label.Value())
		if label == "" {
			label = path
		}
		disks = append(disks, config.Disk{Path: path, Label: label})
	}
	if len(disks) == 0 {
		return config.Config{}, fmt.Errorf("at least one disk path is required")
	}
	cfg.System.Disks = disks
	return cfg, nil
}

func parseSettingsDuration(label, raw string, minimum time.Duration) time.Duration {
	trimmed := strings.TrimSpace(raw)
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		panic(fmt.Sprintf("%s is invalid: %v", label, err))
	}
	if duration < minimum {
		panic(fmt.Sprintf("%s must be at least %s", label, minimum))
	}
	return duration
}

func formatSettingsDuration(duration time.Duration) string {
	if duration <= 0 {
		return ""
	}
	formatted := duration.String()
	for _, suffix := range []string{"0s", "0m"} {
		if strings.HasSuffix(formatted, suffix) {
			formatted = strings.TrimSuffix(formatted, suffix)
		}
	}
	return formatted
}

func (f *settingsForm) buildConfig() (cfg config.Config, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v", recovered)
		}
	}()
	return f.config()
}

func (f *settingsForm) updateCurrentInput(msg tea.Msg) tea.Cmd {
	input := f.currentInput()
	if input == nil {
		return nil
	}
	updated, cmd := input.Update(msg)
	*input = updated
	f.errorText = ""
	return cmd
}

func (f *settingsForm) setInputWidths(contentWidth int) {
	fieldWidth := max(18, contentWidth-18)
	f.dockerHost.SetWidth(fieldWidth - 2)
	f.systemRefresh.SetWidth(fieldWidth - 2)
	f.dockerRefresh.SetWidth(fieldWidth - 2)
	f.weatherRefresh.SetWidth(fieldWidth - 2)

	labelWidth := min(16, max(12, contentWidth/4))
	pathWidth := max(20, contentWidth-labelWidth-6)
	for i := range f.disks {
		f.disks[i].label.SetWidth(labelWidth - 2)
		f.disks[i].path.SetWidth(pathWidth - 2)
	}
}

func (f *settingsForm) fieldView(label string, content string, focused bool, width int) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Bold(true).
		Width(15)
	if focused {
		labelStyle = labelStyle.Foreground(styles.Primary)
	}
	fieldStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Background(styles.BgBase).
		Padding(0, 1).
		Width(width)
	if focused {
		fieldStyle = fieldStyle.Background(styles.BgFocus)
	}
	return labelStyle.Render(label) + fieldStyle.Render(content)
}

func (f *settingsForm) themeView(width int) string {
	parts := make([]string, 0, len(settingsThemeOptions))
	for i, option := range settingsThemeOptions {
		active := i == f.themeIndex
		previewColor := themePreviewColor(option)
		pill := lipgloss.NewStyle().
			Foreground(previewColor).
			Background(styles.BgBase).
			Padding(0, 1)
		if active {
			pill = pill.
				Foreground(styles.BgBase).
				Background(previewColor).
				Bold(true)
			if f.focus == settingsThemeField {
				pill = pill.Underline(true)
			}
		}
		parts = append(parts, pill.Render(option))
	}
	return f.fieldView("Theme", strings.Join(parts, " "), f.focus == settingsThemeField, width)
}

func themePreviewColor(name string) color.Color {
	switch name {
	case "catppuccin":
		return styles.CatppuccinMocha.Secondary
	case "dracula":
		return styles.Dracula.Secondary
	default:
		return styles.TokyoNight.Secondary
	}
}

func (f *settingsForm) diskRowsView(contentWidth int) string {
	labelWidth := min(16, max(12, contentWidth/4))
	pathWidth := max(20, contentWidth-labelWidth-6)
	indexStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary).Width(3)
	headerStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)

	lines := []string{
		sectionStyle.Render("Disks"),
		indexStyle.Render("") +
			headerStyle.Width(labelWidth+2).Render("Label") + " " +
			headerStyle.Width(pathWidth+2).Render("Path"),
	}
	for i, row := range f.disks {
		labelFocused := f.focus == settingsDiskFieldStart+i*2
		pathFocused := f.focus == settingsDiskFieldStart+i*2+1
		lines = append(lines,
			indexStyle.Render(fmt.Sprintf("%d.", i+1))+
				renderSettingsInput(row.label.View(), labelFocused, labelWidth)+" "+
				renderSettingsInput(row.path.View(), pathFocused, pathWidth),
		)
	}
	return strings.Join(lines, "\n")
}

func renderSettingsInput(content string, focused bool, width int) string {
	style := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Background(styles.BgBase).
		Padding(0, 1).
		Width(width)
	if focused {
		style = style.Background(styles.BgFocus)
	}
	return style.Render(content)
}

func settingsHintView() string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
	hints := []string{
		keyStyle.Render("tab") + " " + textStyle.Render("move"),
		keyStyle.Render("←/→") + " " + textStyle.Render("theme"),
		keyStyle.Render("^n") + " " + textStyle.Render("add"),
		keyStyle.Render("^x") + " " + textStyle.Render("del"),
		keyStyle.Render("^d") + " " + textStyle.Render("detect"),
		keyStyle.Render("enter") + " " + textStyle.Render("save"),
		keyStyle.Render("esc") + " " + textStyle.Render("close"),
	}
	return strings.Join(hints, "  ")
}

func (f *settingsForm) View(width int) string {
	contentWidth := max(52, width-4)
	f.setInputWidths(contentWidth)

	title := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Bold(true).
		Render("Options")
	subtitle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Render("Theme, refresh, Docker host, and monitored disks")

	lines := []string{
		title,
		subtitle,
		"",
		f.themeView(contentWidth - 16),
		f.fieldView("Docker Host", f.dockerHost.View(), f.focus == settingsDockerHostField, contentWidth-16),
		f.fieldView("System", f.systemRefresh.View(), f.focus == settingsSystemRefreshField, contentWidth-16),
		f.fieldView("Docker", f.dockerRefresh.View(), f.focus == settingsDockerRefreshField, contentWidth-16),
		f.fieldView("Weather", f.weatherRefresh.View(), f.focus == settingsWeatherRefreshField, contentWidth-16),
		"",
		f.diskRowsView(contentWidth),
		"",
		settingsHintView(),
	}

	if f.saving {
		lines = append(lines, lipgloss.NewStyle().Foreground(styles.Info).Render("Saving settings..."))
	} else if f.errorText != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(styles.Error).Render(f.errorText))
	}

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.BorderFocus).
		Background(styles.BgPanel).
		Foreground(styles.TextPrimary).
		Width(width).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

func handleSettingsKey(msg tea.KeyPressMsg, m *Model) (tea.Model, tea.Cmd) {
	if m.settingsForm.saving {
		switch msg.String() {
		case "esc":
			m.closeSettings()
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.closeSettings()
		return m, nil
	case "tab", "down":
		return m, m.settingsForm.moveFocus(1)
	case "shift+tab", "up":
		return m, m.settingsForm.moveFocus(-1)
	case "left", "h":
		if m.settingsForm.focus == settingsThemeField {
			m.settingsForm.cycleTheme(-1)
			return m, nil
		}
	case "right", "l":
		if m.settingsForm.focus == settingsThemeField {
			m.settingsForm.cycleTheme(1)
			return m, nil
		}
	case "ctrl+n":
		return m, m.settingsForm.addDiskRow()
	case "ctrl+x":
		return m, m.settingsForm.removeDiskRow()
	case "ctrl+d":
		return m, m.settingsForm.detectDisks()
	case "enter":
		cfg, err := m.settingsForm.buildConfig()
		if err != nil {
			m.settingsForm.errorText = err.Error()
			return m, nil
		}
		m.settingsForm.saving = true
		m.settingsForm.errorText = ""
		return m, saveSettingsCmd(cfg)
	}

	return m, m.settingsForm.updateCurrentInput(msg)
}

func (m Model) renderSettingsOverlay(base string) string {
	popupWidth := max(60, m.width-12)
	popupWidth = min(popupWidth, 96)
	if popupWidth > m.width-2 {
		popupWidth = m.width - 2
	}
	if popupWidth < 40 {
		popupWidth = m.width
	}

	popup := m.settingsForm.View(popupWidth)
	return overlayCenter(base, popup, m.width, m.height, lipgloss.Width(popup), lipgloss.Height(popup))
}
