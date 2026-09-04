package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kts982/homedash/internal/collector"
	"github.com/kts982/homedash/internal/collector/registry"
	"github.com/kts982/homedash/internal/config"
	"github.com/kts982/homedash/internal/state"
)

// updateCheckTimeout bounds a whole update sweep. Generous, because it covers
// every image across every registry, but finite so a hung registry cannot
// leave the check spinning forever.
const updateCheckTimeout = 90 * time.Second

type stackActionTarget struct {
	ID   string
	Name string
}

// oneShot is the Epoch of a collection no tick chain asked for. Model epochs
// start at 1, so it never matches a live chain.
const oneShot uint64 = 0

func collectSystemCmd(disks []config.Disk, epoch uint64) tea.Msg {
	data, err := collector.CollectSystem(disks)
	return SystemDataMsg{Data: data, Err: err, Epoch: epoch}
}

func collectDockerCmd(epoch uint64) tea.Msg {
	data, err := collector.CollectDocker()
	return DockerDataMsg{Data: data, Err: err, Epoch: epoch}
}

func collectWeatherCmd(epoch uint64) tea.Msg {
	data, err := collector.CollectWeather()
	return WeatherDataMsg{Data: data, Err: err, Epoch: epoch}
}

func systemTickCmd(disks []config.Disk, interval time.Duration, epoch uint64) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return SystemTickMsg{Epoch: epoch}
	})
}

func dockerTickCmd(interval time.Duration, epoch uint64) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return DockerTickMsg{Epoch: epoch}
	})
}

func weatherTickCmd(interval time.Duration, epoch uint64) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return WeatherTickMsg{Epoch: epoch}
	})
}

func weatherRetryCmd(epoch uint64) tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return WeatherTickMsg{Epoch: epoch}
	})
}

func saveSettingsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		err := config.Save(cfg)
		return SettingsSavedMsg{Config: cfg, Err: err}
	}
}

func collectLogsCmd(containerID string, tail int) tea.Cmd {
	return func() tea.Msg {
		lines, err := collector.FetchContainerLogs(containerID, tail)
		return ContainerLogsMsg{ContainerID: containerID, Lines: lines, Err: err}
	}
}

func collectStackLogsCmd(containers []collector.Container, stackName string, tail int) tea.Cmd {
	return func() tea.Msg {
		lines, err := collector.FetchStackLogs(containers, stackName, tail)
		return StackLogsMsg{StackName: stackName, Lines: lines, Err: err}
	}
}

func containerActionCmd(containerID, action string) tea.Cmd {
	return func() tea.Msg {
		err := collector.ContainerAction(containerID, action)
		return ContainerActionMsg{ContainerID: containerID, Action: action, Err: err}
	}
}

func stackActionTargets(containers []collector.Container, stackName, action string) []stackActionTarget {
	var targets []stackActionTarget
	for _, c := range containers {
		if c.Stack != stackName {
			continue
		}

		switch action {
		case "start":
			if c.State == "running" {
				continue
			}
		case "stop", "restart":
			if c.State != "running" {
				continue
			}
		default:
			return nil
		}

		targets = append(targets, stackActionTarget{
			ID:   c.ID,
			Name: c.Name,
		})
	}
	return targets
}

func stackActionCmd(containers []collector.Container, stackName, action string) tea.Cmd {
	targets := stackActionTargets(containers, stackName, action)
	return func() tea.Msg {
		msg := StackActionMsg{
			StackName: stackName,
			Action:    action,
			Attempted: len(targets),
		}
		for _, target := range targets {
			if err := collector.ContainerAction(target.ID, action); err != nil {
				if msg.Err == nil {
					msg.Err = err
				}
				msg.Failed = append(msg.Failed, target.Name)
			}
		}
		return msg
	}
}

func clearActionResultCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ClearActionResultMsg{}
	})
}

// logFollowCmd reads the next line from the follow channel. When the channel
// closes, the stream's terminal error (nil for EOF or cancellation) is
// waiting on errCh: the producer sends it before closing ch.
func logFollowCmd(ch <-chan string, errCh <-chan error, seq uint64) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return LogFollowLineMsg{Done: true, Err: <-errCh, Seq: seq}
		}
		return LogFollowLineMsg{Line: line, Seq: seq}
	}
}

func collectDetailCmd(containerID string) tea.Cmd {
	return func() tea.Msg {
		detail, err := collector.InspectContainer(containerID)
		return ContainerDetailMsg{ContainerID: containerID, Detail: detail, Err: err}
	}
}

func collapseSaveTickCmd(seq uint64) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return CollapseSaveTickMsg{Seq: seq}
	})
}

func collapseSaveCmd(collapsed map[string]bool, seq uint64) tea.Cmd {
	// Clone to avoid race with main goroutine mutations.
	snapshot := make(map[string]bool, len(collapsed))
	for k, v := range collapsed {
		snapshot[k] = v
	}

	return func() tea.Msg {
		err := state.Save(snapshot)
		return CollapseSavedMsg{Seq: seq, Err: err}
	}
}

func dismissNotificationCmd(id uint64) tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return DismissNotificationMsg{ID: id}
	})
}

// checkUpdatesCmd queries each running container's registry for a newer
// manifest digest.
//
// Deliberately manual rather than tied to the refresh tick: the Docker
// refresh interval defaults to 5s, and hitting five registries at that rate
// is both rude and liable to trip anonymous rate limits. The containers
// slice is cloned because this runs on its own goroutine.
func checkUpdatesCmd(containers []collector.Container) tea.Cmd {
	snapshot := append([]collector.Container(nil), containers...)

	return func() tea.Msg {
		targets, err := collector.UpdateTargets(snapshot)
		if err != nil {
			return UpdateCheckMsg{Err: err}
		}
		if len(targets) == 0 {
			return UpdateCheckMsg{}
		}

		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()

		return UpdateCheckMsg{Statuses: registry.NewHTTPChecker().Check(ctx, targets)}
	}
}
