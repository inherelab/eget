package forge

import (
	"fmt"
	"strings"
)

type Provider string

const (
	ProviderGitLab  Provider = "gitlab"
	ProviderGitea   Provider = "gitea"
	ProviderForgejo Provider = "forgejo"
)

type Target struct {
	Provider   Provider
	Host       string
	Namespace  string
	Project    string
	Normalized string
}

func IsTarget(value string) bool {
	return strings.HasPrefix(value, string(ProviderGitLab)+":") ||
		strings.HasPrefix(value, string(ProviderGitea)+":") ||
		strings.HasPrefix(value, string(ProviderForgejo)+":")
}

func ParseTarget(value string) (Target, error) {
	providerText, rest, ok := strings.Cut(value, ":")
	if !ok || !IsTarget(value) {
		return Target{}, fmt.Errorf("invalid forge target %q", value)
	}

	provider := Provider(providerText)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return Target{}, fmt.Errorf("%s project is required", provider)
	}

	parts := splitPath(rest)
	switch provider {
	case ProviderGitLab:
		return parseGitLab(parts)
	case ProviderGitea, ProviderForgejo:
		return parseGitea(provider, parts)
	default:
		return Target{}, fmt.Errorf("unsupported forge provider %q", provider)
	}
}

func parseGitLab(parts []string) (Target, error) {
	if len(parts) < 2 {
		return Target{}, fmt.Errorf("gitlab project is required")
	}

	host := "gitlab.com"
	namespaceParts := parts[:len(parts)-1]
	project := parts[len(parts)-1]
	if len(parts) >= 3 {
		host = parts[0]
		namespaceParts = parts[1 : len(parts)-1]
	}
	return buildTarget(ProviderGitLab, host, namespaceParts, project)
}

func parseGitea(provider Provider, parts []string) (Target, error) {
	if len(parts) < 3 {
		return Target{}, fmt.Errorf("%s host is required", provider)
	}

	host := parts[0]
	namespaceParts := parts[1 : len(parts)-1]
	project := parts[len(parts)-1]
	return buildTarget(provider, host, namespaceParts, project)
}

func buildTarget(provider Provider, host string, namespaceParts []string, project string) (Target, error) {
	host = strings.TrimSpace(host)
	project = strings.TrimSpace(project)
	if host == "" {
		return Target{}, fmt.Errorf("%s host is required", provider)
	}
	if project == "" {
		return Target{}, fmt.Errorf("%s project is required", provider)
	}

	namespace := strings.Join(namespaceParts, "/")
	if namespace == "" {
		return Target{}, fmt.Errorf("%s namespace is required", provider)
	}

	normalized := string(provider) + ":" + host + "/" + namespace + "/" + project
	return Target{Provider: provider, Host: host, Namespace: namespace, Project: project, Normalized: normalized}, nil
}

func splitPath(value string) []string {
	raw := strings.Split(value, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
