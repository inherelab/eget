package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	sourcegithub "github.com/inherelab/eget/internal/source/github"
)

type noVerifier struct{}

type sha256Error struct {
	Expected []byte
	Got      []byte
}

type sha256Verifier struct {
	Expected []byte
}

type sha256Printer struct{}

type sha256AssetVerifier struct {
	AssetURL string
	Getter   sourcegithub.HTTPGetter
}

func (n *noVerifier) Verify(r io.Reader) error {
	return nil
}

func (e *sha256Error) Error() string {
	return fmt.Sprintf("sha256 checksum mismatch:\nexpected: %x\ngot:      %x", e.Expected, e.Got)
}

func newSha256Verifier(expectedHex string) (*sha256Verifier, error) {
	expected, _ := hex.DecodeString(expectedHex)
	if len(expected) != sha256.Size {
		return nil, fmt.Errorf("sha256sum (%s) too small: %d bytes decoded", expectedHex, len(expectedHex))
	}
	return &sha256Verifier{Expected: expected}, nil
}

func (s *sha256Verifier) Verify(r io.Reader) error {
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return err
	}
	sum := hash.Sum(nil)
	if bytes.Equal(sum, s.Expected) {
		return nil
	}
	return &sha256Error{Expected: s.Expected, Got: sum}
}

func (s *sha256Printer) Verify(r io.Reader) error {
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return err
	}
	fmt.Printf("%x\n", hash.Sum(nil))
	return nil
}

func (s *sha256AssetVerifier) Verify(r io.Reader) error {
	if s.Getter == nil {
		return fmt.Errorf("github getter is required")
	}
	resp, err := s.Getter.Get(s.AssetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return fmt.Errorf("sha256sum is empty")
	}
	checksum := firstSHA256ChecksumField(fields)
	expected, err := hex.DecodeString(checksum)
	if err != nil {
		return fmt.Errorf("invalid checksum %q: %w", checksum, err)
	}
	if len(expected) < sha256.Size {
		return fmt.Errorf("sha256sum (%s) too small: %d bytes decoded", checksum, len(expected))
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return err
	}
	sum := hash.Sum(nil)
	if bytes.Equal(sum, expected[:sha256.Size]) {
		return nil
	}
	return &sha256Error{Expected: expected[:sha256.Size], Got: sum}
}

func firstSHA256ChecksumField(fields []string) string {
	for _, field := range fields {
		candidate := strings.Trim(field, "=:,;")
		if len(candidate) == sha256.Size*2 && isHexString(candidate) {
			return candidate
		}
	}
	return fields[0]
}

func isHexString(s string) bool {
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}
