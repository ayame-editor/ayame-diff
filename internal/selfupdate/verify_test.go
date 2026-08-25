package selfupdate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withPublicKey compiles a key into the package for one test. The real constant
// is empty until a signing key is configured, so every signature path needs a
// key installed here to be exercised at all.
func withPublicKey(t *testing.T, key ed25519.PublicKey) {
	t.Helper()
	original := activePublicKey
	activePublicKey = base64.StdEncoding.EncodeToString(key)
	t.Cleanup(func() { activePublicKey = original })
}

func newKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return public, private
}

func signatureFile(private ed25519.PrivateKey, message []byte) []byte {
	return []byte("# ed25519 signature over SHA256SUMS\n" +
		base64.StdEncoding.EncodeToString(ed25519.Sign(private, message)) + "\n")
}

func TestVerifySumsAcceptsTheReleaseKey(t *testing.T) {
	public, private := newKeyPair(t)
	withPublicKey(t, public)

	sums := []byte("abc  ayame-diff-v1.0.0-linux-amd64.tar.gz\n")
	if err := verifySums(sums, signatureFile(private, sums)); err != nil {
		t.Fatalf("a correctly signed SHA256SUMS was refused: %v", err)
	}
}

func TestVerifySumsRefusesTamperingAndForeignKeys(t *testing.T) {
	public, private := newKeyPair(t)
	_, other := newKeyPair(t)
	withPublicKey(t, public)

	sums := []byte("abc  ayame-diff-v1.0.0-linux-amd64.tar.gz\n")
	signature := signatureFile(private, sums)

	tampered := []byte("dead  ayame-diff-v1.0.0-linux-amd64.tar.gz\n")
	if err := verifySums(tampered, signature); err == nil {
		t.Error("a modified SHA256SUMS passed verification")
	}
	if err := verifySums(sums, signatureFile(other, sums)); err == nil {
		t.Error("a signature from another key passed verification")
	}
	for name, body := range map[string]string{
		"empty":       "",
		"comment":     "# nothing else\n",
		"not base64":  "not base64!!\n",
		"wrong size":  base64.StdEncoding.EncodeToString([]byte("short")) + "\n",
		"truncated":   base64.StdEncoding.EncodeToString(ed25519.Sign(private, sums)[:32]) + "\n",
		"another sum": base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte("other"))) + "\n",
	} {
		if err := verifySums(sums, []byte(body)); err == nil {
			t.Errorf("a %s signature passed verification", name)
		}
	}
}

func TestSignatureRequiredFollowsTheCompiledKey(t *testing.T) {
	if releasePublicKey != "" {
		t.Skip("a release key is configured; the unconfigured path cannot be exercised")
	}
	if signatureRequired() {
		t.Fatal("an unconfigured build must not claim it can verify signatures")
	}
	public, _ := newKeyPair(t)
	withPublicKey(t, public)
	if !signatureRequired() {
		t.Fatal("a configured build must require a signature")
	}
}

func TestPublicKeyMustBeWellFormed(t *testing.T) {
	original := activePublicKey
	t.Cleanup(func() { activePublicKey = original })

	for name, key := range map[string]string{
		"not base64": "not base64!!",
		"too short":  base64.StdEncoding.EncodeToString([]byte("short")),
	} {
		activePublicKey = key
		if _, err := parsePublicKey(); err == nil {
			t.Errorf("a %s public key was accepted", name)
		} else if !strings.Contains(err.Error(), "release public key") {
			t.Errorf("a %s public key reported an unclear error: %v", name, err)
		}
	}
}

// verifyRelease is the gate an update passes through. Each case here is a
// release a user could be served, and the outcome is what they get.
func TestVerifyReleaseGatesTheUpdate(t *testing.T) {
	public, private := newKeyPair(t)
	sums := []byte("abc  ayame-diff-v1.0.0-linux-amd64.tar.gz\n")

	serve := func(body []byte) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(body)
		}))
	}

	t.Run("an unconfigured build says it cannot verify", func(t *testing.T) {
		if releasePublicKey != "" {
			t.Skip("a release key is configured")
		}
		out := &strings.Builder{}
		release := &Release{TagName: "v1.0.0"}
		if err := verifyRelease(context.Background(), release, sums, out); err != nil {
			t.Fatalf("an unconfigured build refused an unsigned release: %v", err)
		}
		if !strings.Contains(out.String(), "unsigned") {
			t.Errorf("the fallback to checksum-only was silent: %q", out.String())
		}
	})

	t.Run("a configured build refuses an unsigned release", func(t *testing.T) {
		withPublicKey(t, public)
		release := &Release{TagName: "v1.0.0"}
		err := verifyRelease(context.Background(), release, sums, io.Discard)
		if !errors.Is(err, errUnsignedRelease) {
			t.Fatalf("an unsigned release was accepted: %v", err)
		}
	})

	t.Run("a valid signature passes", func(t *testing.T) {
		withPublicKey(t, public)
		server := serve(signatureFile(private, sums))
		defer server.Close()
		out := &strings.Builder{}
		release := &Release{TagName: "v1.0.0", Assets: []Asset{{Name: signatureAssetName, URL: server.URL}}}
		if err := verifyRelease(context.Background(), release, sums, out); err != nil {
			t.Fatalf("a signed release was refused: %v", err)
		}
		if !strings.Contains(out.String(), "signature verified") {
			t.Errorf("verification was silent: %q", out.String())
		}
	})

	t.Run("a replaced SHA256SUMS is refused", func(t *testing.T) {
		withPublicKey(t, public)
		server := serve(signatureFile(private, sums))
		defer server.Close()
		release := &Release{TagName: "v1.0.0", Assets: []Asset{{Name: signatureAssetName, URL: server.URL}}}
		replaced := []byte("dead  ayame-diff-v1.0.0-linux-amd64.tar.gz\n")
		if err := verifyRelease(context.Background(), release, replaced, io.Discard); err == nil {
			t.Fatal("a release whose checksums were swapped passed verification")
		}
	})

	t.Run("a signature from another key is refused", func(t *testing.T) {
		withPublicKey(t, public)
		_, other := newKeyPair(t)
		server := serve(signatureFile(other, sums))
		defer server.Close()
		release := &Release{TagName: "v1.0.0", Assets: []Asset{{Name: signatureAssetName, URL: server.URL}}}
		if err := verifyRelease(context.Background(), release, sums, io.Discard); err == nil {
			t.Fatal("a release signed by another key passed verification")
		}
	})
}
