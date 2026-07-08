package app

import (
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	forge "github.com/inherelab/eget/internal/source/forge"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
	"github.com/inherelab/eget/internal/util"
)

func isManagedConfigTarget(target string) bool {
	switch install.DetectTargetKind(target) {
	case install.TargetRepo, install.TargetGitHubURL, install.TargetSourceForge, install.TargetForge, install.TargetTemplate, install.TargetPkgTemplate:
		return true
	default:
		return false
	}
}

func sourceVersion(tag string, sourceBacked bool) string {
	if sourceBacked {
		return tag
	}
	return ""
}

func (s Service) installResolvedTarget(runTarget, recordTarget string, opts install.Options) (RunResult, error) {
	result, err := s.Runner.Run(runTarget, opts)
	if err != nil {
		return RunResult{}, err
	}

	installMode := result.InstallMode
	if installMode == "" && opts.IsGUI && len(result.ExtractedFiles) > 0 {
		installMode = install.InstallModePortable
	}
	shouldRecord := len(result.ExtractedFiles) > 0 || installMode == install.InstallModeInstaller || installMode == install.InstallModeRunAsset
	if s.Store == nil || !shouldRecord {
		return result, nil
	}

	repo := storepkg.NormalizeRepoName(runTarget)
	tag, releaseDate := tagFromReleaseURL(result.URL), time.Time{}
	isSourceForge := sourcesf.IsTarget(repo)
	isForge := forge.IsTarget(repo)
	kind := install.DetectTargetKind(runTarget)
	isTemplate := kind == install.TargetTemplate || kind == install.TargetPkgTemplate
	if tag == "" && isTemplate {
		tag = result.Version
	}
	if tag == "" && isSourceForge {
		tag = sourcesf.VersionFromText(result.URL)
	}
	if tag == "" && isForge && opts.Tag != "" {
		tag = opts.Tag
	}
	if tag == "" && s.ReleaseInfo != nil && shouldFetchReleaseInfo(repo) {
		if gotTag, gotDate, err := s.ReleaseInfo(repo, result.URL); err == nil {
			if tag == "" {
				tag = gotTag
			}
			releaseDate = gotDate
		}
	}
	meta := RepoMetadata{}
	if shouldFetchRepoMetadata(repo) {
		meta = s.repoMetadata(repo)
	}
	if desc := s.configDescForRepo(repo); desc != "" {
		meta.Desc = desc
	}
	if meta.Homepage == "" {
		meta.Homepage = inferRepoWebURL(repo)
	}
	if meta.RepoURL == "" {
		meta.RepoURL = inferRepoWebURL(repo)
	}

	entry := storepkg.Entry{
		Repo:           repo,
		Target:         runTarget,
		InstalledAt:    s.now(),
		URL:            result.URL,
		Asset:          chooseAsset(result),
		AssetID:        result.AssetID,
		AssetSize:      result.AssetSize,
		AssetUpdatedAt: result.AssetUpdatedAt,
		AssetDigest:    result.AssetDigest,
		Desc:           meta.Desc,
		Homepage:       meta.Homepage,
		RepoURL:        meta.RepoURL,
		Tool:           result.Tool,
		ExtractedFiles: append([]string(nil), result.ExtractedFiles...),
		Options:        extractOptionsMap(opts, result.IsGUI || opts.IsGUI),
		Tag:            tag,
		Version:        sourceVersion(tag, isSourceForge || isForge || isTemplate),
		ReleaseDate:    releaseDate,
		IsGUI:          result.IsGUI || opts.IsGUI,
		InstallMode:    installMode,
	}
	if err := s.Store.Record(recordTarget, entry); err != nil {
		return RunResult{}, err
	}
	return result, nil
}

func shouldFetchReleaseInfo(repo string) bool {
	if kind := install.DetectTargetKind(repo); kind == install.TargetTemplate || kind == install.TargetPkgTemplate {
		return false
	}
	if sourcesf.IsTarget(repo) {
		return false
	}
	if forge.IsTarget(repo) {
		return true
	}
	return isGitHubRepoTarget(repo)
}

func shouldFetchRepoMetadata(repo string) bool {
	return isGitHubRepoTarget(repo)
}

func isGitHubRepoTarget(repo string) bool {
	return install.DetectTargetKind(repo) == install.TargetRepo || install.DetectTargetKind(repo) == install.TargetGitHubURL
}

func (s Service) configDescForRepo(repo string) string {
	cfg, err := s.loadConfig()
	if err != nil || cfg == nil {
		return ""
	}
	pkg := packageSectionForRepoTarget(cfg, repo)
	return util.DerefString(pkg.Desc)
}

func (s Service) repoMetadata(repo string) RepoMetadata {
	if s.RepoMetadata == nil {
		return RepoMetadata{}
	}
	meta, err := s.RepoMetadata(repo)
	if err != nil {
		return RepoMetadata{}
	}
	return meta
}

func chooseAsset(result RunResult) string {
	if result.Asset != "" {
		return result.Asset
	}
	return path.Base(result.URL)
}

func tagFromReleaseURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i+3 < len(parts); i++ {
		if parts[i] == "releases" && parts[i+1] == "download" {
			return releaseTagFromPathParts(parts[i+2 : len(parts)-1])
		}
		if parts[i] == "releases" {
			for j := i + 2; j+1 < len(parts); j++ {
				if parts[j] == "downloads" {
					return releaseTagFromPathParts(parts[i+1 : j])
				}
			}
		}
	}
	return ""
}

func releaseTagFromPathParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	raw := strings.Join(parts, "/")
	tag, err := url.PathUnescape(raw)
	if err != nil {
		return raw
	}
	return tag
}

func extractOptionsMap(opts install.Options, isGUI bool) map[string]interface{} {
	recorded := make(map[string]interface{})
	if opts.Tag != "" {
		recorded["tag"] = opts.Tag
	}
	if opts.System != "" {
		recorded["system"] = opts.System
	}
	if opts.Output != "" {
		recorded["output"] = opts.Output
	}
	if isGUI && opts.GuiTarget != "" {
		recorded["gui_target"] = opts.GuiTarget
	}
	if isGUI {
		recorded["is_gui"] = true
	}
	if opts.InstallMode != "" {
		recorded["install_mode"] = opts.InstallMode
	}
	if opts.ExtractFile != "" {
		recorded["extract_file"] = opts.ExtractFile
	}
	if opts.All {
		recorded["all"] = true
	}
	if opts.StripComponents > 0 {
		recorded["strip_components"] = opts.StripComponents
	}
	if opts.Quiet {
		recorded["quiet"] = true
	}
	if opts.DownloadOnly {
		recorded["download_only"] = true
	}
	if opts.UpgradeOnly {
		recorded["upgrade_only"] = true
	}
	if len(opts.Asset) > 0 {
		recorded["asset"] = append([]string(nil), opts.Asset...)
	}
	if len(opts.RenameFiles) > 0 {
		recorded["rename_files"] = util.CloneStringMap(opts.RenameFiles)
	}
	if opts.Hash {
		recorded["hash"] = true
	}
	if opts.Verify != "" {
		recorded["verify"] = opts.Verify
	}
	if opts.Source {
		recorded["download_source"] = true
	}
	if opts.Prerelease {
		recorded["prerelease"] = true
	}
	if opts.DisableSSL {
		recorded["disable_ssl"] = true
	}
	if opts.URLTemplate.URLTemplate != "" {
		recorded["url_template"] = opts.URLTemplate.URLTemplate
	}
	if opts.URLTemplate.LatestURL != "" {
		recorded["latest_url"] = opts.URLTemplate.LatestURL
	}
	if opts.URLTemplate.LatestFormat != "" {
		recorded["latest_format"] = opts.URLTemplate.LatestFormat
	}
	if opts.URLTemplate.LatestJSONPath != "" {
		recorded["latest_json_path"] = opts.URLTemplate.LatestJSONPath
	}
	if opts.URLTemplate.VersionRegex != "" {
		recorded["version_regex"] = opts.URLTemplate.VersionRegex
	}
	if len(opts.URLTemplate.OSMap) > 0 {
		recorded["os_map"] = util.CloneStringMap(opts.URLTemplate.OSMap)
	}
	if len(opts.URLTemplate.ArchMap) > 0 {
		recorded["arch_map"] = util.CloneStringMap(opts.URLTemplate.ArchMap)
	}
	if len(opts.URLTemplate.ExtMap) > 0 {
		recorded["ext_map"] = util.CloneStringMap(opts.URLTemplate.ExtMap)
	}
	if len(opts.URLTemplate.LibcMap) > 0 {
		recorded["libc_map"] = util.CloneStringMap(opts.URLTemplate.LibcMap)
	}
	if opts.URLTemplate.ChecksumURLTemplate != "" {
		recorded["checksum_url_template"] = opts.URLTemplate.ChecksumURLTemplate
	}
	if opts.URLTemplate.ChecksumFormat != "" {
		recorded["checksum_format"] = opts.URLTemplate.ChecksumFormat
	}
	if opts.URLTemplate.ChecksumJSONPath != "" {
		recorded["checksum_json_path"] = opts.URLTemplate.ChecksumJSONPath
	}
	if opts.URLTemplate.ChecksumRegex != "" {
		recorded["checksum_regex"] = opts.URLTemplate.ChecksumRegex
	}
	if opts.URLTemplate.InstallAction != "" {
		recorded["install_action"] = opts.URLTemplate.InstallAction
	}
	if len(opts.URLTemplate.InstallArgs) > 0 {
		recorded["install_args"] = append([]string(nil), opts.URLTemplate.InstallArgs...)
	}
	return recorded
}
