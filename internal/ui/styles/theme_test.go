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
			if p.BgBase == nil {
				t.Fatal("BgBase is nil")
			}
			if p.Primary == nil {
				t.Fatal("Primary is nil")
			}
			if p.Success == nil {
				t.Fatal("Success is nil")
			}
			if p.Error == nil {
				t.Fatal("Error is nil")
			}
			if p.TextPrimary == nil {
				t.Fatal("TextPrimary is nil")
			}
		})
	}
}
