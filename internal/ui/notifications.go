package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/kostas/homedash/internal/ui/styles"
)

type notificationLevel int

const (
	levelInfo notificationLevel = iota
	levelWarning
	levelError
)

const maxNotifications = 20

type notification struct {
	ID      uint64
	Message string
	Level   notificationLevel
}

type notificationQueue struct {
	items  []notification
	nextID uint64
}

// push adds a notification and returns its ID.
func (q *notificationQueue) push(message string, level notificationLevel) uint64 {
	q.nextID++
	q.items = append(q.items, notification{
		ID:      q.nextID,
		Message: message,
		Level:   level,
	})
	// Drop oldest if over max
	if len(q.items) > maxNotifications {
		q.items = q.items[len(q.items)-maxNotifications:]
	}
	return q.nextID
}

// dismiss removes the notification with the given ID.
func (q *notificationQueue) dismiss(id uint64) {
	for i, n := range q.items {
		if n.ID == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return
		}
	}
}

// current returns the head notification, or nil if empty.
func (q *notificationQueue) current() *notification {
	if len(q.items) == 0 {
		return nil
	}
	return &q.items[0]
}

// len returns the number of queued notifications.
func (q *notificationQueue) len() int {
	return len(q.items)
}

// renderNotificationBar renders the current notification as a single styled line.
func renderNotificationBar(q *notificationQueue, width int) string {
	n := q.current()
	if n == nil {
		return ""
	}

	var icon string
	var color color.Color
	switch n.Level {
	case levelError:
		icon = "  ✕ "
		color = styles.Error
	case levelWarning:
		icon = "  ! "
		color = styles.Warning
	default:
		icon = "  i "
		color = styles.Info
	}

	iconStyled := lipgloss.NewStyle().
		Foreground(styles.TextInverse).
		Background(color).
		Bold(true).
		Render(icon)

	msgStyled := lipgloss.NewStyle().
		Foreground(color).
		Render(" " + n.Message)

	content := iconStyled + msgStyled

	return lipgloss.NewStyle().
		Background(styles.BgBase).
		Width(width).
		Render(content)
}
