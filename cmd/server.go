package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"cargo/internal/api"
	"cargo/internal/config"
	"cargo/internal/project"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Cargo REST API server",
	Long: `Start the Cargo HTTPS REST API server.

The server loads the configuration file, reads the auth token from the workdir,
starts the project poller for any projects with poll_interval set,
and listens for incoming API requests.`,
	RunE: runServer,
}

func runServer(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Read auth token from workdir
	token, err := readAuthToken(cfg.Workdir)
	if err != nil {
		return fmt.Errorf("reading auth token: %w", err)
	}

	fmt.Printf("Cargo server starting...\n")
	fmt.Printf("  Config:     %s\n", cfgFile)
	fmt.Printf("  Workdir:    %s\n", cfg.Workdir)
	fmt.Printf("  Address:    https://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("  Auth token: %s\n", token)
	fmt.Println()

	mgr := project.NewManager(cfg)

	// Start poller for projects with poll_interval
	poller := project.NewPoller(mgr)
	poller.Start()
	defer poller.Stop()

	slog.Info("project poller started")

	// Start API server (blocking)
	srv := api.NewServer(mgr, token, cfg)
	if err := srv.Run(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// readAuthToken reads the auth token from <workdir>/auth_token.
func readAuthToken(workdir string) (string, error) {
	tokenPath := filepath.Join(workdir, "auth_token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("reading auth token from %q: %w", tokenPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}
