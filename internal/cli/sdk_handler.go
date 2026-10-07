package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	clirender "github.com/inherelab/eget/internal/cli/render"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/sdk"
)

func (s *cliService) handleSDKInstall(opts *SDKInstallOptions) error {
	if opts == nil || len(opts.Targets) == 0 {
		return fmt.Errorf("sdk install target is required")
	}
	results, err := s.sdkService.InstallMany(context.Background(), opts.Targets, sdk.InstallOptions{
		Force:    opts.Force,
		Progress: s.sdkDownloadProgress(),
		OnStart: func(target string, version string, host string) {
			ccolor.Printf(" - Install SDK %s -> %s from %s\n", target, version, host)
		},
		OnArchiveReady: func(result sdk.DownloadResult) {
			if result.FromCache {
				ccolor.Printf(" - Use cached SDK archive: %s\n", result.Path)
			}
		},
		OnExtractStart: func(archivePath string, installPath string) {
			ccolor.Printf(" - Extract SDK archive: %s -> %s\n", archivePath, installPath)
		},
	})
	if err != nil {
		return err
	}
	for _, result := range results {
		notes := clirender.SDKResultNotes(result.Cached, result.Resumed)
		if notes != "" {
			ccolor.Successf("✓ Installed %s@%s -> %s (%s)\n", result.Name, result.Version, result.Path, notes)
			continue
		}
		ccolor.Successf("✓ Installed %s@%s -> %s\n", result.Name, result.Version, result.Path)
	}
	return nil
}

func (s *cliService) handleSDKDownload(opts *SDKDownloadOptions) error {
	if opts == nil || len(opts.Targets) == 0 {
		return fmt.Errorf("sdk download target is required")
	}
	results, err := s.sdkService.DownloadMany(context.Background(), opts.Targets, sdk.SDKDownloadOptions{
		Platform:  sdk.PlatformOptions{OS: opts.OS, Arch: opts.Arch},
		OutputDir: opts.Output,
		Progress:  s.sdkDownloadProgress(),
		OnStart: func(target string, version string, host string, goos string, goarch string) {
			ccolor.Printf(" - Download SDK %s -> %s (%s/%s) from %s\n", target, version, goos, goarch, host)
		},
	})
	if err != nil {
		return err
	}
	for _, result := range results {
		notes := clirender.SDKResultNotes(result.Cached, result.Resumed)
		platform := result.OS + "/" + result.Arch
		if notes != "" {
			ccolor.Successf("✓ Downloaded %s@%s %s -> %s (%s)\n", result.Name, result.Version, platform, result.Path, notes)
			continue
		}
		ccolor.Successf("✓ Downloaded %s@%s %s -> %s\n", result.Name, result.Version, platform, result.Path)
	}
	return nil
}

func (s *cliService) sdkDownloadProgress() func(int64) io.Writer {
	return func(size int64) io.Writer {
		return install.NewDownloadProgress(s.stderrWriter(), size)
	}
}

func (s *cliService) handleSDKList(opts *SDKListOptions) error {
	name := ""
	jsonOutput := false
	if opts != nil {
		name = opts.Name
		jsonOutput = opts.JSON
	}
	entries, err := s.sdkService.List(name)
	if err != nil {
		return err
	}
	if jsonOutput {
		return clirender.PrintJSON(clirender.SDKEntriesToDisplay(entries))
	}
	if len(entries) == 0 {
		ccolor.Infoln("no SDK versions installed")
		return nil
	}
	cols := []string{"Name", "Version", "Path", "Installed At"}
	rows := make([][]any, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, []any{entry.Name, entry.Version, entry.Path, clirender.CompactTime(entry.InstalledAt)})
	}
	ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	return nil
}

func (s *cliService) handleSDKRemove(opts *SDKRemoveOptions) error {
	if opts == nil || opts.Target == "" {
		return fmt.Errorf("sdk remove target is required")
	}
	result, err := s.sdkService.Remove(opts.Target)
	if err != nil {
		return err
	}
	if result.Missing {
		ccolor.Warnf("SDK directory already missing: %s\n", result.Path)
		return nil
	}
	ccolor.Successf("✓ Removed %s@%s -> %s\n", result.Name, result.Version, result.Path)
	return nil
}

func (s *cliService) handleSDKPath(opts *SDKPathOptions) error {
	if opts == nil || opts.Target == "" {
		return fmt.Errorf("sdk path target is required")
	}
	entry, err := s.sdkService.Path(opts.Target)
	if err != nil {
		return err
	}
	ccolor.Println(entry.Path)
	return nil
}

func (s *cliService) handleSDKSearch(opts *SDKSearchOptions) error {
	if opts == nil || opts.Name == "" {
		return fmt.Errorf("sdk search name is required")
	}
	results, err := s.sdkService.SearchIndex(opts.Name, sdk.SearchOptions{
		Keywords:          opts.Keywords,
		Number:            opts.Number,
		Sort:              opts.Sort,
		IncludePrerelease: opts.Prerelease,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		return clirender.PrintJSON(results)
	}
	if len(results) == 0 {
		ccolor.Infoln("no SDK index matches found")
		return nil
	}
	cols := []string{"Version", "Stable", "OS", "Arch", "Ext", "Filename"}
	rows := make([][]any, 0, len(results))
	for _, result := range results {
		rows = append(rows, []any{result.Version, result.Stable, result.OS, result.Arch, result.Ext, result.Filename})
	}
	ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	return nil
}

func (s *cliService) handleSDKIndex(opts *SDKIndexOptions) error {
	if opts == nil {
		return fmt.Errorf("sdk index action is required")
	}
	switch opts.Action {
	case "list":
		infos, err := s.sdkService.ListIndexes()
		if err != nil {
			return err
		}
		if opts.JSON {
			return clirender.PrintJSON(clirender.SDKCachedIndexesToDisplay(infos))
		}
		if len(infos) == 0 {
			ccolor.Infoln("no SDK index cache found")
			return nil
		}
		cols := []string{"Name", "Versions", "Source", "Updated At"}
		rows := make([][]any, 0, len(infos))
		for _, info := range infos {
			versions := any(info.Versions)
			updatedAt := clirender.CompactTime(info.FetchedAt)
			if !info.Cached {
				versions = "-"
				updatedAt = "-"
			}
			rows = append(rows, []any{info.SDK, versions, clirender.TruncateTableText(info.SourceURL, 48), updatedAt})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
		return nil
	case "show":
		index, err := s.sdkService.ShowIndex(opts.Name)
		if err != nil {
			return err
		}
		clirender.PrintSDKIndexSummary(index)
		return nil
	case "refresh":
		svc := s.sdkServiceWithIndexReporter()
		if opts.All {
			indexes, err := svc.RefreshAllIndexes(context.Background())
			if err != nil {
				return err
			}
			for _, index := range indexes {
				ccolor.Successf("✓ Refreshed SDK index: %s (%d versions)\n", index.SDK, len(index.Items))
			}
			return nil
		}
		index, err := svc.RefreshIndex(context.Background(), opts.Name)
		if err != nil {
			return err
		}
		ccolor.Successf("✓ Refreshed SDK index: %s (%d versions)\n", index.SDK, len(index.Items))
		return nil
	case "clear":
		if opts.All {
			if err := s.sdkService.ClearAllIndexes(); err != nil {
				return err
			}
			ccolor.Successln("✓ Cleared all SDK indexes")
			return nil
		}
		if err := s.sdkService.ClearIndex(opts.Name); err != nil {
			return err
		}
		ccolor.Successf("✓ Cleared SDK index: %s\n", opts.Name)
		return nil
	default:
		return fmt.Errorf("sdk index action is required")
	}
}

func (s *cliService) handleSDKConfig(opts *SDKConfigOptions) error {
	if opts == nil || opts.Action != "add" {
		return fmt.Errorf("sdk config action is required")
	}
	result, err := s.cfgService.AddSDKConfig(app.SDKConfigAddOptions{
		Name:   opts.Name,
		All:    opts.All,
		Force:  opts.Force,
		Mirror: opts.Mirror,
	})
	if err != nil {
		return err
	}
	for _, item := range result.Items {
		source := item.Source
		if source == "" {
			source = "official"
		}
		switch item.Action {
		case app.SDKConfigActionAdded:
			ccolor.Successf("✓ Added SDK config: %s (%s)\n", item.Name, source)
		case app.SDKConfigActionUpdated:
			ccolor.Successf("✓ Updated SDK config: %s (%s)\n", item.Name, source)
		case app.SDKConfigActionSkipped:
			ccolor.Infof("- Skipped SDK config: %s %s\n", item.Name, item.Reason)
		}
	}
	return nil
}

func (s *cliService) sdkServiceWithIndexReporter() sdkCLIService {
	reporter := s.sdkIndexRefreshReporter()
	switch svc := s.sdkService.(type) {
	case sdk.Service:
		svc.OnIndexRefresh = reporter
		return svc
	case *sdk.Service:
		cloned := *svc
		cloned.OnIndexRefresh = reporter
		return &cloned
	default:
		return s.sdkService
	}
}

func (s *cliService) sdkIndexRefreshReporter() func(sdk.IndexRefreshEvent) {
	return func(event sdk.IndexRefreshEvent) {
		switch event.Stage {
		case sdk.IndexRefreshFetchStart:
			ccolor.Fprintf(s.stderrWriter(), " - Fetch SDK index %s: %s\n", event.SDK, event.URL)
		case sdk.IndexRefreshFetchDone:
			ccolor.Fprintf(s.stderrWriter(), " - Fetched SDK index %s\n", event.SDK)
		case sdk.IndexRefreshParseStart:
			parser := event.Parser
			if parser == "" {
				parser = "-"
			}
			ccolor.Fprintf(s.stderrWriter(), " - Parse SDK index %s: format=%s parser=%s\n", event.SDK, event.Format, parser)
		case sdk.IndexRefreshParseDone:
			ccolor.Fprintf(s.stderrWriter(), " - Parsed SDK index %s: %d versions, %d files\n", event.SDK, event.Versions, event.Files)
		case sdk.IndexRefreshParseFailed:
			ccolor.Fprintf(s.stderrWriter(), " - Parse SDK index failed %s: %v\n", event.SDK, event.Err)
		case sdk.IndexRefreshCacheHit:
			ccolor.Fprintf(s.stderrWriter(), " - Reused cached SDK index %s after fetch failed: %v\n", event.SDK, event.Err)
		}
	}
}
