package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var notifANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestNotificationQueuePush(t *testing.T) {
	var q notificationQueue
	q.push("container exited", levelError)

	if q.len() != 1 {
		t.Fatalf("len = %d, want 1", q.len())
	}

	n := q.current()
	if n == nil {
		t.Fatal("current() returned nil")
	}
	if n.Message != "container exited" {
		t.Fatalf("Message = %q, want %q", n.Message, "container exited")
	}
	if n.Level != levelError {
		t.Fatalf("Level = %d, want %d", n.Level, levelError)
	}
	if n.ID == 0 {
		t.Fatal("ID should be non-zero")
	}
	if n.At.IsZero() {
		t.Fatal("At should be set")
	}
}

func TestNotificationQueueDismiss(t *testing.T) {
	var q notificationQueue
	id1 := q.push("first", levelInfo)
	q.push("second", levelWarning)

	// Dismiss first
	q.dismiss(id1)
	if q.len() != 1 {
		t.Fatalf("len = %d, want 1", q.len())
	}
	n := q.current()
	if n.Message != "second" {
		t.Fatalf("Message = %q, want %q", n.Message, "second")
	}

	// Dismiss with wrong ID is a no-op
	q.dismiss(999)
	if q.len() != 1 {
		t.Fatalf("len = %d after bad dismiss, want 1", q.len())
	}
}

func TestNotificationQueueMaxSize(t *testing.T) {
	var q notificationQueue
	for i := 0; i < 25; i++ {
		q.push("msg", levelInfo)
	}
	if q.len() != maxNotifications {
		t.Fatalf("len = %d, want %d", q.len(), maxNotifications)
	}
}

func TestNotificationQueueRecentKeepsDismissedHistory(t *testing.T) {
	var q notificationQueue
	id1 := q.push("first", levelInfo)
	q.push("second", levelWarning)

	q.dismiss(id1)

	recent := q.recent(2)
	if len(recent) != 2 {
		t.Fatalf("len(recent) = %d, want 2", len(recent))
	}
	if recent[0].Message != "second" {
		t.Fatalf("recent[0].Message = %q, want %q", recent[0].Message, "second")
	}
	if recent[1].Message != "first" {
		t.Fatalf("recent[1].Message = %q, want %q", recent[1].Message, "first")
	}
}

func TestNotificationQueueEmpty(t *testing.T) {
	var q notificationQueue
	if q.current() != nil {
		t.Fatal("current() should be nil on empty queue")
	}
	if q.len() != 0 {
		t.Fatalf("len = %d, want 0", q.len())
	}
}

func TestFormatAlertTimestamp(t *testing.T) {
	now := time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

	if got := formatAlertTimestamp(time.Date(2026, 3, 16, 14, 23, 4, 0, time.UTC), now); got != "14:23:04" {
		t.Fatalf("formatAlertTimestamp(same day) = %q, want %q", got, "14:23:04")
	}
	if got := formatAlertTimestamp(time.Date(2026, 3, 15, 9, 5, 6, 0, time.UTC), now); got != "Mar15 09:05:06" {
		t.Fatalf("formatAlertTimestamp(other day) = %q, want %q", got, "Mar15 09:05:06")
	}
}

func TestRenderAlertsPanelShowsTimestampsForRecentEvents(t *testing.T) {
	now := time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)
	view := renderAlertsPanelAt(
		nil,
		[]notification{
			{Message: "web health healthy -> unhealthy", Level: levelError, At: time.Date(2026, 3, 16, 14, 23, 4, 0, time.UTC)},
			{Message: "Disk /data at 91%", Level: levelWarning, At: time.Date(2026, 3, 15, 9, 5, 6, 0, time.UTC)},
		},
		120,
		now,
	)

	for _, want := range []string{"Recent events", "14:23:04", "Mar15 09:05:06", "web health healthy -> unhealthy", "Disk /data at 91%"} {
		if !containsStrippedANSI(view, want) {
			t.Fatalf("renderAlertsPanelAt() = %q, want substring %q", view, want)
		}
	}
}

func containsStrippedANSI(s, want string) bool {
	return strings.Contains(notifANSI.ReplaceAllString(s, ""), want)
}
