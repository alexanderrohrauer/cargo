package sops

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"filippo.io/age"

	"cargo/internal/config"
)

// GenerateAgeKey generates an age X25519 keypair and saves the private key to outputPath.
// It returns the public key string.
func GenerateAgeKey(outputPath string) (string, error) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return "", fmt.Errorf("generating age X25519 identity: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return "", fmt.Errorf("creating directory for age key: %w", err)
	}

	privKeyContent := fmt.Sprintf("# created by cargo\n# public key: %s\n%s\n",
		identity.Recipient().String(),
		identity.String(),
	)

	if err := os.WriteFile(outputPath, []byte(privKeyContent), 0600); err != nil {
		return "", fmt.Errorf("writing age private key to %q: %w", outputPath, err)
	}

	slog.Info("generated age keypair", "private_key_path", outputPath, "public_key", identity.Recipient().String())
	return identity.Recipient().String(), nil
}

// Decrypt decrypts a SOPS-encrypted file using the age key specified in sopsConfig.
// The decrypted output is written to outputPath.
func Decrypt(sopsConfig config.SOPSConfig, encryptedFilePath, outputPath string) error {
	if !sopsConfig.Enabled {
		return nil
	}

	sopsPath, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops not found in PATH: %w", err)
	}

	slog.Info("decrypting SOPS file", "input", encryptedFilePath, "output", outputPath, "age-key-path", sopsConfig.AgeKey)
	// #nosec G204 - paths come from configuration, not user input
	cmd := exec.Command(sopsPath, "--decrypt", "--output", outputPath, encryptedFilePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOPS_AGE_KEY_FILE=%s", sopsConfig.AgeKey))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sops decrypt failed: %w\noutput: %s", err, string(output))
	}

	slog.Info("decrypted SOPS file", "input", encryptedFilePath, "output", outputPath)
	return nil
}

// Encrypt encrypts a plaintext file using SOPS with the age public key specified in sopsConfig.
// The encrypted output is written to outputPath.
func Encrypt(sopsConfig config.SOPSConfig, plaintextFilePath, outputPath string) error {
	if !sopsConfig.Enabled {
		return nil
	}

	publicKey, err := readPublicKeyFromPrivateKeyFile(sopsConfig.AgeKey)
	if err != nil {
		return fmt.Errorf("reading public key from age key file %q: %w", sopsConfig.AgeKey, err)
	}

	sopsPath, err := exec.LookPath("sops")
	if err != nil {
		return fmt.Errorf("sops not found in PATH: %w", err)
	}

	// #nosec G204 - paths come from configuration, not user input
	cmd := exec.Command(sopsPath, "--encrypt",
		"--age", publicKey,
		"--output", outputPath,
		plaintextFilePath,
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("SOPS_AGE_KEY_FILE=%s", sopsConfig.AgeKey))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sops encrypt failed: %w\noutput: %s", err, string(output))
	}

	slog.Info("encrypted file with SOPS", "input", plaintextFilePath, "output", outputPath)
	return nil
}

// ReadPublicKey reads an age private key file and extracts the public key (recipient).
func ReadPublicKey(privateKeyPath string) (string, error) {
	return readPublicKeyFromPrivateKeyFile(privateKeyPath)
}

// readPublicKeyFromPrivateKeyFile reads an age private key file and extracts the public key
// by parsing the identities with the age library.
func readPublicKeyFromPrivateKeyFile(privateKeyPath string) (string, error) {
	data, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("reading age key file %q: %w", privateKeyPath, err)
	}

	identities, err := age.ParseIdentities(io.NopCloser(strings.NewReader(string(data))))
	if err != nil {
		return "", fmt.Errorf("parsing age identities from %q: %w", privateKeyPath, err)
	}

	if len(identities) == 0 {
		return "", fmt.Errorf("no age identities found in %q", privateKeyPath)
	}

	x25519Identity, ok := identities[0].(*age.X25519Identity)
	if !ok {
		return "", fmt.Errorf("identity in %q is not an X25519 identity", privateKeyPath)
	}

	return x25519Identity.Recipient().String(), nil
}
