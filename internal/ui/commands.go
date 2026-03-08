package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kostas/homedash/internal/collector"
	"github.com/kostas/homedash/internal/config"
	"github.com/kostas/homedash/internal/state"
)

type stackActionTarget struct {
	ID   string
	Name string
}

func collectSystemCmd(disks []config.Disk) tea.Msg {
	data, err := collector.CollectSystem(disks)
	return SystemDataMsg{Data: data, Err: err}
}

func collectDockerCmd() tea.Msg {
	data, err := collector.CollectDocker()
	return DockerDataMsg{Data: data, Err: err}
}

func collectWeatherCmd() tea.Msg {
	data, err := collector.CollectWeather()
	return WeatherDataMsg{Data: data, Err: err}
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

// logFollowCmd reads the next line from the follow channel.
func logFollowCmd(ch <-chan string, seq uint64) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return LogFollowLineMsg{Done: true, Seq: seq}
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
