package sourceforge

import (
	"fmt"
	"strings"
)

const Prefix = "sourceforge:"

type Target struct {
	Project    string
	Path       string
	Normalized string
}

func IsTarget(value string) bool {
	return strings.HasPrefix(value, Prefix)
}

func ParseTarget(value string) (Target, error) {
	if !IsTarget(value) {
		return Target{}, fmt.Errorf("invalid SourceForge target %q", value)
	}
	rest := strings.TrimPrefix(value, Prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return Target{}, fmt.Errorf("sourceforge project is required")
	}
	project, sourcePath, _ := strings.Cut(rest, "/")
	if project == "" {
		return Target{}, fmt.Errorf("sourceforge project is required")
	}
	sourcePath = strings.Trim(sourcePath, "/")
	return Target{Project: project, Path: sourcePath, Normalized: Prefix + project}, nil
}
