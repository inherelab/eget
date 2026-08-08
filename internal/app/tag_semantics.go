package app

import (
	"regexp"
	"strings"

	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
)

var versionTagPattern = regexp.MustCompile(`(?i)(^|[-_/@])v?\d+\.\d+(\.\d+)?([-.+_][0-9a-z]+)*$`)

const (
	tagPolicyLatest = "latest"
	tagPolicyTag    = "tag"
)

func trackingTag(tag string) string {
	return trackingTagWithPolicy(tag, "")
}

func trackingTagWithPolicy(tag, policy string) string {
	tag = strings.TrimSpace(tag)
	switch cleanTagPolicy(policy) {
	case tagPolicyLatest:
		return ""
	case tagPolicyTag:
		return tag
	}
	if tag == "" || isVersionTag(tag) {
		return ""
	}
	return tag
}

func tagPolicyForInstall(tag, policy string) string {
	if policy := cleanTagPolicy(policy); policy != "" {
		return policy
	}
	if strings.TrimSpace(tag) == "" {
		return tagPolicyLatest
	}
	if isVersionTag(tag) {
		return tagPolicyLatest
	}
	return tagPolicyTag
}

func installedTagPolicy(entry storepkg.Entry) string {
	if policy, ok := stringOption(entry.Options, "tag_policy"); ok {
		return cleanTagPolicy(policy)
	}
	return cleanTagPolicy(entry.TagPolicy)
}

func itemTagPolicy(item ListItem, entry storepkg.Entry) string {
	if policy := cleanTagPolicy(util.DerefString(item.Package.TagPolicy)); policy != "" {
		return policy
	}
	return installedTagPolicy(entry)
}

func cleanTagPolicy(policy string) string {
	switch strings.TrimSpace(policy) {
	case tagPolicyLatest, tagPolicyTag:
		return strings.TrimSpace(policy)
	default:
		return ""
	}
}

func isVersionTag(tag string) bool {
	return versionTagPattern.MatchString(strings.TrimSpace(tag))
}
