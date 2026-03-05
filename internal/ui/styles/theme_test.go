package styles

import (
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

func TestApplyNamed(t *testing.T) {
	t.Cleanup(func() {
		Apply(TokyoNight)
	})

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"tokyo-night", false},
		{"catppuccin", false},
		{"dracula", false},
		{"", false}, // empty defaults to tokyo-night
		{"nonexistent", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ApplyNamed(tc.name)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ApplyNamed(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestBuiltinPalettesHaveAllFields(t *testing.T) {
	palettes := []Palette{TokyoNight, CatppuccinMocha, Dracula}

	for _, p := range palettes {
		t.Run(p.Name, func(t *testing.T) {
			if p.Name == "" {
				t.Fatal("Name is empty")
			}
			if p.BgBase == "" {
				t.Fatal("BgBase is empty")
			}
			if p.Primary == "" {
				t.Fatal("Primary is empty")
			}
			if p.Success == "" {
				t.Fatal("Success is empty")
			}
			if p.Error == "" {
				t.Fatal("Error is empty")
			}
			if p.TextPrimary == "" {
				t.Fatal("TextPrimary is empty")
			}
		})
	}
}
