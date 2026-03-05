package ui

import (
	"testing"
)

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

func TestNotificationQueueEmpty(t *testing.T) {
	var q notificationQueue
	if q.current() != nil {
		t.Fatal("current() should be nil on empty queue")
	}
	if q.len() != 0 {
		t.Fatalf("len = %d, want 0", q.len())
	}
}
