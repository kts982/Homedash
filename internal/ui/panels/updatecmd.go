package panels

import (
	"path/filepath"
	"strings"
)

// Compose labels every container created by `docker compose` carries. They
// are what makes a zero-configuration update command possible: the container
// itself records which files defined it, under which project and service
// name, and which environment files were interpolated into it.
const (
	labelComposeProject     = "com.docker.compose.project"
	labelComposeConfigFiles = "com.docker.compose.project.config_files"
	labelComposeWorkingDir  = "com.docker.compose.project.working_dir"
	labelComposeEnvFiles    = "com.docker.compose.project.environment_file"
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
//
// Every flag compose would need to reproduce the original `up` is passed
// explicitly. Leaving one out is not harmless: without -p a stack started
// under a custom project name is recreated as a second project, and without
// --env-file the variables the compose file interpolates come out empty, so
// `up -d` would recreate the container with a different configuration.
func UpdateCommand(labels map[string]string) string {
	files := splitComposeList(labels[labelComposeConfigFiles])
	service := strings.TrimSpace(labels[labelComposeService])
	if len(files) == 0 || service == "" {
		return ""
	}
	workingDir := strings.TrimSpace(labels[labelComposeWorkingDir])
	for i := range files {
		files[i] = resolveComposePath(files[i], workingDir)
	}

	args := []string{"docker", "compose"}
	if project := strings.TrimSpace(labels[labelComposeProject]); project != "" {
		args = append(args, "-p", quoteIfNeeded(project))
	}
	// compose resolves .env and relative paths against the first file's
	// directory unless told otherwise; only spell it out when they differ.
	if workingDir != "" && filepath.Clean(workingDir) != filepath.Dir(files[0]) {
		args = append(args, "--project-directory", quoteIfNeeded(workingDir))
	}
	for _, f := range files {
		args = append(args, "-f", quoteIfNeeded(f))
	}
	for _, envFile := range splitComposeList(labels[labelComposeEnvFiles]) {
		args = append(args, "--env-file", quoteIfNeeded(resolveComposePath(envFile, workingDir)))
	}
	args = append(args, "up", "-d", "--pull", "always", quoteIfNeeded(service))
	return strings.Join(args, " ")
}

// splitComposeList splits the comma-separated lists compose writes into its
// labels (config files in override order, environment files in load order).
func splitComposeList(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// resolveComposePath anchors a relative label path (compose v1 wrote those)
// to the project working directory so the command works from any cwd.
func resolveComposePath(path, workingDir string) string {
	if filepath.IsAbs(path) || workingDir == "" {
		return path
	}
	return filepath.Join(workingDir, path)
}

// quoteIfNeeded single-quotes a path containing shell-significant characters,
// so a command copied out of the panel pastes correctly.
func quoteIfNeeded(s string) string {
	if !strings.ContainsAny(s, " \t\"'$&|;<>()*?[]#~!`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
