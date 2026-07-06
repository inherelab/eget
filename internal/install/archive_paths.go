package install

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func archiveChildPath(parent, name string) (string, bool) {
	parent = archivePathForCompare(parent)
	name = archivePathForCompare(name)
	if name == parent {
		return "", true
	}
	prefix := strings.TrimRight(parent, "/") + "/"
	if !strings.HasPrefix(name, prefix) {
		return "", false
	}
	return strings.TrimPrefix(name, prefix), true
}

func archivePathForCompare(name string) string {
	return path.Clean(strings.ReplaceAll(name, `\`, "/"))
}

func safeArchiveRelativePath(name string) (string, error) {
	name, err := archiveMemberPathForWrite(name)
	if err != nil {
		return "", err
	}
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || cleanName == ".." || filepath.VolumeName(cleanName) != "" {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return cleanName, nil
}

func safeArchiveOutputPath(output, name string) (string, error) {
	if output == "" {
		output = "."
	}
	cleanName, err := safeArchiveRelativePath(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(output, cleanName), nil
}

func archiveMemberPathForWrite(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	if archivePathHasWindowsVolume(name) || strings.HasPrefix(name, "//") {
		return "", fmt.Errorf("unsafe archive path %q", archivePathForCompare(name))
	}
	name = strings.TrimLeft(name, "/")
	name = path.Clean(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || archivePathHasWindowsVolume(name) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return name, nil
}

func archivePathHasWindowsVolume(name string) bool {
	return len(name) >= 2 && name[1] == ':' && (name[0] >= 'A' && name[0] <= 'Z' || name[0] >= 'a' && name[0] <= 'z')
}

func validateArchiveLinkTarget(name string) error {
	_, err := safeArchiveLinkTarget(name)
	return err
}

func safeArchiveLinkName(name string, typ FileType) (string, error) {
	if typ != TypeLink && typ != TypeSymlink || name == "" {
		return name, nil
	}
	if typ == TypeSymlink {
		return strings.ReplaceAll(name, `\`, "/"), nil
	}
	return safeArchiveLinkTarget(name)
}

func safeArchiveLinkTarget(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(name, "/") || archivePathHasWindowsVolume(name) {
		return "", fmt.Errorf("unsafe archive path %q", archivePathForCompare(name))
	}
	name = path.Clean(name)
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || filepath.IsAbs(cleanName) || filepath.VolumeName(cleanName) != "" {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return filepath.ToSlash(cleanName), nil
}

func safeArchiveRelativeLinkOutputPath(output, linkOutputPath, linkTarget string) (string, error) {
	linkTarget, err := safeArchiveLinkTarget(linkTarget)
	if err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(filepath.Dir(linkOutputPath), filepath.FromSlash(linkTarget)))
	if err := ensurePathWithinRoot(output, target); err != nil {
		return "", fmt.Errorf("unsafe archive path %q", linkTarget)
	}
	return target, nil
}

func archiveSymlinkTargetForOutput(output, linkOutputPath, sourceArchiveName, linkTarget string, stripComponents int) (string, bool) {
	linkTarget = strings.ReplaceAll(linkTarget, `\`, "/")
	if target, err := safeArchiveLinkTarget(linkTarget); err == nil {
		if _, err := safeArchiveRelativeLinkOutputPath(output, linkOutputPath, target); err == nil {
			return target, true
		}
		return "", false
	}
	if !strings.HasPrefix(linkTarget, "/") || archivePathHasWindowsVolume(linkTarget) {
		return "", false
	}

	sourceDir := archivePathParts(path.Dir(archivePathForCompare(sourceArchiveName)))
	targetParts := archivePathParts(strings.TrimLeft(linkTarget, "/"))
	var best []string
	bestMatch := 0
	for sourceStart := range sourceDir {
		for targetStart := range targetParts {
			match := 0
			for sourceStart+match < len(sourceDir) &&
				targetStart+match < len(targetParts) &&
				sourceDir[sourceStart+match] == targetParts[targetStart+match] {
				match++
			}
			if match >= 2 && match > bestMatch {
				bestMatch = match
				best = append(append([]string{}, sourceDir[:sourceStart]...), targetParts[targetStart:]...)
			}
		}
	}
	if len(best) == 0 {
		return "", false
	}

	stripped, ok, err := stripArchivePath(strings.Join(best, "/"), stripComponents)
	if err != nil || !ok {
		return "", false
	}
	targetOutput, err := safeArchiveOutputPath(output, stripped)
	if err != nil || filepath.Clean(targetOutput) == filepath.Clean(linkOutputPath) {
		return "", false
	}
	rel, err := filepath.Rel(filepath.Dir(linkOutputPath), targetOutput)
	if err != nil || rel == "." || rel == "" {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func archivePathParts(name string) []string {
	name = strings.Trim(archivePathForCompare(name), "/")
	if name == "." || name == "" {
		return nil
	}
	return strings.Split(name, "/")
}

func safeArchiveHardlinkOutputPath(output, sourceArchiveName, linkTarget string, stripComponents int) (string, error) {
	archiveTarget, err := resolveArchiveLinkTarget(sourceArchiveName, linkTarget)
	if err != nil {
		return "", err
	}
	if !archivePathWithinStripRoot(sourceArchiveName, archiveTarget, stripComponents) {
		return "", fmt.Errorf("unsafe archive path %q", linkTarget)
	}
	stripped, ok, err := stripArchivePath(archiveTarget, stripComponents)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("linked target %q was stripped", linkTarget)
	}
	return safeArchiveOutputPath(output, stripped)
}

func resolveArchiveLinkTarget(sourceArchiveName, linkTarget string) (string, error) {
	linkTarget, err := safeArchiveLinkTarget(linkTarget)
	if err != nil {
		return "", err
	}
	sourceArchiveName = archivePathForCompare(sourceArchiveName)
	target := linkTarget
	if archivePathHasRelativeComponent(linkTarget) {
		target = path.Clean(path.Join(path.Dir(sourceArchiveName), linkTarget))
	}
	return safeArchiveRelativePath(target)
}

func archivePathHasRelativeComponent(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == "." || part == ".." {
			return true
		}
	}
	return false
}

func archivePathWithinStripRoot(sourceArchiveName, targetArchiveName string, components int) bool {
	if components <= 0 {
		return true
	}
	source := strings.Split(archivePathForCompare(sourceArchiveName), "/")
	target := strings.Split(archivePathForCompare(targetArchiveName), "/")
	if len(source) < components || len(target) < components {
		return false
	}
	for i := 0; i < components; i++ {
		if source[i] != target[i] {
			return false
		}
	}
	return true
}

func ensurePathWithinRoot(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return err
	}
	if rel == "." || rel == "" || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("path %q outside %q", target, root)
	}
	return nil
}
