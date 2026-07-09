package app

import (
	"regexp"
	"strings"
)

var versionTagPattern = regexp.MustCompile(`(?i)(^|[-_/])v?\d+\.\d+(\.\d+)?([-.+_][0-9a-z]+)*$`)

func trackingTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" || isVersionTag(tag) {
		return ""
	}
	return tag
}

func isVersionTag(tag string) bool {
	return versionTagPattern.MatchString(strings.TrimSpace(tag))
}
