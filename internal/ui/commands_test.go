package ui

import (
	"reflect"
	"testing"

	"github.com/kostas/homedash/internal/collector"
)

func TestStackActionTargets(t *testing.T) {
	containers := []collector.Container{
		{ID: "run-1", Name: "web", Stack: "media", State: "running"},
		{ID: "stop-1", Name: "worker", Stack: "media", State: "exited"},
		{ID: "run-2", Name: "db", Stack: "media", State: "running"},
		{ID: "other", Name: "proxy", Stack: "edge", State: "running"},
	}

	tests := []struct {
		name   string
		action string
		want   []stackActionTarget
	}{
		{
			name:   "start targets stopped containers in stack",
			action: "start",
			want: []stackActionTarget{
				{ID: "stop-1", Name: "worker"},
			},
		},
		{
			name:   "stop targets running containers in stack",
			action: "stop",
			want: []stackActionTarget{
				{ID: "run-1", Name: "web"},
				{ID: "run-2", Name: "db"},
			},
		},
		{
			name:   "restart targets running containers in stack",
			action: "restart",
			want: []stackActionTarget{
				{ID: "run-1", Name: "web"},
				{ID: "run-2", Name: "db"},
			},
		},
		{
			name:   "unknown action returns no targets",
			action: "noop",
			want:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stackActionTargets(containers, "media", tc.action)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("stackActionTargets() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
