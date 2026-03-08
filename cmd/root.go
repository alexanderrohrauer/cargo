package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	remoteURL string
	authToken string
)

var rootCmd = &cobra.Command{
	Use:   "cargo",
	Short: "Cargo - GitOps tool for Docker Compose",
	Long: `Cargo is a GitOps tool for Docker Compose.
It can run as a server on a remote machine, or as a client that communicates
with the remote Cargo server via its REST API.`,
}

// Execute runs the root command. Called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "~/.cargo/config.yml",
		"path to the config file")
	rootCmd.PersistentFlags().StringVar(&remoteURL, "remote", "",
		"remote Cargo server URL (enables client mode, e.g. https://my-server:8443)")
	rootCmd.PersistentFlags().StringVar(&authToken, "token", "",
		"auth token for remote server (defaults to reading from ~/.cargo/workdir/auth_token)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(projectCmd)
}
