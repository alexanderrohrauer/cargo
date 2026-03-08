package project

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"cargo/internal/compose"
	"cargo/internal/config"
	"cargo/internal/git"
	"cargo/internal/sops"
)

// Manager manages all configured compose projects.
type Manager struct {
	Config  *config.Config
	Workdir string
}

// SyncResult holds the result of syncing a single project.
type SyncResult struct {
	ProjectName string `json:"project_name"`
	Commit      string `json:"commit"`
	Error       error  `json:"error,omitempty"`
}

// NewManager creates a new Manager for the given configuration.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		Config:  cfg,
		Workdir: cfg.Workdir,
	}
}

// ProjectDir returns the directory where the project's repository is cloned.
func (m *Manager) ProjectDir(name string) string {
	return filepath.Join(m.Workdir, "projects", name)
}

// findProject looks up a project by name in the config.
func (m *Manager) findProject(name string) (config.ProjectConfig, error) {
	for _, p := range m.Config.Projects {
		if p.Name == name {
			return p, nil
		}
	}
	return config.ProjectConfig{}, fmt.Errorf("project %q not found in config", name)
}

// SyncProject performs a full sync of the named project:
// git clone/pull → sops decrypt env → docker compose pull → docker compose up.
func (m *Manager) SyncProject(name string) SyncResult {
	result := SyncResult{ProjectName: name}

	projectCfg, err := m.findProject(name)
	if err != nil {
		result.Error = err
		return result
	}

	projectDir := m.ProjectDir(name)

	// Ensure the projects directory exists
	if err := os.MkdirAll(filepath.Dir(projectDir), 0755); err != nil {
		result.Error = fmt.Errorf("creating projects directory: %w", err)
		return result
	}

	// Clone or pull the repository
	if git.IsCloned(projectDir) {
		slog.Info("pulling existing repository", "project", name, "dir", projectDir)
		if err := git.Pull(projectCfg, projectDir); err != nil {
			result.Error = fmt.Errorf("pulling repository: %w", err)
			return result
		}
	} else {
		slog.Info("cloning repository", "project", name, "url", projectCfg.Git.URL)
		if err := git.Clone(projectCfg, projectDir); err != nil {
			result.Error = fmt.Errorf("cloning repository: %w", err)
			return result
		}
	}

	// Capture current commit
	commit, err := git.CurrentCommit(projectDir)
	if err != nil {
		slog.Warn("could not get current commit", "project", name, "error", err)
	}
	result.Commit = commit

	// Resolve compose file path
	composeDir := filepath.Join(projectDir, projectCfg.Compose.Path)
	composeFile := filepath.Join(composeDir, projectCfg.Compose.File)

	// Handle SOPS decryption
	var envFile string
	if projectCfg.SOPS.Enabled && projectCfg.SOPS.EnvFile != "" {
		encryptedEnvPath := filepath.Join(projectDir, projectCfg.SOPS.EnvFile)
		decryptedEnvPath := filepath.Join(projectDir, ".decrypted.env")

		slog.Info("decrypting SOPS env file", "project", name, "encrypted", encryptedEnvPath)
		if err := sops.Decrypt(projectCfg.SOPS, encryptedEnvPath, decryptedEnvPath); err != nil {
			result.Error = fmt.Errorf("decrypting SOPS env file: %w", err)
			return result
		}
		envFile = decryptedEnvPath
	}

	// Pull latest docker images
	slog.Info("pulling docker images", "project", name)
	if err := compose.Pull(composeDir, composeFile, m.Config.HostWorkdir); err != nil {
		slog.Warn("docker compose pull failed (continuing)", "project", name, "error", err)
	}

	// Start the compose project
	slog.Info("starting compose project", "project", name)
	if err := compose.Up(composeDir, composeFile, envFile, m.Config.HostWorkdir); err != nil {
		result.Error = fmt.Errorf("starting compose project: %w", err)
		return result
	}

	slog.Info("project synced successfully", "project", name, "commit", commit)
	return result
}

// SyncAll syncs all projects defined in the config.
func (m *Manager) SyncAll() []SyncResult {
	results := make([]SyncResult, 0, len(m.Config.Projects))
	for _, p := range m.Config.Projects {
		slog.Info("syncing project", "project", p.Name)
		result := m.SyncProject(p.Name)
		if result.Error != nil {
			slog.Error("project sync failed", "project", p.Name, "error", result.Error)
		}
		results = append(results, result)
	}
	return results
}

// StatusProject returns the docker compose ps output for the named project.
func (m *Manager) StatusProject(name string) (string, error) {
	projectCfg, err := m.findProject(name)
	if err != nil {
		return "", err
	}

	projectDir := m.ProjectDir(name)
	composeDir := filepath.Join(projectDir, projectCfg.Compose.Path)
	composeFile := filepath.Join(composeDir, projectCfg.Compose.File)

	status, err := compose.Status(composeDir, composeFile, m.Config.HostWorkdir)
	if err != nil {
		return "", fmt.Errorf("getting status for project %q: %w", name, err)
	}

	return status, nil
}
