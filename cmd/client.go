package cmd

import (
	"crypto/tls"
)

// insecureTLSConfig returns a *tls.Config that skips certificate verification.
// This is used for the CLI client to communicate with a Cargo server that uses
// a self-signed certificate.
func insecureTLSConfig() *tls.Config {
	return &tls.Config{
		// #nosec G402 - intentionally skipping verification for self-signed certs
		InsecureSkipVerify: true,
	}
}
