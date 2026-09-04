package ui

import (
	"github.com/kts982/homedash/internal/collector"
	"github.com/kts982/homedash/internal/collector/registry"
	"github.com/kts982/homedash/internal/config"
)

// SystemDataMsg carries updated system metrics.
//
// Epoch identifies the tick chain whose timer requested this collection. It
// is oneShot for a refresh nobody scheduled (startup, `r`, the follow-up
// after an action), and such a result must not schedule the next tick: only
// the chain's own result continues the chain, otherwise every extra refresh
// would add a permanent second chain and the poll rate would multiply.
type SystemDataMsg struct {
	Data  collector.SystemData
	Err   error
	Epoch uint64
}

// DockerDataMsg carries updated Docker container data. See SystemDataMsg
// for Epoch.
type DockerDataMsg struct {
	Data  collector.DockerData
	Err   error
	Epoch uint64
}

// WeatherDataMsg carries updated weather data. See SystemDataMsg for Epoch.
type WeatherDataMsg struct {
	Data  collector.WeatherData
	Err   error
	Epoch uint64
}

// SystemTickMsg is sent by the periodic timer to trigger system collection.
type SystemTickMsg struct{ Epoch uint64 }

// DockerTickMsg is sent by the periodic timer to trigger Docker collection.
type DockerTickMsg struct{ Epoch uint64 }

// WeatherTickMsg is sent by the periodic timer to trigger weather collection.
type WeatherTickMsg struct{ Epoch uint64 }

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
	Err  error  // with Done: the stream failed rather than ending or being cancelled
	Seq  uint64 // session counter to discard stale messages
}

// followRestartMsg triggers an automatic restart of log following after
// a stream ends unexpectedly (e.g. container restart).
type followRestartMsg struct{}

// configWarningsMsg carries non-fatal problems found while loading config, so
// they surface as notifications rather than blocking startup.
type configWarningsMsg struct {
	warnings []string
}

// UpdateCheckMsg carries the result of a manual image update check.
type UpdateCheckMsg struct {
	Statuses []registry.Status
	Err      error
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

// SettingsSavedMsg carries the result of persisting updated app settings.
type SettingsSavedMsg struct {
	Config config.Config
	Err    error
}
