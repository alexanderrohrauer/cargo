package compose

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// Up runs `docker compose up -d` in the given project directory.
// If envFile is non-empty, it is passed via --env-file.
// hostWorkdir is passed as CARGO_HOST_WORKDIR so compose files can reference host paths in volumes.
func Up(projectDir, composeFile string, envFile string, hostWorkdir string) error {
	args := []string{"compose", "-f", composeFile, "up", "-d"}
	if envFile != "" {
		args = []string{"compose", "--env-file", envFile, "-f", composeFile, "up", "-d"}
	}

	cmd := buildCommand(projectDir, hostWorkdir, args...)
	slog.Info("running docker compose up", "dir", projectDir, "file", composeFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up failed: %w\noutput: %s", err, string(output))
	}
	slog.Debug("docker compose up output", "output", string(output))
	return nil
}

// Down runs `docker compose down` in the given project directory.
// hostWorkdir is passed as CARGO_HOST_WORKDIR so compose files can reference host paths in volumes.
func Down(projectDir, composeFile string, hostWorkdir string) error {
	cmd := buildCommand(projectDir, hostWorkdir, "compose", "-f", composeFile, "down")
	slog.Info("running docker compose down", "dir", projectDir, "file", composeFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose down failed: %w\noutput: %s", err, string(output))
	}
	slog.Debug("docker compose down output", "output", string(output))
	return nil
}

// Pull runs `docker compose pull` to pull the latest images.
// hostWorkdir is passed as CARGO_HOST_WORKDIR so compose files can reference host paths in volumes.
func Pull(projectDir, composeFile string, hostWorkdir string) error {
	cmd := buildCommand(projectDir, hostWorkdir, "compose", "-f", composeFile, "pull")
	slog.Info("running docker compose pull", "dir", projectDir, "file", composeFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose pull failed: %w\noutput: %s", err, string(output))
	}
	slog.Debug("docker compose pull output", "output", string(output))
	return nil
}

// Status runs `docker compose ps --format json` and returns the raw JSON output.
// hostWorkdir is passed as CARGO_HOST_WORKDIR so compose files can reference host paths in volumes.
func Status(projectDir, composeFile string, hostWorkdir string) (string, error) {
	cmd := buildCommand(projectDir, hostWorkdir, "compose", "-f", composeFile, "ps", "--format", "json")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Info("running docker compose ps", "dir", projectDir, "file", composeFile)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("docker compose ps failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// buildCommand constructs an exec.Cmd for docker with the given args, setting the working
// directory to projectDir and resolving the compose file as an absolute path if needed.
// hostWorkdir is exposed as CARGO_HOST_WORKDIR in the child process environment.
func buildCommand(projectDir string, hostWorkdir string, args ...string) *exec.Cmd {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		dockerPath = "docker"
	}

	// Make composeFile absolute relative to projectDir if it's not already absolute
	for i, arg := range args {
		if i > 0 && args[i-1] == "-f" && !filepath.IsAbs(arg) {
			args[i] = filepath.Join(projectDir, arg)
		}
	}

	// #nosec G204 - args come from configuration, not user input
	cmd := exec.Command(dockerPath, args...)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "CARGO_HOST_WORKDIR="+hostWorkdir)
	return cmd
}
