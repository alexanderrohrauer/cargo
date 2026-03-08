package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Workdir     string          `yaml:"workdir"`
	HostWorkdir string          `yaml:"host_workdir"`
	Server      ServerConfig    `yaml:"server"`
	Projects    []ProjectConfig `yaml:"projects"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type ProjectConfig struct {
	Name         string        `yaml:"name"`
	Git          GitConfig     `yaml:"git"`
	Compose      ComposeConfig `yaml:"compose"`
	PollInterval string        `yaml:"poll_interval"`
	SOPS         SOPSConfig    `yaml:"sops"`
}

type GitConfig struct {
	URL      string `yaml:"url"`
	Revision string `yaml:"revision"`
	SSHKey   string `yaml:"ssh_key"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ComposeConfig struct {
	Path string `yaml:"path"` // subfolder in repo
	File string `yaml:"file"` // defaults to docker-compose.yml
}

type SOPSConfig struct {
	Enabled bool       `yaml:"enabled"`
	AgeKey  string     `yaml:"age_key"` // path to age private key file
	Files   []SOPSFile `yaml:"files"`   // list of files to decrypt
}

// SOPSFile describes one SOPS-encrypted file and where to write the decrypted output.
type SOPSFile struct {
	Input  string `yaml:"input"`  // encrypted file path relative to the project dir
	Output string `yaml:"output"` // decrypted output path relative to the project dir; defaults to Input with .enc stripped
}

// ResolvedOutput returns the output path for the decrypted file.
// If Output is set it is returned as-is; otherwise .enc is stripped from Input
// (handling both "file.enc" and "file.enc.ext" patterns). Falls back to Input+".dec".
func (f SOPSFile) ResolvedOutput() string {
	if f.Output != "" {
		return f.Output
	}
	ext := filepath.Ext(f.Input)
	base := strings.TrimSuffix(f.Input, ext)
	if strings.HasSuffix(base, ".enc") {
		return strings.TrimSuffix(base, ".enc") + ext
	}
	if strings.HasSuffix(f.Input, ".enc") {
		return strings.TrimSuffix(f.Input, ".enc")
	}
	return f.Input + ".dec"
}

// Load reads a YAML config file and returns the parsed Config.
// It expands ~ in paths and sets sensible defaults.
func Load(path string) (*Config, error) {
	path = expandHome(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	setDefaults(cfg)
	expandPaths(cfg)

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Workdir == "" {
		cfg.Workdir = "~/.cargo/workdir"
	}
	// HostWorkdir is the workdir path as seen by the Docker daemon on the host.
	// When cargo runs inside a container, this must be set to the host-side mount path
	// so that Docker volume mounts in compose files resolve correctly.
	// Defaults to workdir (correct for non-containerized usage).
	if cfg.HostWorkdir == "" {
		cfg.HostWorkdir = cfg.Workdir
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8443
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	for i := range cfg.Projects {
		if cfg.Projects[i].Compose.File == "" {
			cfg.Projects[i].Compose.File = "docker-compose.yml"
		}
		if cfg.Projects[i].Git.Revision == "" {
			cfg.Projects[i].Git.Revision = "main"
		}
	}
}

func expandPaths(cfg *Config) {
	cfg.Workdir = expandHome(cfg.Workdir)
	cfg.HostWorkdir = expandHome(cfg.HostWorkdir)
	for i := range cfg.Projects {
		if cfg.Projects[i].Git.SSHKey != "" {
			cfg.Projects[i].Git.SSHKey = expandHome(cfg.Projects[i].Git.SSHKey)
		}
		if cfg.Projects[i].SOPS.AgeKey != "" {
			cfg.Projects[i].SOPS.AgeKey = expandHome(cfg.Projects[i].SOPS.AgeKey)
		}
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
