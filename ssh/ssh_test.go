package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	cf "github.com/rosscartlidge/autocli/v4"
	gossh "golang.org/x/crypto/ssh"
)

// setupTestKeys writes a generated ed25519 client keypair plus an
// authorized_keys file containing the public key into a tmpdir.
// Returns: dir, client signer, authorized_keys path.
func setupTestKeys(t *testing.T) (string, gossh.Signer, string) {
	t.Helper()
	dir := t.TempDir()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen ed25519: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	signer, err := gossh.ParsePrivateKey(privPEM)
	if err != nil {
		t.Fatalf("parse signer: %v", err)
	}

	sshPub, err := gossh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh pub: %v", err)
	}
	authorizedKeyLine := gossh.MarshalAuthorizedKey(sshPub)
	authKeysPath := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(authKeysPath, authorizedKeyLine, 0o600); err != nil {
		t.Fatalf("write auth keys: %v", err)
	}
	return dir, signer, authKeysPath
}

// startTestServer launches Serve on 127.0.0.1:0 and returns the bound
// address plus a stop func that cancels the server ctx and waits for
// it to return.
func startTestServer(t *testing.T, cli *cf.Command, opts Options) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr = ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeListener(ctx, ln, cli, opts) }()

	stop = func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Logf("server exited with: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("server did not stop within 10s")
		}
	}
	return addr, stop
}

// dialTestClient establishes an SSH client connection using the given
// signer. Caller is responsible for Close().
func dialTestClient(t *testing.T, addr string, signer gossh.Signer) *gossh.Client {
	t.Helper()
	cfg := &gossh.ClientConfig{
		User: "tester",
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	}
	client, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return client
}

// pingCommand builds a tiny CLI for use across tests.
func pingCommand() *cf.Command {
	return cf.NewCommand("svc").
		Subcommand("ping").
		Handler(func(ctx *cf.Context) error {
			fmt.Fprintln(ctx.Stdout(), "PONG")
			return nil
		}).
		Done().
		Build()
}

// TestServe_HappyPath exercises the full flow:
//
//	host-key gen → auth → session → command → response → :exit
//
// Verifies our SSH wrapper correctly bridges crypto/ssh and shell.Serve.
func TestServe_HappyPath(t *testing.T) {
	dir, signer, authKeys := setupTestKeys(t)
	addr, stop := startTestServer(t, pingCommand(), Options{
		HostKeyPath:    filepath.Join(dir, "host_key"),
		AuthorizedKeys: authKeys,
	})
	defer stop()

	client := dialTestClient(t, addr, signer)
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	var out bytes.Buffer
	var outMu sync.Mutex
	sess.Stdout = &lockedWriter{w: &out, mu: &outMu}
	sess.Stderr = &lockedWriter{w: &out, mu: &outMu}

	if err := sess.Shell(); err != nil {
		t.Fatalf("shell: %v", err)
	}
	// Send command + exit, then close stdin to signal EOF.
	io.WriteString(stdin, "ping\n:exit\n")
	stdin.Close()

	// Wait for session to end (server-side :exit closes channel).
	if err := sess.Wait(); err != nil {
		// Some SSH implementations report a non-nil error on normal
		// close; the value matters less than the captured output.
		t.Logf("sess.Wait: %v", err)
	}

	outMu.Lock()
	got := out.String()
	outMu.Unlock()
	if !strings.Contains(got, "PONG") {
		t.Errorf("expected PONG in output, got: %q", got)
	}
}

// TestServe_WrongKeyRejected dials with a different keypair than the
// authorized_keys file and asserts authentication fails.
func TestServe_WrongKeyRejected(t *testing.T) {
	dir, _, authKeys := setupTestKeys(t)
	addr, stop := startTestServer(t, pingCommand(), Options{
		HostKeyPath:    filepath.Join(dir, "host_key"),
		AuthorizedKeys: authKeys,
	})
	defer stop()

	// Fresh keypair NOT in authorized_keys.
	_, badPriv, _ := ed25519.GenerateKey(rand.Reader)
	badDer, _ := x509.MarshalPKCS8PrivateKey(badPriv)
	badPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: badDer})
	badSigner, _ := gossh.ParsePrivateKey(badPEM)

	cfg := &gossh.ClientConfig{
		User:            "tester",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(badSigner)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	}
	_, err := gossh.Dial("tcp", addr, cfg)
	if err == nil {
		t.Fatal("expected auth failure, got nil")
	}
	if !strings.Contains(err.Error(), "unable to authenticate") &&
		!strings.Contains(err.Error(), "ssh:") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestServe_RefusesEmptyAuth asserts Serve fails to start when
// neither AuthorizedKeys nor AuthCallback is set — no-auth services
// are refused.
func TestServe_RefusesEmptyAuth(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Serve(ctx, pingCommand(), Options{
		Addr:        "127.0.0.1:0",
		HostKeyPath: filepath.Join(dir, "host_key"),
	})
	if err == nil {
		t.Fatal("expected error for missing auth config")
	}
	if !strings.Contains(err.Error(), "AuthorizedKeys") {
		t.Errorf("error did not mention AuthorizedKeys: %v", err)
	}
}

// TestServe_HostKeyGeneratedOnFirstRun runs the server twice with
// the same HostKeyPath. First run should create the file; second run
// should reuse it (file content unchanged).
func TestServe_HostKeyGeneratedOnFirstRun(t *testing.T) {
	dir, signer, authKeys := setupTestKeys(t)
	hkPath := filepath.Join(dir, "host_key")
	opts := Options{HostKeyPath: hkPath, AuthorizedKeys: authKeys}

	addr1, stop1 := startTestServer(t, pingCommand(), opts)
	// Trigger generation by opening a connection.
	client := dialTestClient(t, addr1, signer)
	client.Close()
	stop1()

	hk1, err := os.ReadFile(hkPath)
	if err != nil {
		t.Fatalf("read host key: %v", err)
	}
	if len(hk1) == 0 {
		t.Fatal("host key file is empty")
	}

	addr2, stop2 := startTestServer(t, pingCommand(), opts)
	client = dialTestClient(t, addr2, signer)
	client.Close()
	stop2()

	hk2, err := os.ReadFile(hkPath)
	if err != nil {
		t.Fatalf("read host key after restart: %v", err)
	}
	if !bytes.Equal(hk1, hk2) {
		t.Error("host key changed across restarts")
	}
}

// TestServe_GracefulShutdown asserts a cancelled ctx returns from
// Serve within the grace timeout even with an open session.
func TestServe_GracefulShutdown(t *testing.T) {
	dir, signer, authKeys := setupTestKeys(t)
	addr, stop := startTestServer(t, pingCommand(), Options{
		HostKeyPath:    filepath.Join(dir, "host_key"),
		AuthorizedKeys: authKeys,
		GraceTimeout:   500 * time.Millisecond,
	})

	client := dialTestClient(t, addr, signer)
	sess, _ := client.NewSession()
	stdin, _ := sess.StdinPipe()
	sess.Stdout = io.Discard
	sess.Stderr = io.Discard
	sess.Shell()
	_ = stdin // session held open

	// Cancel server while session is alive.
	t0 := time.Now()
	stop()
	elapsed := time.Since(t0)
	if elapsed > 5*time.Second {
		t.Errorf("graceful shutdown took %v, want < 5s", elapsed)
	}
	client.Close()
}

// lockedWriter serialises writes to an underlying io.Writer — used
// because SSH session stdout/stderr can be written from multiple
// goroutines.
type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
