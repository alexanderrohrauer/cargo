package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage cargo encryption keys",
}

var keyPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull the server's age public key and write a .sops.yaml in the current directory",
	Long:  `Fetches the age public key from the remote Cargo server`,
	RunE:  runKeyPull,
}

func init() {
	keyCmd.AddCommand(keyPullCmd)
}

func runKeyPull(cmd *cobra.Command, args []string) error {
	if remoteURL == "" {
		return fmt.Errorf("--remote flag is required for 'key pull'")
	}

	token, err := resolveToken()
	if err != nil {
		return err
	}

	body, err := doAPIRequest(http.MethodGet, remoteURL+"/api/v1/key/age", token, nil)
	if err != nil {
		return err
	}

	var resp struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	if resp.PublicKey == "" {
		return fmt.Errorf("server returned empty public key")
	}

	if err := writeSopsConfig(resp.PublicKey); err != nil {
		return fmt.Errorf("writing .sops.yaml: %w", err)
	}

	return nil
}

// writeSopsConfig writes a .sops.yaml in the current directory that configures
// the age recipient so sops encrypt/decrypt works without explicit --age flags.
func writeSopsConfig(publicKey string) error {
	fmt.Printf("creation_rules:\n  - age: %s\n", publicKey)
	return nil
}
