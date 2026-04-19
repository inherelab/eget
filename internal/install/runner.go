package install

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

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
	assets, err := finder.Find()
	if err != nil {
		return RunResult{}, err
	}

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

	if _, err := fmt.Fprintf(output, "%s\n", url); err != nil {
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
	if err := verifier.Verify(body); err != nil {
		return RunResult{}, err
	}
	if opts.Verify == "" && sumAsset != "" {
		if _, err := fmt.Fprintf(output, "Checksum verified with %s\n", path.Base(sumAsset)); err != nil {
			return RunResult{}, err
		}
	} else if opts.Verify != "" {
		if _, err := fmt.Fprintln(output, "Checksum verified"); err != nil {
			return RunResult{}, err
		}
	}

	extractor, err := SelectExtractorAs[Extractor](r.Service, url, tool, &opts)
	if err != nil {
		return RunResult{}, err
	}

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
		out := outputPath(file, opts.Output, opts.All)
		if err := file.Extract(out); err != nil {
			return "", err
		}
		if _, err := fmt.Fprintf(output, "Extracted `%s` to `%s`\n", file.ArchiveName, out); err != nil {
			return "", err
		}
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
	if cachePath != "" && !IsLocalFile(url) {
		if data, err := os.ReadFile(cachePath); err == nil {
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

func outputPath(file ExtractedFile, output string, all bool) string {
	mode := file.Mode()
	out := filepath.Base(file.Name)
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

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
