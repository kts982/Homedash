package styles

import (
	"image/color"
	"testing"
)

func TestApply(t *testing.T) {
	// Save original values
	origPrimary := Primary
	origBgBase := BgBase
	t.Cleanup(func() {
		Apply(TokyoNight)
	})

	// Apply a different palette
	Apply(CatppuccinMocha)

	if Primary != CatppuccinMocha.Primary {
		t.Fatalf("Primary = %q, want %q", Primary, CatppuccinMocha.Primary)
	}
	if BgBase != CatppuccinMocha.BgBase {
		t.Fatalf("BgBase = %q, want %q", BgBase, CatppuccinMocha.BgBase)
	}

	// Verify it actually changed from the original
	if Primary == origPrimary {
		t.Fatal("Primary should have changed from Tokyo Night")
	}
	if BgBase == origBgBase {
		t.Fatal("BgBase should have changed from Tokyo Night")
	}
}

// An unrecognised theme resolves to the default instead of failing. This is
// what keeps a typo in config.yaml from preventing startup.
func TestApplyThemeFallsBackInsteadOfFailing(t *testing.T) {
	t.Cleanup(func() {
		ApplyTheme(DefaultThemeID, true)
	})

	tests := []struct {
		name        string
		wantApplied string
		wantKnown   bool
	}{
		{"tokyo-night", "tokyo-night", true},
		{"catppuccin", "catppuccin", true},
		{"dracula", "dracula", true},
		{"nord", "nord", true},
		{"ember", "ember", true},
		{"mono", "mono", true},
		{"", DefaultThemeID, true},             // empty resolves to the default
		{"  DRACULA  ", "dracula", true},       // case and padding tolerated
		{"nonexistent", DefaultThemeID, false}, // unknown falls back
		{"tokyonight", DefaultThemeID, false},  // near-miss is still a miss
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applied, known := ApplyTheme(tc.name, true)
			if applied != tc.wantApplied {
				t.Errorf("ApplyTheme(%q) applied = %q, want %q", tc.name, applied, tc.wantApplied)
			}
			if known != tc.wantKnown {
				t.Errorf("ApplyTheme(%q) known = %v, want %v", tc.name, known, tc.wantKnown)
			}
			if BgBase == nil || Primary == nil {
				t.Errorf("ApplyTheme(%q) left the palette unpopulated", tc.name)
			}
		})
	}
}

// Every theme must appear in both the map and the cycle order. In the map
// only, it is unreachable from the picker; in the order only, the picker
// offers something that silently resolves to the default.
func TestThemeOrderMatchesRegistry(t *testing.T) {
	ids := ThemeIDs()
	if len(ids) != len(themes) {
		t.Fatalf("ThemeIDs() has %d entries, registry has %d", len(ids), len(themes))
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			t.Errorf("theme %q appears twice in the cycle order", id)
		}
		seen[id] = true
		if _, ok := themes[id]; !ok {
			t.Errorf("theme %q is in the cycle order but not the registry", id)
		}
	}
	for id := range themes {
		if !seen[id] {
			t.Errorf("theme %q is in the registry but not the cycle order", id)
		}
	}
}

// ThemeIDs must hand out a copy; a caller mutating the result would corrupt
// the picker order for the rest of the process.
func TestThemeIDsReturnsCopy(t *testing.T) {
	first := ThemeIDs()
	original := first[0]
	first[0] = "mutated"
	if ThemeIDs()[0] != original {
		t.Fatal("ThemeIDs() exposes the underlying slice; callers can corrupt the cycle order")
	}
}

// Light and dark variants must genuinely differ, or light-terminal support is
// cosmetic only.
func TestEveryThemeHasDistinctLightAndDarkVariants(t *testing.T) {
	for _, id := range ThemeIDs() {
		t.Run(id, func(t *testing.T) {
			th := LookupTheme(id)
			if !hasLightVariant(th.Light) {
				t.Fatalf("theme %q has no light variant", id)
			}
			if th.Dark.BgBase == th.Light.BgBase {
				t.Errorf("theme %q has the same BgBase in both variants", id)
			}
			if th.Dark.TextPrimary == th.Light.TextPrimary {
				t.Errorf("theme %q has the same TextPrimary in both variants", id)
			}
		})
	}
}

func TestApplyThemeSelectsVariantByBackground(t *testing.T) {
	t.Cleanup(func() {
		ApplyTheme(DefaultThemeID, true)
	})

	for _, id := range ThemeIDs() {
		t.Run(id, func(t *testing.T) {
			th := LookupTheme(id)

			ApplyTheme(id, true)
			if BgBase != th.Dark.BgBase {
				t.Errorf("dark background: BgBase = %v, want %v", BgBase, th.Dark.BgBase)
			}

			ApplyTheme(id, false)
			if BgBase != th.Light.BgBase {
				t.Errorf("light background: BgBase = %v, want %v", BgBase, th.Light.BgBase)
			}
		})
	}
}

// ReapplyForBackground keeps the active theme and swaps only the variant.
func TestReapplyForBackgroundKeepsActiveTheme(t *testing.T) {
	t.Cleanup(func() {
		ApplyTheme(DefaultThemeID, true)
	})

	ApplyTheme("nord", true)
	if ActiveTheme() != "nord" {
		t.Fatalf("ActiveTheme() = %q, want nord", ActiveTheme())
	}

	ReapplyForBackground(false)
	if ActiveTheme() != "nord" {
		t.Errorf("ActiveTheme() = %q after reapply, want nord", ActiveTheme())
	}
	if BgBase != LookupTheme("nord").Light.BgBase {
		t.Error("ReapplyForBackground(false) did not switch to the light variant")
	}
}

// Every field of every variant must be set. A nil color reaches lipgloss as
// an unstyled render rather than an error, so a missed field shows up as an
// invisible or mis-coloured widget rather than a crash — cheap to catch here,
// expensive to notice by eye across 12 palettes.
func TestBuiltinPalettesHaveAllFields(t *testing.T) {
	for _, id := range ThemeIDs() {
		th := LookupTheme(id)
		variants := map[string]Palette{
			"dark":  th.Dark,
			"light": th.Light,
		}
		for variant, p := range variants {
			t.Run(id+"/"+variant, func(t *testing.T) {
				if p.Name == "" {
					t.Error("Name is empty")
				}
				fields := map[string]color.Color{
					"BgBase":        p.BgBase,
					"BgPanel":       p.BgPanel,
					"BgFocus":       p.BgFocus,
					"TextPrimary":   p.TextPrimary,
					"TextSecondary": p.TextSecondary,
					"TextMuted":     p.TextMuted,
					"TextInverse":   p.TextInverse,
					"Primary":       p.Primary,
					"Secondary":     p.Secondary,
					"Success":       p.Success,
					"Warning":       p.Warning,
					"Error":         p.Error,
					"Info":          p.Info,
					"Border":        p.Border,
					"BorderFocus":   p.BorderFocus,
				}
				for name, c := range fields {
					if c == nil {
						t.Errorf("%s is nil", name)
					}
				}
			})
		}
	}
}

// The gauge thresholds and container-state colors must stay distinguishable
// in every theme, including Monochrome — collapsing them to one grey would
// remove information, not just decoration.
func TestSemanticColorsRemainDistinct(t *testing.T) {
	t.Cleanup(func() {
		ApplyTheme(DefaultThemeID, true)
	})

	for _, id := range ThemeIDs() {
		for _, dark := range []bool{true, false} {
			variant := "dark"
			if !dark {
				variant = "light"
			}
			t.Run(id+"/"+variant, func(t *testing.T) {
				ApplyTheme(id, dark)

				ok, warn, bad := GaugeColor(10), GaugeColor(80), GaugeColor(95)
				if ok == warn || warn == bad || ok == bad {
					t.Errorf("gauge colors collapse: ok=%v warn=%v critical=%v", ok, warn, bad)
				}

				running := ContainerStateColor("running")
				exited := ContainerStateColor("exited")
				paused := ContainerStateColor("paused")
				if running == exited || running == paused || exited == paused {
					t.Errorf("state colors collapse: running=%v exited=%v paused=%v", running, exited, paused)
				}
			})
		}
	}
}
