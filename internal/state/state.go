package state

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	sf := stateFile{CollapsedStacks: names}
	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
