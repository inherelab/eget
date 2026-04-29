package config

func MergeInstallOptions(global, repo, pkg Section, cli CLIOverrides) Merged {
	merged := Merged{}

	merged.ExtractAll = firstBool(cli.ExtractAll, pkg.ExtractAll, repo.ExtractAll, global.ExtractAll)
	merged.DownloadOnly = firstBool(cli.DownloadOnly, pkg.DownloadOnly, repo.DownloadOnly, global.DownloadOnly)
	merged.Source = firstBool(cli.Source, pkg.Source, repo.Source, global.Source)
	merged.Quiet = firstBool(cli.Quiet, pkg.Quiet, repo.Quiet, global.Quiet)
	merged.ShowHash = firstBool(cli.ShowHash, pkg.ShowHash, repo.ShowHash, global.ShowHash)
	merged.UpgradeOnly = firstBool(cli.UpgradeOnly, pkg.UpgradeOnly, repo.UpgradeOnly, global.UpgradeOnly)
	merged.DisableSSL = firstBool(cli.DisableSSL, pkg.DisableSSL, repo.DisableSSL, global.DisableSSL)
	merged.IsGUI = firstBool(cli.IsGUI, pkg.IsGUI, repo.IsGUI)

	merged.File = firstString(cli.File, pkg.File, repo.File, global.File)
	merged.CacheDir = firstString(cli.CacheDir, pkg.CacheDir, repo.CacheDir, global.CacheDir)
	merged.ProxyURL = firstString(cli.ProxyURL, pkg.ProxyURL, repo.ProxyURL, global.ProxyURL)
	merged.GithubToken = firstString(cli.GithubToken, pkg.GithubToken, repo.GithubToken, global.GithubToken)
	merged.GuiTarget = firstString(global.GuiTarget)
	merged.Name = firstString(cli.Name, pkg.Name, repo.Name, global.Name)
	merged.SourcePath = firstString(cli.SourcePath, pkg.SourcePath, repo.SourcePath, global.SourcePath)
	merged.System = firstString(cli.System, pkg.System, repo.System, global.System)
	merged.Tag = firstString(cli.Tag, pkg.Tag, repo.Tag, global.Tag)
	merged.Target = firstString(cli.Target, pkg.Target, repo.Target, global.Target)
	merged.Verify = firstString(cli.Verify, pkg.Verify, repo.Verify, global.Verify)

	merged.AssetFilters = firstStrings(cli.AssetFilters, pkg.AssetFilters, repo.AssetFilters, global.AssetFilters)

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
