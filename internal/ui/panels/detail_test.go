package panels

import (
	"strings"
	"testing"

	"github.com/kostas/homedash/internal/collector"
)

func TestRenderDetailAcceptsShortContainerID(t *testing.T) {
	c := &collector.Container{
		ID:    "abc",
		Name:  "svc",
		Image: "svc:latest",
		State: "running",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RenderDetail() panicked for short container ID: %v", r)
		}
	}()

	view := RenderDetail(c, nil, []string{"log line"}, nil, "", "", 0, 80, 20, false)
	if !strings.Contains(view, "abc") {
		t.Fatalf("RenderDetail() output does not contain short container ID: %q", view)
	}
}
