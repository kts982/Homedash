package ui

import "github.com/kostas/homedash/internal/collector"

// SystemDataMsg carries updated system metrics.
type SystemDataMsg struct {
	Data collector.SystemData
	Err  error
}

// DockerDataMsg carries updated Docker container data.
type DockerDataMsg struct {
	Data collector.DockerData
	Err  error
}

// WeatherDataMsg carries updated weather data.
type WeatherDataMsg struct {
	Data collector.WeatherData
	Err  error
}

// SystemTickMsg is sent by the periodic timer to trigger system collection.
type SystemTickMsg struct{}

// DockerTickMsg is sent by the periodic timer to trigger Docker collection.
type DockerTickMsg struct{}

// WeatherTickMsg is sent by the periodic timer to trigger weather collection.
type WeatherTickMsg struct{}

// ForceRefreshMsg triggers an immediate refresh of all data.
type ForceRefreshMsg struct{}

// ContainerLogsMsg carries fetched container logs.
type ContainerLogsMsg struct {
	ContainerID string
	Lines       []string
	Err         error
}

// StackLogsMsg carries fetched stack logs.
type StackLogsMsg struct {
	StackName string
	Lines     []string
	Err       error
}

// ContainerActionMsg carries the result of a container action.
type ContainerActionMsg struct {
	ContainerID string
	Action      string
	Err         error
}

// StackActionMsg carries the result of a stack action.
type StackActionMsg struct {
	StackName string
	Action    string
	Attempted int
	Failed    []string
	Err       error
}

// ClearActionResultMsg clears the action result message after a delay.
type ClearActionResultMsg struct{}

// LogFollowLineMsg carries a single log line from the streaming follow.
type LogFollowLineMsg struct {
	Line string
	Done bool   // true when stream ends (container stopped, context cancelled)
	Seq  uint64 // session counter to discard stale messages
}

// CollapseSaveTickMsg fires after debounce delay to trigger save.
type CollapseSaveTickMsg struct{ Seq uint64 }

// CollapseSavedMsg carries the result of a save operation.
type CollapseSavedMsg struct {
	Seq uint64
	Err error
}

// DismissNotificationMsg fires after 5s to auto-dismiss a notification.
type DismissNotificationMsg struct{ ID uint64 }

// ContainerDetailMsg carries inspect data for the detail view.
type ContainerDetailMsg struct {
	ContainerID string
	Detail      collector.ContainerDetail
	Err         error
}
