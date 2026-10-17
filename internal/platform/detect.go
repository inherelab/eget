package platform

import "regexp"

// OSSpec is the recognition data for one operating system. Regex is the match
// pattern, Anti a pattern that disqualifies a match (e.g. android for linux) and
// Priority a pattern that makes the OS a preferred match. Nil means "absent" and
// carries no meaning by itself: how anti/priority are applied stays with the
// detector.
type OSSpec struct {
	Name     string
	Regex    *regexp.Regexp
	Anti     *regexp.Regexp
	Priority *regexp.Regexp
}

// ArchSpec is the recognition data for one architecture.
type ArchSpec struct {
	Name  string
	Regex *regexp.Regexp
}

// DetectOSSpecs is the OS recognition table of install/detect. The regexes are
// kept verbatim from the original table.
var DetectOSSpecs = []OSSpec{
	{Name: "darwin", Regex: regexp.MustCompile(`(?i)(darwin|mac.?(os)?|osx)`)},
	{Name: "windows", Regex: regexp.MustCompile(`(?i)([^r]win|windows|\.exe$|\.msi$)`), Priority: regexp.MustCompile(`(?i)\.exe$`)},
	{Name: "linux", Regex: regexp.MustCompile(`(?i)(linux|ubuntu)`), Anti: regexp.MustCompile(`(?i)(android)`), Priority: regexp.MustCompile(`\.appimage$`)},
	{Name: "netbsd", Regex: regexp.MustCompile(`(?i)(netbsd)`)},
	{Name: "freebsd", Regex: regexp.MustCompile(`(?i)(freebsd)`)},
	{Name: "openbsd", Regex: regexp.MustCompile(`(?i)(openbsd)`)},
	{Name: "android", Regex: regexp.MustCompile(`(?i)(android)`)},
	{Name: "illumos", Regex: regexp.MustCompile(`(?i)(illumos)`)},
	{Name: "solaris", Regex: regexp.MustCompile(`(?i)(solaris)`)},
	{Name: "plan9", Regex: regexp.MustCompile(`(?i)(plan9)`)},
}

// DetectArchSpecs is the architecture recognition table of install/detect.
var DetectArchSpecs = []ArchSpec{
	{Name: "amd64", Regex: regexp.MustCompile(`(?i)(x64|amd64|x86(-|_)?64)`)},
	{Name: "386", Regex: regexp.MustCompile(`(?i)(x32|amd32|x86(-|_)?32|i?386)`)},
	{Name: "arm", Regex: regexp.MustCompile(`(?i)(arm32|armv6|arm\b)`)},
	{Name: "arm64", Regex: regexp.MustCompile(`(?i)(arm64|armv8|aarch64)`)},
	{Name: "riscv64", Regex: regexp.MustCompile(`(?i)(riscv64)`)},
}

// GOOSSpecIndex indexes DetectOSSpecs by name; the key set is the authoritative
// "known GOOS" set.
var GOOSSpecIndex = indexOSSpecs()

// GOARCHSpecIndex indexes DetectArchSpecs by name.
var GOARCHSpecIndex = indexArchSpecs()

func indexOSSpecs() map[string]OSSpec {
	index := make(map[string]OSSpec, len(DetectOSSpecs))
	for _, spec := range DetectOSSpecs {
		index[spec.Name] = spec
	}
	return index
}

func indexArchSpecs() map[string]ArchSpec {
	index := make(map[string]ArchSpec, len(DetectArchSpecs))
	for _, spec := range DetectArchSpecs {
		index[spec.Name] = spec
	}
	return index
}
