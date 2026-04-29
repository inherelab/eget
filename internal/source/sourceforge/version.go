package sourceforge

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`\d+(?:\.\d+)+`)

func VersionFromText(text string) string {
	return versionPattern.FindString(text)
}

func LatestVersionFile(files []File) (File, bool) {
	var latest File
	var latestVersion string
	found := false

	for _, file := range files {
		version := VersionFromText(file.Name)
		if version == "" && file.FullPath != "" {
			version = VersionFromText(path.Base(strings.Trim(file.FullPath, "/")))
		}
		if version == "" {
			continue
		}
		if !found || compareVersion(version, latestVersion) > 0 {
			latest = file
			latestVersion = version
			found = true
		}
	}

	return latest, found
}

func compareVersion(left, right string) int {
	leftParts := splitVersion(left)
	rightParts := splitVersion(right)
	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}

	for i := 0; i < maxLen; i++ {
		leftPart, rightPart := 0, 0
		if i < len(leftParts) {
			leftPart = leftParts[i]
		}
		if i < len(rightParts) {
			rightPart = rightParts[i]
		}
		if leftPart > rightPart {
			return 1
		}
		if leftPart < rightPart {
			return -1
		}
	}
	return 0
}

func splitVersion(version string) []int {
	parts := strings.Split(version, ".")
	nums := make([]int, 0, len(parts))
	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			num = 0
		}
		nums = append(nums, num)
	}
	return nums
}
