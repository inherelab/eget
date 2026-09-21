package install

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestSha256AssetVerifierRejectsInvalidHex(t *testing.T) {
	verifier := &sha256AssetVerifier{
		AssetURL: "https://example.com/tool.sha256",
		Getter: fakeHTTPGetterFunc(func(url string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("not-a-sha256")),
			}, nil
		}),
	}

	err := verifier.Verify(bytes.NewReader([]byte("test")))
	if err == nil || !strings.Contains(err.Error(), "invalid checksum") {
		t.Fatalf("expected invalid checksum error, got %v", err)
	}
}

func TestSha256AssetVerifierAcceptsCertutilOutput(t *testing.T) {
	verifier := &sha256AssetVerifier{
		AssetURL: "https://example.com/tool.zip.sha256",
		Getter: fakeHTTPGetterFunc(func(url string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`SHA256 hash of tool.zip:
9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
CertUtil: -hashfile command completed successfully.`)),
			}, nil
		}),
	}

	assert.NoErr(t, verifier.Verify(bytes.NewReader([]byte("test"))))
}
