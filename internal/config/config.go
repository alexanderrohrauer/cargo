package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Workdir  string          `yaml:"workdir"`
	Server   ServerConfig    `yaml:"server"`
	Projects []ProjectConfig `yaml:"projects"`
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
	Enabled bool   `yaml:"enabled"`
	AgeKey  string `yaml:"age_key"`  // path to age private key file
	EnvFile string `yaml:"env_file"` // encrypted env file path in repo
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
