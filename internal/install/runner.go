package install

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gookit/goutil/x/ccolor"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/util"
	pb "github.com/schollz/progressbar/v3"
)

type RunResult struct {
	URL            string
	Tool           string
	Asset          string
	ExtractedFiles []string
	IsGUI          bool
	InstallMode    string
	InstallerFile  string
}

type Runner interface {
	Run(target string, opts Options) (RunResult, error)
}

type PromptFunc func(choices []string) (int, error)
type ConfirmFunc func(file string) (bool, error)

type InstallRunner struct {
	Service                *Service
	InstalledLoad          func() (map[string]string, map[string]string, error)
	Prompt                 PromptFunc
	ConfirmLaunchInstaller ConfirmFunc
	InstallerLauncher      InstallerLauncher
	Stdout                 io.Writer
	Stderr                 io.Writer
}

func NewRunner(service *Service) *InstallRunner {
	return &InstallRunner{
		Service:                service,
		ConfirmLaunchInstaller: defaultConfirmLaunchInstaller,
		InstallerLauncher:      DefaultInstallerLauncher{},
		Stdout:                 os.Stdout,
		Stderr:                 os.Stderr,
	}
}

func (r *InstallRunner) Run(target string, opts Options) (RunResult, error) {
	if r.Service == nil {
		return RunResult{}, fmt.Errorf("install service is required")
	}

	output := r.Stderr
	if output == nil {
		output = io.Discard
	}
	if opts.Quiet {
		output = io.Discard
	}

	finder, tool, err := r.Service.SelectFinder(target, &opts)
	if err != nil {
		return RunResult{}, err
	}
	targetKind := DetectTargetKind(target)
	ccolor.Fprintf(output, "🚀 Install <info>%s</> from <info>%s</>\n", target, targetKind)
	// verbosef("target kind: %s", targetKind)
	assets, err := finder.Find()
	if err != nil {
		return RunResult{}, err
	}
	verbosef("finder assets: %d", len(assets))

	detector, err := r.Service.SelectDetector(&opts)
	if err != nil {
		return RunResult{}, err
	}

	url, candidates, err := detector.Detect(assets)
	if len(candidates) != 0 && err != nil {
		url, err = r.resolveCandidate(target, candidates)
		if err != nil {
			return RunResult{}, err
		}
	} else if len(candidates) == 0 && err != nil {
		url, err = r.resolveFallback(target, opts, err)
		if err != nil {
			return RunResult{}, err
		}
	} else if err != nil {
		return RunResult{}, err
	}
	verbosef("selected asset url: %s", url)

	if _, err := fmt.Fprintf(output, "📦 Asset %s\n", url); err != nil {
		return RunResult{}, err
	}

	body, err := r.downloadBody(url, opts)
	if err != nil {
		return RunResult{}, fmt.Errorf("%s (URL: %s)", err, url)
	}

	sumAsset := checksumAsset(url, assets)

	verifier, err := r.Service.SelectVerifier(sumAsset, &opts)
	if err != nil {
		return RunResult{}, err
	}
	verbosef("verifier: checksum_asset=%t verify_arg=%t", sumAsset != "", opts.Verify != "")
	if err := verifier.Verify(body); err != nil {
		return RunResult{}, err
	}
	if opts.Verify == "" && sumAsset != "" {
		if _, err := fmt.Fprintf(output, "Checksum verified with %s\n", path.Base(sumAsset)); err != nil {
			return RunResult{}, err
		}
	} else if opts.Verify != "" {
		ccolor.Fprintln(output, "<error>Checksum verified</>")
	}

	extractor, err := SelectExtractorAs[Extractor](r.Service, url, tool, &opts)
	if err != nil {
		return RunResult{}, err
	}
	verbosef("extractor selected for tool=%s", tool)

	bin, bins, err := extractor.Extract(body, opts.All)
	if len(bins) != 0 && err != nil && !opts.All {
		bin, opts.All, err = r.resolveExtractedFile(bins, opts)
		if err != nil {
			return RunResult{}, err
		}
	} else if err != nil && len(bins) == 0 {
		return RunResult{}, err
	}
	if len(bins) == 0 {
		bins = []ExtractedFile{bin}
	}
	selectedName := selectedFileName(url, bin)
	if opts.IsGUI {
		opts.InstallMode = DetectGUIInstallMode(true, selectedName)
	} else if DetectInstallerKind(selectedName) != InstallerKindUnknown {
		confirmed, err := r.confirmLaunchInstaller(selectedName)
		if err != nil {
			return RunResult{}, err
		}
		if !confirmed {
			return RunResult{}, fmt.Errorf("installer launch cancelled")
		}
		opts.IsGUI = true
		opts.InstallMode = InstallModeInstaller
	}

	result := RunResult{
		URL:         url,
		Tool:        tool,
		Asset:       path.Base(url),
		IsGUI:       opts.IsGUI,
		InstallMode: opts.InstallMode,
	}
	if opts.InstallMode == InstallModeInstaller {
		installerPath, err := r.materializeInstallerFile(body, url, bin, opts)
		if err != nil {
			return RunResult{}, err
		}
		result, err := r.launchGUIInstaller(installerPath, bin, opts)
		if err != nil {
			return RunResult{}, err
		}
		result.URL = url
		result.Tool = tool
		if result.Asset == "" {
			result.Asset = path.Base(url)
		}
		return result, nil
	}

	extract := func(file ExtractedFile) (string, error) {
		out, err := outputPath(file, effectiveOutput(opts), opts.All, opts.Name)
		if err != nil {
			return "", err
		}
		if err := file.Extract(out); err != nil {
			return "", err
		}
		verbosef("extract output: %s", out)
		ccolor.Fprintf(output, "✅ Extracted <info>%s</> to <cyan>%s</>\n", file.ArchiveName, out)
		return out, nil
	}

	if opts.All {
		for _, file := range bins {
			out, err := extract(file)
			if err != nil {
				return RunResult{}, err
			}
			if out != "-" {
				result.ExtractedFiles = append(result.ExtractedFiles, out)
			}
		}
	} else {
		out, err := extract(bin)
		if err != nil {
			return RunResult{}, err
		}
		if out != "-" {
			result.ExtractedFiles = append(result.ExtractedFiles, out)
		}
	}

	return result, nil
}

func selectedFileName(url string, file ExtractedFile) string {
	if file.ArchiveName != "" {
		return file.ArchiveName
	}
	if file.Name != "" {
		return file.Name
	}
	return path.Base(url)
}

func (r *InstallRunner) confirmLaunchInstaller(file string) (bool, error) {
	confirm := r.ConfirmLaunchInstaller
	if confirm == nil {
		confirm = defaultConfirmLaunchInstaller
	}
	return confirm(filepath.Base(file))
}

func defaultConfirmLaunchInstaller(file string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s looks like a GUI installer. Launch it now? [y/N]: ", file)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		if err == io.EOF && strings.TrimSpace(answer) == "" {
			return false, nil
		}
		if err != io.EOF {
			return false, err
		}
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func effectiveOutput(opts Options) string {
	if opts.IsGUI && opts.InstallMode == InstallModePortable && !opts.OutputExplicit && opts.GuiTarget != "" {
		return opts.GuiTarget
	}
	return opts.Output
}

func (r *InstallRunner) launchGUIInstaller(path string, file ExtractedFile, opts Options) (RunResult, error) {
	kind := DetectInstallerKind(file.ArchiveName)
	if kind == InstallerKindUnknown {
		kind = DetectInstallerKind(file.Name)
	}
	launcher := r.InstallerLauncher
	if launcher == nil {
		launcher = DefaultInstallerLauncher{}
	}
	if err := launcher.LaunchInstaller(path, kind); err != nil {
		return RunResult{}, err
	}
	return RunResult{
		Asset:         filepath.Base(path),
		IsGUI:         true,
		InstallMode:   InstallModeInstaller,
		InstallerFile: path,
	}, nil
}

func (r *InstallRunner) materializeInstallerFile(body []byte, url string, file ExtractedFile, opts Options) (string, error) {
	if IsLocalFile(url) {
		return url, nil
	}
	cachePath := CacheFilePath(opts.CacheDir, url)
	if cachePath != "" && filepath.Base(cachePath) == filepath.Base(url) {
		if _, err := os.Stat(cachePath); err == nil {
			return cachePath, nil
		}
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(cachePath, body, 0o644); err != nil {
			return "", err
		}
		return cachePath, nil
	}

	target := installerMaterializePath(opts, file)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if file.Extract != nil {
		if err := file.Extract(target); err != nil {
			return "", err
		}
		return target, nil
	}
	if err := os.WriteFile(target, body, 0o755); err != nil {
		return "", err
	}
	return target, nil
}

func installerMaterializePath(opts Options, file ExtractedFile) string {
	dir := opts.CacheDir
	if dir == "" {
		dir = os.TempDir()
	}
	name := file.Name
	if name == "" {
		name = file.ArchiveName
	}
	if name == "" {
		name = "installer"
	}
	return filepath.Join(dir, "installers", filepath.Base(name))
}

func (r *InstallRunner) downloadBody(url string, opts Options) ([]byte, error) {
	cachePath := CacheFilePath(opts.CacheDir, url)
	output := r.Stderr
	if output == nil || opts.Quiet {
		output = io.Discard
	}
	if cachePath != "" && !IsLocalFile(url) {
		if data, err := os.ReadFile(cachePath); err == nil {
			ccolor.Fprintf(output, " - Using cached file <cyan>%s</>\n", filepath.Base(cachePath))
			return data, nil
		}
	}

	buf := &bytes.Buffer{}
	err := Download(url, buf, func(size int64) *pb.ProgressBar {
		pbout := r.Stderr
		if pbout == nil || opts.Quiet {
			pbout = io.Discard
		}
		return pb.NewOptions64(size,
			pb.OptionSetWriter(pbout),
			pb.OptionShowBytes(true),
			pb.OptionSetWidth(10),
			pb.OptionThrottle(65*time.Millisecond),
			pb.OptionShowCount(),
			pb.OptionSpinnerType(14),
			pb.OptionFullWidth(),
			pb.OptionSetDescription("⬇️ Downloading"),
			pb.OptionOnCompletion(func() {
				fmt.Fprint(pbout, "\n")
			}),
			pb.OptionSetTheme(pb.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}))
	}, opts)
	if err != nil {
		return nil, err
	}

	body := buf.Bytes()
	if cachePath != "" && !IsLocalFile(url) {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err == nil {
			_ = os.WriteFile(cachePath, body, 0o644)
		}
	}
	return body, nil
}

func (r *InstallRunner) resolveCandidate(target string, candidates []string) (string, error) {
	previousAssets, _, _ := r.loadInstalled()
	if previous := previousAssets[storepkg.NormalizeRepoName(target)]; previous != "" {
		for _, candidate := range candidates {
			if path.Base(candidate) == previous {
				if r.Stderr != nil {
					ccolor.Fprintf(r.Stderr, "<yellow>Warning: using previous selection '%s' as fallback</>\n", previous)
				}
				return candidate, nil
			}
		}
	}

	if r.Prompt == nil {
		return "", fmt.Errorf("%d candidates found for asset chain", len(candidates))
	}

	choices := make([]string, len(candidates))
	for i, candidate := range candidates {
		choices[i] = path.Base(candidate)
	}
	choice, err := r.Prompt(choices)
	if err != nil {
		return "", err
	}
	if choice < 0 || choice >= len(candidates) {
		return "", fmt.Errorf("selection %d is out of bounds", choice)
	}
	return candidates[choice], nil
}

func (r *InstallRunner) resolveFallback(target string, opts Options, original error) (string, error) {
	_, previousURLs, _ := r.loadInstalled()
	repoKey := storepkg.NormalizeRepoName(target)
	if previousURL := previousURLs[repoKey]; previousURL != "" {
		currentTag := opts.Tag
		if currentTag == "" {
			currentTag = "latest"
		}
		fallback := replaceTagInURL(previousURL, currentTag)
		if r.Stderr != nil {
			ccolor.Fprintf(r.Stderr, "<yellow>Warning: no assets matched current filters, using fallback asset '%s' from previous installation</>\n", path.Base(fallback))
		}
		return fallback, nil
	}
	return "", original
}

func (r *InstallRunner) resolveExtractedFile(candidates []ExtractedFile, opts Options) (ExtractedFile, bool, error) {
	goos, goarch := selectionPlatform(opts)
	if selected, ok := autoSelectExtractedFile(candidates, goos, goarch); ok {
		if r.Stderr != nil {
			ccolor.Fprintf(r.Stderr, "🪄 <yellow>Auto-selected extracted file '%s' for %s/%s</>\n", selected.ArchiveName, goos, goarch)
		}
		return selected, false, nil
	}

	if r.Prompt == nil {
		return ExtractedFile{}, false, fmt.Errorf("%d candidates for target found", len(candidates))
	}
	choices := make([]string, len(candidates)+1)
	for i := range candidates {
		choices[i] = candidates[i].String()
	}
	choices[len(candidates)] = "all"
	choice, err := r.Prompt(choices)
	if err != nil {
		return ExtractedFile{}, false, err
	}
	if choice == len(candidates) {
		return ExtractedFile{}, true, nil
	}
	if choice < 0 || choice >= len(candidates) {
		return ExtractedFile{}, false, fmt.Errorf("selection %d is out of bounds", choice)
	}
	return candidates[choice], false, nil
}

func selectionPlatform(opts Options) (string, string) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if opts.System == "" {
		return goos, goarch
	}
	parts := strings.SplitN(opts.System, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1]
	}
	return goos, goarch
}

func autoSelectExtractedFile(candidates []ExtractedFile, goos, goarch string) (ExtractedFile, bool) {
	if len(candidates) == 0 {
		return ExtractedFile{}, false
	}
	if strings.EqualFold(goos, "windows") {
		if selected, ok := autoSelectOnlyWindowsExecutable(candidates); ok {
			return selected, true
		}
	}
	patterns := archSelectionPatterns(goarch)
	if len(patterns) == 0 {
		return ExtractedFile{}, false
	}

	matches := make([]ExtractedFile, 0, len(candidates))
	for _, candidate := range candidates {
		name := util.NormalizeSlashesLower(candidate.ArchiveName)
		for _, pattern := range patterns {
			if pattern.MatchString(name) {
				matches = append(matches, candidate)
				break
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return ExtractedFile{}, false
}

func autoSelectOnlyWindowsExecutable(candidates []ExtractedFile) (ExtractedFile, bool) {
	var selected ExtractedFile
	count := 0
	for _, candidate := range candidates {
		if strings.EqualFold(filepath.Ext(candidate.ArchiveName), ".exe") {
			selected = candidate
			count++
		}
	}
	return selected, count == 1
}

func archSelectionPatterns(goarch string) []*regexp.Regexp {
	switch strings.ToLower(goarch) {
	case "amd64":
		return compileArchPatterns(`(^|/)(x64|amd64|x86_64)(/|$)`)
	case "386":
		return compileArchPatterns(`(^|/)(x86|i386|386)(/|$)`)
	case "arm64":
		return compileArchPatterns(`(^|/)(arm64|aarch64)(/|$)`)
	case "arm":
		return compileArchPatterns(`(^|/)(arm32|armv6|armv7|arm)(/|$)`)
	default:
		return nil
	}
}

func compileArchPatterns(exprs ...string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(exprs))
	for _, expr := range exprs {
		patterns = append(patterns, regexp.MustCompile(expr))
	}
	return patterns
}

func (r *InstallRunner) loadInstalled() (map[string]string, map[string]string, error) {
	if r.InstalledLoad == nil {
		return map[string]string{}, map[string]string{}, nil
	}
	return r.InstalledLoad()
}

func checksumAsset(asset string, assets []string) string {
	for _, candidate := range assets {
		if candidate == asset+".sha256sum" || candidate == asset+".sha256" {
			return candidate
		}
	}
	return ""
}

func replaceTagInURL(url, newTag string) string {
	parts := strings.Split(url, "/")
	if len(parts) < 8 || parts[2] != "github.com" {
		return url
	}
	if len(parts) >= 8 && parts[5] == "releases" && parts[6] == "download" {
		parts[7] = newTag
		return strings.Join(parts, "/")
	}
	return url
}

func outputPath(file ExtractedFile, output string, all bool, preferredName string) (string, error) {
	mode := file.Mode()
	out := resolvedOutputName(file.Name, mode, preferredName)
	if all && output != "-" && file.Name != "" {
		var err error
		out, err = safeArchiveOutputPath(output, file.Name)
		if err != nil {
			return "", err
		}
	}
	if output == "-" {
		return "-", nil
	}
	if all && output != "" && file.Name != "" {
		return out, nil
	}
	if output != "" && all {
		os.MkdirAll(output, 0o755)
		return filepath.Join(output, out), nil
	}
	if output != "" && util.IsDirectory(output) {
		return filepath.Join(output, out), nil
	}
	if output != "" {
		out = output
	}
	if os.Getenv("EGET_BIN") != "" && !strings.ContainsRune(out, os.PathSeparator) && mode&0o111 != 0 && !file.Dir {
		return filepath.Join(os.Getenv("EGET_BIN"), out), nil
	}
	return out, nil
}

func safeArchiveOutputPath(output, name string) (string, error) {
	if output == "" {
		output = "."
	}
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || cleanName == ".." || filepath.VolumeName(cleanName) != "" {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return filepath.Join(output, cleanName), nil
}

func resolvedOutputName(name string, mode os.FileMode, preferredName string) string {
	base := filepath.Base(name)
	if preferredName != "" {
		return applyPreferredName(base, preferredName)
	}
	if !isExec(base, mode) {
		return base
	}
	return heuristicExecutableName(base)
}

func applyPreferredName(originalName, preferredName string) string {
	ext := executableSuffix(originalName)
	if preferredName == "" {
		return originalName
	}
	if filepath.Ext(preferredName) != "" || ext == "" {
		return preferredName
	}
	return preferredName + ext
}

func heuristicExecutableName(name string) string {
	ext := executableSuffix(name)
	base := strings.TrimSuffix(name, ext)
	patterns := []string{
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)$`,
		`(?i)[-_.](amd64|x86_64|x64|386|x86|i386|arm64|aarch64|armv?6|armv?7|arm|riscv64)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
		`(?i)[-_.](windows|darwin|linux|freebsd|openbsd|netbsd|android|illumos|solaris|plan9|macos|osx)$`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if trimmed := re.ReplaceAllString(base, ""); trimmed != "" && trimmed != base {
			return trimmed + ext
		}
	}
	return name
}

func executableSuffix(name string) string {
	switch {
	case strings.HasSuffix(strings.ToLower(name), ".exe"):
		return ".exe"
	case strings.HasSuffix(strings.ToLower(name), ".appimage"):
		return ".appimage"
	default:
		return ""
	}
}
