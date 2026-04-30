package forge

import (
	"strings"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantProvider  Provider
		wantHost      string
		wantNamespace string
		wantProject   string
		wantNorm      string
		wantErr       string
	}{
		{name: "gitlab default host", input: "gitlab:fdroid/fdroidserver", wantProvider: ProviderGitLab, wantHost: "gitlab.com", wantNamespace: "fdroid", wantProject: "fdroidserver", wantNorm: "gitlab:gitlab.com/fdroid/fdroidserver"},
		{name: "gitlab explicit host", input: "gitlab:gitlab.gnome.org/GNOME/gtk", wantProvider: ProviderGitLab, wantHost: "gitlab.gnome.org", wantNamespace: "GNOME", wantProject: "gtk", wantNorm: "gitlab:gitlab.gnome.org/GNOME/gtk"},
		{name: "gitlab nested namespace", input: "gitlab:gitlab.example.com/group/subgroup/project", wantProvider: ProviderGitLab, wantHost: "gitlab.example.com", wantNamespace: "group/subgroup", wantProject: "project", wantNorm: "gitlab:gitlab.example.com/group/subgroup/project"},
		{name: "gitea explicit host", input: "gitea:codeberg.org/forgejo/forgejo", wantProvider: ProviderGitea, wantHost: "codeberg.org", wantNamespace: "forgejo", wantProject: "forgejo", wantNorm: "gitea:codeberg.org/forgejo/forgejo"},
		{name: "forgejo explicit host", input: "forgejo:codeberg.org/forgejo/forgejo", wantProvider: ProviderForgejo, wantHost: "codeberg.org", wantNamespace: "forgejo", wantProject: "forgejo", wantNorm: "forgejo:codeberg.org/forgejo/forgejo"},
		{name: "empty gitlab", input: "gitlab:", wantErr: "gitlab project is required"},
		{name: "gitea missing host", input: "gitea:forgejo/forgejo", wantErr: "gitea host is required"},
		{name: "not forge", input: "sourceforge:winmerge", wantErr: "invalid forge target"},
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
			assert.Eq(t, tt.wantProvider, got.Provider)
			assert.Eq(t, tt.wantHost, got.Host)
			assert.Eq(t, tt.wantNamespace, got.Namespace)
			assert.Eq(t, tt.wantProject, got.Project)
			assert.Eq(t, tt.wantNorm, got.Normalized)
		})
	}
}

func TestIsTarget(t *testing.T) {
	assert.True(t, IsTarget("gitlab:fdroid/fdroidserver"))
	assert.True(t, IsTarget("gitea:codeberg.org/forgejo/forgejo"))
	assert.True(t, IsTarget("forgejo:codeberg.org/forgejo/forgejo"))
	assert.False(t, IsTarget("sourceforge:winmerge"))
	assert.False(t, IsTarget("inhere/markview"))
}
