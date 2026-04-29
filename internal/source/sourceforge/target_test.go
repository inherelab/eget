package sourceforge

import (
	"strings"
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantProject string
		wantPath    string
		wantNorm    string
		wantErr     string
	}{
		{name: "project only", input: "sourceforge:winmerge", wantProject: "winmerge", wantNorm: "sourceforge:winmerge"},
		{name: "project with path", input: "sourceforge:winmerge/stable", wantProject: "winmerge", wantPath: "stable", wantNorm: "sourceforge:winmerge"},
		{name: "nested path", input: "sourceforge:winmerge/stable/2.16.44", wantProject: "winmerge", wantPath: "stable/2.16.44", wantNorm: "sourceforge:winmerge"},
		{name: "empty project", input: "sourceforge:", wantErr: "sourceforge project is required"},
		{name: "not sourceforge", input: "junegunn/fzf", wantErr: "invalid SourceForge target"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTarget(): %v", err)
			}
			if got.Project != tt.wantProject || got.Path != tt.wantPath || got.Normalized != tt.wantNorm {
				t.Fatalf("unexpected target: %#v", got)
			}
		})
	}
}
