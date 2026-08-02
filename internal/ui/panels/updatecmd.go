package panels

import (
	"fmt"
	"strings"
)

// Compose labels every container created by `docker compose` carries. They
// are what makes a zero-configuration update command possible: the container
// itself records which file defined it and under which service name.
const (
	labelComposeConfigFiles = "com.docker.compose.project.config_files"
	labelComposeService     = "com.docker.compose.service"
)

// UpdateCommand builds the command that applies a pending image update for a
// container, from the container's own labels.
//
// Returns "" when the container was not created by compose, since there is
// then no correct single command to offer — `docker run` invocations cannot
// be reconstructed from a running container without guessing at flags, and a
// wrong command is worse than none.
//
// The command is shown, never executed: HomeDash speaks only the Docker HTTP
// API and has no subprocess dependency on the docker CLI.
func UpdateCommand(labels map[string]string) string {
	configFiles := strings.TrimSpace(labels[labelComposeConfigFiles])
	service := strings.TrimSpace(labels[labelComposeService])
	if configFiles == "" || service == "" {
		return ""
	}

	// compose records multiple -f files comma-separated, in override order.
	var fileArgs []string
	for _, f := range strings.Split(configFiles, ",") {
		if f = strings.TrimSpace(f); f != "" {
			fileArgs = append(fileArgs, "-f "+quoteIfNeeded(f))
		}
	}
	if len(fileArgs) == 0 {
		return ""
	}

	return fmt.Sprintf("docker compose %s up -d --pull always %s",
		strings.Join(fileArgs, " "), quoteIfNeeded(service))
}

// quoteIfNeeded single-quotes a path containing shell-significant characters,
// so a command copied out of the panel pastes correctly.
func quoteIfNeeded(s string) string {
	if !strings.ContainsAny(s, " \t\"'$&|;<>()*?[]#~!`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
