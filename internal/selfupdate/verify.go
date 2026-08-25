package selfupdate

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Release trust (#148). A checksum taken from the same release only proves the
// download was not corrupted in transit: whoever can publish the assets can
// publish a matching SHA256SUMS. A detached signature over SHA256SUMS, checked
// against a key compiled into this binary, is what makes a tampered release
// fail instead of install.
//
// releasePublicKey is that key, base64-encoded. It is empty until a signing key
// is configured — see docs/self-update.md — and while it is empty an update
// falls back to checksum-only and says so. Once it is set, an update without a
// valid signature is refused.
const releasePublicKey = ""

// activePublicKey is what the code reads, so a test can install a key without
// rebuilding. Production always starts from the constant above.
var activePublicKey = releasePublicKey

// signatureAssetName is the detached signature published beside SHA256SUMS.
const signatureAssetName = "SHA256SUMS.sig"

// maxSignatureBytes bounds the signature download; a base64 ed25519 signature
// is under 100 bytes, so this is generous even with comments.
const maxSignatureBytes = 4 << 10

var errUnsignedRelease = errors.New("release is not signed")

// signatureRequired reports whether this build refuses an unsigned release.
func signatureRequired() bool { return activePublicKey != "" }

// parsePublicKey decodes the compiled-in key.
func parsePublicKey() (ed25519.PublicKey, error) {
	return decodePublicKey(activePublicKey)
}

func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("release public key is not valid base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("release public key is %d bytes, want %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

// FormatDetachedSignature renders the signature file the release publishes and
// the updater reads. Both sides share this one definition of the format so they
// cannot drift apart.
func FormatDetachedSignature(sourceName string, signature []byte) []byte {
	return []byte(fmt.Sprintf("# ed25519 signature over %s\n%s\n",
		sourceName, base64.StdEncoding.EncodeToString(signature)))
}

// ParseSignature reads the first meaningful line of a detached signature file.
// Blank lines and "#" comments are ignored so the file can carry a header.
func ParseSignature(body []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			return nil, fmt.Errorf("signature is not valid base64: %w", err)
		}
		if len(raw) != ed25519.SignatureSize {
			return nil, fmt.Errorf("signature is %d bytes, want %d", len(raw), ed25519.SignatureSize)
		}
		return raw, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("signature file is empty")
}

// VerifyDetached checks a detached signature over message against a base64
// public key. The release tooling uses it to check its own output before
// publishing, so a release cannot ship a signature this updater would refuse.
func VerifyDetached(message, signatureFile []byte, publicKeyBase64 string) error {
	key, err := decodePublicKey(publicKeyBase64)
	if err != nil {
		return err
	}
	signature, err := ParseSignature(signatureFile)
	if err != nil {
		return err
	}
	if !ed25519.Verify(key, message, signature) {
		return errors.New("signature does not match the given public key")
	}
	return nil
}

// verifySums checks a detached signature over the SHA256SUMS body against the
// key compiled into this binary.
func verifySums(sums, signature []byte) error {
	if err := VerifyDetached(sums, signature, activePublicKey); err != nil {
		return err
	}
	return nil
}
