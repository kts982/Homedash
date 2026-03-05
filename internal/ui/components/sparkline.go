package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kostas/homedash/internal/ui/styles"
)

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders a mini chart from a slice of values (0-100 range).
// width controls how many data points are shown (most recent).
func Sparkline(data []float64, width int, color lipgloss.Color) string {
	if len(data) == 0 {
		return ""
	}

	// Take the most recent `width` data points
	start := 0
	if len(data) > width {
		start = len(data) - width
	}
	visible := data[start:]

	style := lipgloss.NewStyle().Foreground(color)
	dimStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)

	out := ""
	for _, v := range visible {
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		idx := int(v / 100 * float64(len(sparkBlocks)-1))
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		if v < 5 {
			out += dimStyle.Render(string(sparkBlocks[0]))
		} else {
			out += style.Render(string(sparkBlocks[idx]))
		}
	}

	// Pad with dim blocks if not enough data
	for i := len(visible); i < width; i++ {
		out = dimStyle.Render(string(sparkBlocks[0])) + out
	}

	return out
}

// RingBuffer is a fixed-size circular buffer for sparkline history.
type RingBuffer struct {
	data []float64
	size int
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		data: make([]float64, 0, size),
		size: size,
	}
}

func (r *RingBuffer) Push(v float64) {
	if len(r.data) >= r.size {
		r.data = r.data[1:]
	}
	r.data = append(r.data, v)
}

func (r *RingBuffer) Data() []float64 {
	return r.data
}
