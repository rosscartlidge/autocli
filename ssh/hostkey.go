package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	gossh "golang.org/x/crypto/ssh"
)

// loadOrGenerateHostKey reads path as an OpenSSH/PEM-encoded private
// key. If the file is absent, generates a fresh ed25519 key, writes
// it (0600), and returns the parsed signer.
//
// Matches the contract of a real sshd: first-run boot generates the
// key persistently; subsequent runs reuse it; rotation is "replace
// the file and restart".
func loadOrGenerateHostKey(path string) (gossh.Signer, error) {
	if data, err := os.ReadFile(path); err == nil {
		signer, err := gossh.ParsePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		return signer, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Generate fresh ed25519 key.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519: %w", err)
	}
	derBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal pkcs8: %w", err)
	}
	pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: derBytes}
	pemBytes := pem.EncodeToMemory(pemBlock)
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	signer, err := gossh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("re-parse generated key: %w", err)
	}
	return signer, nil
}
