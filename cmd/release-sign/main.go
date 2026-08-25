// Command release-sign generates the release signing key and signs release
// metadata with it (#148). It is standard-library only, so the release workflow
// can run it with `go run` on any runner.
//
//	go run ./cmd/release-sign keygen
//	go run ./cmd/release-sign sign -in release/SHA256SUMS -out release/SHA256SUMS.sig
//	go run ./cmd/release-sign verify -in release/SHA256SUMS -sig release/SHA256SUMS.sig -key <base64>
//
// The private key is read from AYAME_RELEASE_SIGNING_KEY so it never reaches a
// command line, where it would be visible to every process on the machine.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ayame-editor/ayame-diff/internal/selfupdate"
)

const keyEnv = "AYAME_RELEASE_SIGNING_KEY"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: release-sign <keygen|sign|verify> [options]")
	}
	switch args[0] {
	case "keygen":
		return keygen(stdout)
	case "sign":
		return sign(args[1:], stdout)
	case "verify":
		return verify(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func keygen(stdout io.Writer) error {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "public  (compile into internal/selfupdate/verify.go): %s\n", base64.StdEncoding.EncodeToString(public))
	fmt.Fprintf(stdout, "private (store as the %s repository secret):          %s\n", keyEnv, base64.StdEncoding.EncodeToString(private))
	return nil
}

func sign(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("release-sign sign", flag.ContinueOnError)
	in := fs.String("in", "", "file to sign")
	out := fs.String("out", "", "detached signature to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *out == "" {
		return errors.New("sign requires -in and -out")
	}
	private, err := privateKeyFromEnv()
	if err != nil {
		return err
	}
	message, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	body := selfupdate.FormatDetachedSignature(trimDir(*in), ed25519.Sign(private, message))
	if err := os.WriteFile(*out, body, 0o644); err != nil {
		return err
	}
	// Check the file the way the updater will, so a release can never publish a
	// signature its own updater would refuse.
	if err := selfupdate.VerifyDetached(message, body, base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))); err != nil {
		return fmt.Errorf("the signature just written does not verify: %w", err)
	}
	fmt.Fprintf(stdout, "signed %s -> %s\n", *in, *out)
	return nil
}

func verify(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("release-sign verify", flag.ContinueOnError)
	in := fs.String("in", "", "signed file")
	sig := fs.String("sig", "", "detached signature")
	key := fs.String("key", "", "base64 public key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *sig == "" || *key == "" {
		return errors.New("verify requires -in, -sig and -key")
	}
	message, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	signatureFile, err := os.ReadFile(*sig)
	if err != nil {
		return err
	}
	if err := selfupdate.VerifyDetached(message, signatureFile, *key); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "signature verified")
	return nil
}

func privateKeyFromEnv() (ed25519.PrivateKey, error) {
	encoded := strings.TrimSpace(os.Getenv(keyEnv))
	if encoded == "" {
		return nil, fmt.Errorf("%s is not set", keyEnv)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%s is not valid base64: %w", keyEnv, err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%s is %d bytes, want %d", keyEnv, len(raw), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(raw), nil
}

func trimDir(path string) string {
	if index := strings.LastIndexAny(path, `/\`); index >= 0 {
		return path[index+1:]
	}
	return path
}
