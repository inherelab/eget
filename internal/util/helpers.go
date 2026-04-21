package util

import (
	"os"
	"strings"
)

func BoolPtr(value bool) *bool {
	return &value
}

func StringPtr(value string) *string {
	return &value
}

func DerefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func ContainsPathSeparator(value string) bool {
	for _, ch := range value {
		if ch == os.PathSeparator || ch == '/' || ch == '\\' {
			return true
		}
	}
	return false
}

func NormalizeSlashesLower(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
}
