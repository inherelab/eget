package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectTargetKind(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "archive.tar.gz")
	if err := os.WriteFile(localFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	tests := []struct {
		name   string
		target string
		want   TargetKind
	}{
		{name: "repo target", target: "inhere/markview", want: TargetRepo},
		{name: "sourceforge target", target: "sourceforge:winmerge", want: TargetSourceForge},
		{name: "sourceforge target with path", target: "sourceforge:winmerge/stable", want: TargetSourceForge},
		{name: "github url", target: "https://github.com/inhere/markview", want: TargetGitHubURL},
		{name: "direct url", target: "https://example.com/download/tool.tar.gz", want: TargetDirectURL},
		{name: "local file", target: localFile, want: TargetLocalFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectTargetKind(tt.target); got != tt.want {
				t.Fatalf("DetectTargetKind(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestTargetHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "tool.zip")
	if err := os.WriteFile(localFile, []byte("zip"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if !IsURL("https://example.com/tool.tar.gz") {
		t.Fatal("expected direct URL to be recognized")
	}
	if !IsGitHubURL("https://github.com/inhere/markview") {
		t.Fatal("expected GitHub URL to be recognized")
	}
	if IsGitHubURL("https://example.com/inhere/markview") {
		t.Fatal("expected non-GitHub URL to be rejected")
	}
	if !IsLocalFile(localFile) {
		t.Fatal("expected local file to be recognized")
	}
}
