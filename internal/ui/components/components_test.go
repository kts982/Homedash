package components

import (
	"reflect"
	"regexp"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestNewRingBufferCreatesEmptyBuffer(t *testing.T) {
	rb := NewRingBuffer(3)

	if got := len(rb.Data()); got != 0 {
		t.Fatalf("len(Data()) = %d, want 0", got)
	}
	if got := cap(rb.Data()); got != 3 {
		t.Fatalf("cap(Data()) = %d, want 3", got)
	}
}

func TestRingBufferPushAddsValues(t *testing.T) {
	rb := NewRingBuffer(4)
	rb.Push(1)
	rb.Push(2)
	rb.Push(3)

	want := []float64{1, 2, 3}
	if !reflect.DeepEqual(rb.Data(), want) {
		t.Fatalf("Data() = %v, want %v", rb.Data(), want)
	}
}

func TestRingBufferPushBeyondCapacityEvictsOldest(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(1)
	rb.Push(2)
	rb.Push(3)
	rb.Push(4)
	rb.Push(5)

	want := []float64{3, 4, 5}
	if !reflect.DeepEqual(rb.Data(), want) {
		t.Fatalf("Data() = %v, want %v", rb.Data(), want)
	}
}

func TestRingBufferCapacityBoundary(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(10)
	rb.Push(20)
	rb.Push(30)

	wantAtCapacity := []float64{10, 20, 30}
	if !reflect.DeepEqual(rb.Data(), wantAtCapacity) {
		t.Fatalf("Data() at capacity = %v, want %v", rb.Data(), wantAtCapacity)
	}

	rb.Push(40)
	wantAfterOneMore := []float64{20, 30, 40}
	if !reflect.DeepEqual(rb.Data(), wantAfterOneMore) {
		t.Fatalf("Data() after overflow = %v, want %v", rb.Data(), wantAfterOneMore)
	}
}

func TestSparklineEmptyDataReturnsEmptyString(t *testing.T) {
	if got := Sparkline(nil, 8, lipgloss.Color("2")); got != "" {
		t.Fatalf("Sparkline(nil) = %q, want empty string", got)
	}
}

func TestSparklineReturnsNonEmptyStringForValidData(t *testing.T) {
	got := Sparkline([]float64{20, 40, 60}, 3, lipgloss.Color("2"))
	if got == "" {
		t.Fatalf("Sparkline() returned empty string for non-empty data")
	}
}

func TestSparklineWidthLimitsOutput(t *testing.T) {
	data := []float64{5, 10, 15, 20, 25, 30, 35}
	width := 4

	got := stripANSI(Sparkline(data, width, lipgloss.Color("2")))
	if runes := utf8.RuneCountInString(got); runes != width {
		t.Fatalf("Sparkline rune count = %d, want %d (output %q)", runes, width, got)
	}
}

func TestSparklineClampsValues(t *testing.T) {
	data := []float64{-10, 0, 101, 100}

	got := []rune(stripANSI(Sparkline(data, 4, lipgloss.Color("2"))))
	want := []rune{sparkBlocks[0], sparkBlocks[0], sparkBlocks[len(sparkBlocks)-1], sparkBlocks[len(sparkBlocks)-1]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sparkline clamped output = %q, want %q", string(got), string(want))
	}
}

func TestSparklinePadsWhenDataShorterThanWidth(t *testing.T) {
	data := []float64{50, 100}
	width := 5

	got := []rune(stripANSI(Sparkline(data, width, lipgloss.Color("2"))))
	if len(got) != width {
		t.Fatalf("Sparkline output length = %d, want %d", len(got), width)
	}

	padCount := width - len(data)
	for i := 0; i < padCount; i++ {
		if got[i] != sparkBlocks[0] {
			t.Fatalf("expected padded rune %q at index %d, got %q", string(sparkBlocks[0]), i, string(got[i]))
		}
	}
}
