package config

import "github.com/inherelab/eget/internal/util"

func MergeInstallOptions(global, repo, pkg Section, cli CLIOverrides) Merged {
	merged := Merged{}

	merged.ExtractAll = firstBool(cli.ExtractAll, pkg.ExtractAll, repo.ExtractAll, global.ExtractAll)
	merged.DownloadOnly = firstBool(cli.DownloadOnly, pkg.DownloadOnly, repo.DownloadOnly, global.DownloadOnly)
	merged.Source = firstBool(cli.Source, pkg.Source, repo.Source, global.Source)
	merged.Quiet = firstBool(cli.Quiet, pkg.Quiet, repo.Quiet, global.Quiet)
	merged.Prerelease = firstBool(cli.Prerelease, pkg.Prerelease, repo.Prerelease, global.Prerelease)
	merged.ShowHash = firstBool(cli.ShowHash, pkg.ShowHash, repo.ShowHash, global.ShowHash)
	merged.UpgradeOnly = firstBool(cli.UpgradeOnly, pkg.UpgradeOnly, repo.UpgradeOnly, global.UpgradeOnly)
	merged.DisableSSL = firstBool(cli.DisableSSL, pkg.DisableSSL, repo.DisableSSL, global.DisableSSL)
	merged.IsGUI = firstBool(cli.IsGUI, pkg.IsGUI, repo.IsGUI)

	merged.File = firstString(cli.File, pkg.File, repo.File, global.File)
	merged.CacheDir = firstString(cli.CacheDir, pkg.CacheDir, repo.CacheDir, global.CacheDir)
	merged.ProxyURL = firstString(cli.ProxyURL, pkg.ProxyURL, repo.ProxyURL, global.ProxyURL)
	merged.UserAgent = firstString(global.UserAgent)
	merged.GithubToken = firstString(cli.GithubToken, pkg.GithubToken, repo.GithubToken, global.GithubToken)
	merged.GuiTarget = firstString(global.GuiTarget)
	merged.Name = firstString(cli.Name, pkg.Name, repo.Name, global.Name)
	merged.SourcePath = firstString(cli.SourcePath, pkg.SourcePath, repo.SourcePath, global.SourcePath)
	merged.Sys7zPath = firstString(pkg.Sys7zPath, repo.Sys7zPath, global.Sys7zPath)
	merged.System = firstString(cli.System, pkg.System, repo.System, global.System)
	merged.Tag = firstString(cli.Tag, pkg.Tag, repo.Tag, global.Tag)
	merged.TagPolicy = firstString(cli.TagPolicy, pkg.TagPolicy, repo.TagPolicy, global.TagPolicy)
	merged.Target = firstString(cli.Target, pkg.Target, repo.Target, global.Target)
	merged.Verify = firstString(cli.Verify, pkg.Verify, repo.Verify, global.Verify)
	merged.URLTemplate = firstString(pkg.URLTemplate, repo.URLTemplate, global.URLTemplate)
	merged.LatestURL = firstString(pkg.LatestURL, repo.LatestURL, global.LatestURL)
	merged.LatestFormat = firstString(pkg.LatestFormat, repo.LatestFormat, global.LatestFormat)
	merged.LatestJSONPath = firstString(pkg.LatestJSONPath, repo.LatestJSONPath, global.LatestJSONPath)
	merged.VersionRegex = firstString(pkg.VersionRegex, repo.VersionRegex, global.VersionRegex)
	merged.ChecksumURLTemplate = firstString(pkg.ChecksumURLTemplate, repo.ChecksumURLTemplate, global.ChecksumURLTemplate)
	merged.ChecksumFormat = firstString(pkg.ChecksumFormat, repo.ChecksumFormat, global.ChecksumFormat)
	merged.ChecksumJSONPath = firstString(pkg.ChecksumJSONPath, repo.ChecksumJSONPath, global.ChecksumJSONPath)
	merged.ChecksumRegex = firstString(pkg.ChecksumRegex, repo.ChecksumRegex, global.ChecksumRegex)
	merged.InstallAction = firstString(pkg.InstallAction, repo.InstallAction, global.InstallAction)
	merged.InstallMode = firstString(cli.InstallMode, pkg.InstallMode, repo.InstallMode, global.InstallMode)
	merged.ChunkConcurrency = firstInt(cli.ChunkConcurrency, pkg.ChunkConcurrency, repo.ChunkConcurrency, global.ChunkConcurrency)
	merged.StripComponents = firstInt(cli.StripComponents, pkg.StripComponents, repo.StripComponents, global.StripComponents)

	merged.AssetFilters = firstStrings(cli.AssetFilters, pkg.AssetFilters, repo.AssetFilters, global.AssetFilters)
	merged.RenameFiles = firstStringMap(cli.RenameFiles, pkg.RenameFiles, repo.RenameFiles, global.RenameFiles)
	merged.OSMap = firstStringMap(nil, pkg.OSMap, repo.OSMap, global.OSMap)
	merged.ArchMap = firstStringMap(nil, pkg.ArchMap, repo.ArchMap, global.ArchMap)
	merged.ExtMap = firstStringMap(nil, pkg.ExtMap, repo.ExtMap, global.ExtMap)
	merged.LibcMap = firstStringMap(nil, pkg.LibcMap, repo.LibcMap, global.LibcMap)
	merged.InstallArgs = firstStrings(nil, pkg.InstallArgs, repo.InstallArgs, global.InstallArgs)

	return merged
}

func firstBool(values ...*bool) bool {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return false
}

func firstString(values ...*string) string {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return ""
}

func firstInt(values ...*int) int {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return 0
}

func firstStrings(cli *[]string, values ...[]string) []string {
	if cli != nil {
		return append([]string(nil), (*cli)...)
	}
	for _, value := range values {
		if len(value) > 0 {
			return append([]string(nil), value...)
		}
	}
	return []string{}
}

func firstStringMap(cli *map[string]string, values ...map[string]string) map[string]string {
	if cli != nil {
		return util.CloneStringMap(*cli)
	}
	for _, value := range values {
		if len(value) > 0 {
			return util.CloneStringMap(value)
		}
	}
	return nil
}
