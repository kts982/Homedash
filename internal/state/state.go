package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type stateFile struct {
	CollapsedStacks []string `json:"collapsed_stacks"`
}

func statePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "homedash", "state.json"), nil
}

// Load reads collapsed stacks from state file. Returns empty map on any error.
func Load() map[string]bool {
	path, err := statePath()
	if err != nil {
		return make(map[string]bool)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]bool)
	}

	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return make(map[string]bool)
	}

	result := make(map[string]bool, len(sf.CollapsedStacks))
	for _, name := range sf.CollapsedStacks {
		result[name] = true
	}
	return result
}

// Save writes collapsed stacks to state file atomically (temp + rename).
func Save(collapsed map[string]bool) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var names []string
	for name, isCollapsed := range collapsed {
		if isCollapsed {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	sf := stateFile{CollapsedStacks: names}
	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}

	// A unique temp name keeps two HomeDash instances (tmux panes are
	// normal for this audience) from interleaving writes into one file and
	// renaming a truncated result into place.
	tmp, err := os.CreateTemp(dir, ".state-*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
