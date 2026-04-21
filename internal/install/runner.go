package install

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gookit/goutil/x/ccolor"
	storepkg "github.com/inherelab/eget/internal/installed"
	pb "github.com/schollz/progressbar/v3"
)

type RunResult struct {
	URL            string
	Tool           string
	Asset          string
	ExtractedFiles []string
}

type Runner interface {
	Run(target string, opts Options) (RunResult, error)
}

type PromptFunc func(choices []string) (int, error)

type InstallRunner struct {
	Service       *Service
	InstalledLoad func() (map[string]string, map[string]string, error)
	Prompt        PromptFunc
	Stdout        io.Writer
	Stderr        io.Writer
}

func NewRunner(service *Service) *InstallRunner {
	return &InstallRunner{
		Service: service,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
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
	verbosef("target kind: %s", DetectTargetKind(target))
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

	if _, err := fmt.Fprintf(output, "Asset %s\n", url); err != nil {
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
		bin, opts.All, err = r.resolveExtractedFile(bins)
		if err != nil {
			return RunResult{}, err
		}
	} else if err != nil && len(bins) == 0 {
		return RunResult{}, err
	}
	if len(bins) == 0 {
		bins = []ExtractedFile{bin}
	}

	result := RunResult{
		URL:   url,
		Tool:  tool,
		Asset: path.Base(url),
	}
	extract := func(file ExtractedFile) (string, error) {
		out := outputPath(file, opts.Output, opts.All, opts.Name)
		if err := file.Extract(out); err != nil {
			return "", err
		}
		verbosef("extract output: %s", out)
		ccolor.Fprintf(output, "Extracted <info>%s</> to <cyan>%s</>\n", file.ArchiveName, out)
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

func (r *InstallRunner) downloadBody(url string, opts Options) ([]byte, error) {
	cachePath := CacheFilePath(opts.CacheDir, url)
	output := r.Stderr
	if output == nil || opts.Quiet {
		output = io.Discard
	}
	if cachePath != "" && !IsLocalFile(url) {
		if data, err := os.ReadFile(cachePath); err == nil {
			ccolor.Fprintf(output, "Using cached file <cyan>%s</>\n", cachePath)
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
			pb.OptionSetDescription("Downloading"),
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
					fmt.Fprintf(r.Stderr, "\033[33mUsing previous selection '%s' as fallback\033[0m\n", previous)
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
			fmt.Fprintf(r.Stderr, "\033[33mWarning: no assets matched current filters, using fallback asset '%s' from previous installation\033[0m\n", path.Base(fallback))
		}
		return fallback, nil
	}
	return "", original
}

func (r *InstallRunner) resolveExtractedFile(candidates []ExtractedFile) (ExtractedFile, bool, error) {
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

func outputPath(file ExtractedFile, output string, all bool, preferredName string) string {
	mode := file.Mode()
	out := resolvedOutputName(file.Name, mode, preferredName)
	if output == "-" {
		return "-"
	}
	if output != "" && isDirectory(output) {
		return filepath.Join(output, out)
	}
	if output != "" && all {
		os.MkdirAll(output, 0o755)
		return filepath.Join(output, out)
	}
	if output != "" {
		out = output
	}
	if os.Getenv("EGET_BIN") != "" && !strings.ContainsRune(out, os.PathSeparator) && mode&0o111 != 0 && !file.Dir {
		return filepath.Join(os.Getenv("EGET_BIN"), out)
	}
	return out
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

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
