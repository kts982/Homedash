package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kts982/homedash/internal/ui/components"
	"github.com/kts982/homedash/internal/ui/styles"
)

type notificationLevel int

const (
	levelInfo notificationLevel = iota
	levelWarning
	levelError
)

const maxNotifications = 20
const maxNotificationHistory = 50
const maxAlertPanelLines = 6

type notification struct {
	ID      uint64
	Message string
	Level   notificationLevel
	At      time.Time
}

type notificationQueue struct {
	items   []notification
	history []notification
	nextID  uint64
}

// push adds a notification and returns its ID.
func (q *notificationQueue) push(message string, level notificationLevel) uint64 {
	return q.pushAt(message, level, time.Now())
}

func (q *notificationQueue) pushAt(message string, level notificationLevel, at time.Time) uint64 {
	q.nextID++
	n := notification{
		ID:      q.nextID,
		Message: message,
		Level:   level,
		At:      at,
	}
	q.items = append(q.items, n)
	q.history = append(q.history, n)
	// Drop oldest if over max
	if len(q.items) > maxNotifications {
		q.items = q.items[len(q.items)-maxNotifications:]
	}
	if len(q.history) > maxNotificationHistory {
		q.history = q.history[len(q.history)-maxNotificationHistory:]
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

// recent returns up to limit notifications from history, newest first.
func (q *notificationQueue) recent(limit int) []notification {
	if limit <= 0 || len(q.history) == 0 {
		return nil
	}

	if limit > len(q.history) {
		limit = len(q.history)
	}

	recent := make([]notification, 0, limit)
	for i := len(q.history) - 1; i >= 0 && len(recent) < limit; i-- {
		recent = append(recent, q.history[i])
	}
	return recent
}

// renderNotificationBar renders the current notification as a single styled line.
func renderNotificationBar(q *notificationQueue, width int) string {
	n := q.current()
	if n == nil {
		return ""
	}

	icon, color := notificationChrome(n.Level)
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

func renderAlertsPanel(active, recent []notification, width int) string {
	return renderAlertsPanelAt(active, recent, width, time.Now())
}

func renderAlertsPanelAt(active, recent []notification, width int, now time.Time) string {
	innerWidth := width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}

	labelStyle := lipgloss.NewStyle().Foreground(styles.TextMuted).Bold(true)
	lines := make([]string, 0, maxAlertPanelLines)

	if len(active) == 0 && len(recent) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(styles.TextMuted).Render("No active problems or recent events"))
	} else {
		if len(active) > 0 {
			lines = append(lines, labelStyle.Render("Active problems"))
			for _, alert := range active {
				if len(lines) >= maxAlertPanelLines {
					break
				}
				lines = append(lines, renderAlertLine(alert, innerWidth, false, now))
			}
		}
		if len(recent) > 0 && len(lines) < maxAlertPanelLines {
			lines = append(lines, labelStyle.Render("Recent events"))
			for _, alert := range recent {
				if len(lines) >= maxAlertPanelLines {
					break
				}
				lines = append(lines, renderAlertLine(alert, innerWidth, true, now))
			}
		}
	}

	content := strings.Join(lines, "\n")
	title := "ALERTS"
	var summary []string
	if len(active) > 0 {
		summary = append(summary, fmt.Sprintf("%d active", len(active)))
	}
	if len(recent) > 0 {
		summary = append(summary, fmt.Sprintf("%d recent", len(recent)))
	}
	if len(summary) > 0 {
		title += " · " + strings.Join(summary, " / ")
	}
	return components.Panel(title, content, width, len(lines)+3, false)
}

func renderAlertLine(n notification, width int, showTimestamp bool, now time.Time) string {
	icon, color := notificationChrome(n.Level)
	prefix := lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(icon + " ")

	prefixWidth := lipgloss.Width(icon) + 1
	if showTimestamp && !n.At.IsZero() {
		timestamp := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Render(formatAlertTimestamp(n.At, now) + " ")
		prefix += timestamp
		prefixWidth += lipgloss.Width(formatAlertTimestamp(n.At, now)) + 1
	}
	message := lipgloss.NewStyle().
		Inline(true).
		MaxWidth(max(1, width-prefixWidth)).
		Foreground(styles.TextPrimary).
		Render(n.Message)
	return prefix + message
}

func formatAlertTimestamp(at, now time.Time) string {
	at = at.Local()
	now = now.Local()
	if sameLocalDay(at, now) {
		return at.Format("15:04:05")
	}
	return at.Format("Jan02 15:04:05")
}

func sameLocalDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func notificationChrome(level notificationLevel) (string, color.Color) {
	switch level {
	case levelError:
		return "✕", styles.Error
	case levelWarning:
		return "!", styles.Warning
	default:
		return "i", styles.Info
	}
}
