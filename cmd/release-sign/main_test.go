package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The signing tool and the updater must agree byte for byte, so the round trip
// is what this checks: a key from keygen signs a file the verifier accepts.
func TestKeygenSignVerifyRoundTrip(t *testing.T) {
	out := &strings.Builder{}
	if err := run([]string{"keygen"}, out); err != nil {
		t.Fatal(err)
	}
	public, private := keysFromKeygen(t, out.String())
	t.Setenv(keyEnv, private)

	dir := t.TempDir()
	sums := filepath.Join(dir, "SHA256SUMS")
	signature := sums + ".sig"
	if err := os.WriteFile(sums, []byte("abc  ayame-diff-v1.0.0-linux-amd64.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"sign", "-in", sums, "-out", signature}, io.Discard); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(signature)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "# ed25519 signature over SHA256SUMS\n") {
		t.Errorf("the signature file lost its header: %q", body)
	}
	if err := run([]string{"verify", "-in", sums, "-sig", signature, "-key", public}, io.Discard); err != nil {
		t.Fatalf("the tool refused its own signature: %v", err)
	}

	// A changed file must not verify against the old signature.
	if err := os.WriteFile(sums, []byte("dead  ayame-diff-v1.0.0-linux-amd64.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify", "-in", sums, "-sig", signature, "-key", public}, io.Discard); err == nil {
		t.Fatal("a modified file verified against the old signature")
	}
}

func TestSignRefusesAnUnusableKey(t *testing.T) {
	dir := t.TempDir()
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(sums, []byte("abc  file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, key := range map[string]string{
		"missing":    "",
		"not base64": "not base64!!",
		"wrong size": base64.StdEncoding.EncodeToString([]byte("short")),
	} {
		t.Setenv(keyEnv, key)
		err := run([]string{"sign", "-in", sums, "-out", filepath.Join(dir, "out.sig")}, io.Discard)
		if err == nil {
			t.Errorf("a %s key produced a signature", name)
		}
	}
}

func TestUnknownCommandsAndMissingFlagsFail(t *testing.T) {
	if err := run(nil, io.Discard); err == nil {
		t.Error("no arguments was accepted")
	}
	if err := run([]string{"nope"}, io.Discard); err == nil {
		t.Error("an unknown command was accepted")
	}
	if err := run([]string{"sign"}, io.Discard); err == nil {
		t.Error("sign without -in/-out was accepted")
	}
	if err := run([]string{"verify"}, io.Discard); err == nil {
		t.Error("verify without arguments was accepted")
	}
}

// keysFromkeygen pulls the two base64 keys out of the human-readable output,
// which also checks that output stays parseable enough to copy from.
func keysFromKeygen(t *testing.T, output string) (public string, private string) {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		encoded := fields[len(fields)-1]
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		switch len(raw) {
		case ed25519.PublicKeySize:
			public = encoded
		case ed25519.PrivateKeySize:
			private = encoded
		}
	}
	if public == "" || private == "" {
		t.Fatalf("keygen output holds no usable key pair: %q", output)
	}
	return public, private
}
