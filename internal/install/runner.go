package install

import (
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/source/urltemplate"
)

type RunResult struct {
	URL            string
	Tool           string
	Asset          string
	AssetID        int64
	AssetSize      int64
	AssetUpdatedAt time.Time
	AssetDigest    string
	ExtractedFiles []string
	IsGUI          bool
	InstallMode    string
	InstallerFile  string
	Version        string
}

type Runner interface {
	Run(target string, opts Options) (RunResult, error)
}

type versionedFinder interface {
	ReleaseVersion() string
}

type assetMetadataFinder interface {
	AssetMetadata(url string) (id, size int64, updatedAt time.Time, digest string, ok bool)
}

type PromptFunc func(title, filterPrompt string, choices []string) (int, error)
type ConfirmFunc func(file string) (bool, error)

type InstallRunner struct {
	Service                *Service
	InstalledLoad          func() (map[string]string, map[string]string, error)
	Prompt                 PromptFunc
	ConfirmLaunchInstaller ConfirmFunc
	InstallerLauncher      InstallerLauncher
	AssetRunner            func(path string, args []string, stdout, stderr io.Writer) error
	Stdout                 io.Writer
	Stderr                 io.Writer
}

type directAllExtractorWithOptions interface {
	ExtractAllToWithOptions([]byte, string, ArchiveExtractOptions) ([]string, error)
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

	output := r.Stdout
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
	if opts.CacheName == "" {
		opts.CacheName = tool
	}
	targetKind := DetectTargetKind(target)
	ccolor.Fprintf(output, "🚀 %s <info>%s</> from <info>%s</>%s\n", operationDisplayName(opts.Operation), target, TargetKindDisplayName(targetKind), versionDisplaySuffix(opts))
	// verbosef("target kind: %s", targetKind)
	assets, err := finder.Find()
	if err != nil {
		return RunResult{}, err
	}
	if templateFinder, ok := finder.(*urltemplate.Finder); ok {
		opts.URLTemplate.ResolvedVersion = templateFinder.Version
		opts.URLTemplate.ResolvedVars = templateFinder.Vars()
		opts.CacheVersion = templateFinder.Version
	}
	verbosef("finder assets: %d", len(assets))

	detector, err := r.Service.SelectDetector(&opts)
	if err != nil {
		return RunResult{}, err
	}

	url, candidates, err := detector.Detect(assets)
	if len(candidates) != 0 && err != nil {
		url, err = r.resolveCandidate(target, candidates, opts, promptReleaseVersion(finder))
		if err != nil {
			return RunResult{}, err
		}
	} else if len(candidates) == 0 && err != nil {
		fallbackURL, fallbackAssets, fallbackErr := r.resolveVersionFallback(finder, detector, opts, err)
		if fallbackErr != nil {
			return RunResult{}, fallbackErr
		}
		if fallbackURL == "" {
			return RunResult{}, err
		}
		url = fallbackURL
		assets = fallbackAssets
	} else if err != nil {
		return RunResult{}, err
	}
	verbosef("selected asset url: %s", url)
	assetID, assetSize, assetUpdatedAt, assetDigest, _ := assetMetadata(finder, url)
	withAssetMetadata := func(result RunResult) RunResult {
		result.AssetID = assetID
		result.AssetSize = assetSize
		result.AssetUpdatedAt = assetUpdatedAt
		result.AssetDigest = assetDigest
		return result
	}

	if _, err := fmt.Fprintf(output, "📦 Asset %s\n", url); err != nil {
		return RunResult{}, err
	}
	if err := validateInstallAction(opts); err != nil {
		return RunResult{}, err
	}

	downloaded, err := r.downloadBody(url, opts)
	if err != nil {
		return RunResult{}, fmt.Errorf("%s (URL: %s)", err, url)
	}

	sumAsset := checksumAsset(url, assets)

	verifier, err := r.Service.SelectVerifier(sumAsset, &opts)
	if err != nil {
		return RunResult{}, err
	}
	verbosef("verifier: checksum_asset=%t verify_arg=%t", sumAsset != "", opts.Verify != "")
	if err := verifier.Verify(downloaded.Body); err != nil {
		return RunResult{}, err
	}
	if opts.Verify == "" && sumAsset != "" {
		if _, err := fmt.Fprintf(output, "Checksum verified with %s\n", path.Base(sumAsset)); err != nil {
			return RunResult{}, err
		}
	} else if opts.Verify != "" {
		ccolor.Fprintln(output, "<error>Checksum verified</>")
	}

	if opts.DownloadOnly && opts.ExtractFile == "" && !opts.All {
		result, err := r.extractDownloadedBody(url, tool, downloaded, opts, output)
		if err != nil {
			return RunResult{}, err
		}
		return withAssetMetadata(result), nil
	}

	if opts.URLTemplate.InstallAction == InstallActionRunAsset {
		assetPath, err := r.materializeRunAsset(downloaded.Body, url, opts)
		if err != nil {
			return RunResult{}, err
		}
		ccolor.Fprintf(output, "Running installer asset: <cyan>%s</>\n", assetPath)
		if err := r.runAsset(assetPath, opts.URLTemplate.InstallArgs); err != nil {
			return RunResult{}, err
		}
		return withAssetMetadata(RunResult{
			URL:         url,
			Tool:        tool,
			Asset:       path.Base(url),
			InstallMode: InstallModeRunAsset,
			Version:     opts.URLTemplate.ResolvedVersion,
		}), nil
	}

	result, err := r.extractDownloadedBody(url, tool, downloaded, opts, output)
	if err != nil {
		return RunResult{}, err
	}
	return withAssetMetadata(result), nil
}

func assetMetadata(finder Finder, url string) (id, size int64, updatedAt time.Time, digest string, ok bool) {
	if finder == nil || url == "" {
		return 0, 0, time.Time{}, "", false
	}
	metaFinder, ok := finder.(assetMetadataFinder)
	if !ok {
		return 0, 0, time.Time{}, "", false
	}
	return metaFinder.AssetMetadata(url)
}

func operationDisplayName(operation string) string {
	switch operation {
	case OperationUpdate:
		return "Update"
	default:
		return "Install"
	}
}

func versionDisplaySuffix(opts Options) string {
	if opts.CurrentVersion != "" && opts.TargetVersion != "" {
		return fmt.Sprintf(" (<info>%s</> -> <info>%s</>)", opts.CurrentVersion, opts.TargetVersion)
	}
	if opts.TargetVersion != "" {
		return fmt.Sprintf(" (<info>%s</>)", opts.TargetVersion)
	}
	return ""
}

func (r *InstallRunner) extractDownloadedBody(url, tool string, downloaded downloadBodyResult, opts Options, output io.Writer) (RunResult, error) {
	if portableGUIArchiveExtractAll(url, opts) {
		opts.All = true
	}
	assetName := firstNonEmpty(downloaded.Filename, assetFilename(url))
	extractor, err := SelectExtractorAs[Extractor](r.Service, assetName, tool, &opts)
	if err != nil {
		return RunResult{}, err
	}
	verbosef("extractor selected for tool=%s", tool)

	if opts.All && len(opts.RenameFiles) == 0 {
		if direct, ok := extractor.(DirectAllExtractor); ok && effectiveOutput(opts) != "-" {
			result := RunResult{
				URL:         url,
				Tool:        tool,
				Asset:       assetName,
				IsGUI:       opts.IsGUI,
				InstallMode: opts.InstallMode,
			}
			paths, err := extractAllTo(direct, downloaded.Body, effectiveOutput(opts), opts.StripComponents)
			if err != nil {
				return RunResult{}, err
			}
			printExtractedPaths(output, paths)
			for _, out := range paths {
				verbosef("extract output: %s", out)
				if out != "-" {
					result.ExtractedFiles = append(result.ExtractedFiles, out)
				}
			}
			return result, nil
		}
	}

	bin, bins, err := extractor.Extract(downloaded.Body, opts.All)
	if len(bins) != 0 && err != nil && !opts.All {
		if selected, ok := autoExtractCurrentPlatformExecutables(bins, opts); ok {
			bins = selected
			opts.All = true
			err = nil
		} else {
			bin, opts.All, err = r.resolveExtractedFile(bins, opts)
			if err != nil {
				return RunResult{}, err
			}
		}
	} else if err != nil && len(bins) == 0 {
		return RunResult{}, err
	}
	if len(bins) == 0 {
		bins = []ExtractedFile{bin}
	}
	selectedName := selectedFileName(assetName, bin)
	if opts.IsGUI && opts.InstallMode == "" {
		opts.InstallMode = DetectGUIInstallMode(true, selectedName)
	} else if !opts.All && DetectInstallerKind(selectedName) != InstallerKindUnknown {
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
		Asset:       assetName,
		IsGUI:       opts.IsGUI,
		InstallMode: opts.InstallMode,
		Version:     opts.URLTemplate.ResolvedVersion,
	}
	if opts.InstallMode == InstallModeInstaller {
		installerPath, err := r.materializeInstallerFile(downloaded.Body, url, bin, opts)
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
			result.Asset = assetName
		}
		return result, nil
	}

	extract := func(file ExtractedFile, multiple bool) (string, error) {
		out, err := outputPath(file, effectiveOutput(opts), opts.All, opts.Name, opts.OutputExplicit, opts.RenameFiles)
		if err != nil {
			return "", err
		}
		if err := file.Extract(out); err != nil {
			return "", err
		}
		verbosef("extract output: %s", out)
		printExtractedFile(output, file.ArchiveName, out, multiple)
		if out != "-" && shouldApplyDownloadedModTime(file, url, opts, downloaded.ModTime) {
			if err := applyModTime(out, downloaded.ModTime); err != nil {
				return "", err
			}
		}
		return out, nil
	}

	if opts.All {
		multiple := len(bins) > 1
		if multiple {
			ccolor.Fprintln(output, "✅ Extracted files:")
		}
		for _, file := range bins {
			out, err := extract(file, multiple)
			if err != nil {
				return RunResult{}, err
			}
			if out != "-" {
				result.ExtractedFiles = append(result.ExtractedFiles, out)
			}
		}
	} else {
		out, err := extract(bin, false)
		if err != nil {
			return RunResult{}, err
		}
		if out != "-" {
			result.ExtractedFiles = append(result.ExtractedFiles, out)
		}
	}

	return result, nil
}

func printExtractedPaths(output io.Writer, paths []string) {
	if len(paths) > 1 {
		ccolor.Fprintln(output, "✅ Extracted files:")
		for _, out := range paths {
			ccolor.Fprintf(output, " - <cyan>%s</>\n", out)
		}
		return
	}
	for _, out := range paths {
		ccolor.Fprintf(output, "✅ Extracted <cyan>%s</>\n", out)
	}
}

func printExtractedFile(output io.Writer, archiveName, out string, multiple bool) {
	if multiple {
		ccolor.Fprintf(output, " - <info>%s</> to <cyan>%s</>\n", archiveName, out)
		return
	}
	ccolor.Fprintf(output, "✅ Extracted <info>%s</> to <cyan>%s</>\n", archiveName, out)
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
