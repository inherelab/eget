package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/cli/prompts"
)

func (s *cliService) handleUpdate(opts *UpdateOptions) error {
	if opts.Self {
		if opts.All {
			return fmt.Errorf("update --self cannot be used with --all")
		}
		if len(opts.Targets) > 0 {
			return fmt.Errorf("update --self cannot be used with target")
		}
		if opts.BatchConcurrency > 0 {
			return fmt.Errorf("--batch can only be used with --all")
		}
		if opts.Interactive {
			return fmt.Errorf("update --self cannot be used with --interactive")
		}
		if s.selfUpdateService == nil {
			return fmt.Errorf("self update service is required")
		}
		source := s.selfUpdateSource(opts)
		if opts.Check {
			printSelfUpdateCheckSource(source)
		}
		result, err := s.selfUpdateService.Update(app.SelfUpdateOptions{
			CheckOnly: opts.Check,
			Tag:       opts.Tag,
			Source:    source,
			Asset:     splitAssetFilters(opts.Asset),
			Install:   s.applyGlobalFlags(installOptionsFromUpdate(opts)),
		})
		if err != nil {
			return err
		}
		printSelfUpdateResult(result)
		return nil
	}
	if opts.Check {
		if len(opts.Targets) > 0 {
			return s.handleUpdateCheckTargets(opts.Targets)
		}
		return s.handleList(&ListOptions{Outdated: true})
	}
	if opts.DryRun {
		return fmt.Errorf("update --dry-run is not implemented")
	}
	installOpts := s.applyGlobalFlags(installOptionsFromUpdate(opts))
	if opts.All || opts.Interactive {
		var targets []string
		if opts.Interactive && !opts.All {
			targets = opts.Targets
		}
		items, err := s.updateCandidatesForPrompt(targets)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			ccolor.Cyanln("🎉 No outdated packages found")
			return nil
		}

		cols := []string{"Name", "Repo", "Installed", "Latest version"}
		rows := make([][]any, 0, len(items))
		for _, item := range items {
			rows = append(rows, []any{item.Name, item.Repo, item.InstalledTag, item.LatestTag})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))

		if opts.Interactive {
			selected, err := selectInteractiveUpdateCandidates(items)
			if err != nil {
				return err
			}
			if len(selected) == 0 {
				ccolor.Cyanln("No packages selected")
				return nil
			}
			items = selected
		}

		ccolor.Magentaf("\n🪄🚀 Updating %d packages:\n", len(items))
		if opts.BatchConcurrency < 0 {
			installOpts.BatchConcurrency = 1
			installOpts.BatchConcurrencySet = true
		}
		prevOnUpdateStart := s.updService.OnUpdateStart
		s.updService.OnUpdateStart = func(index, total int, name string) {
			printUpdateSeparator(index)
		}
		defer func() {
			s.updService.OnUpdateStart = prevOnUpdateStart
		}()
		_, err = s.updService.UpdateCandidates(items, installOpts)
		return err
	}
	if opts.BatchConcurrency > 0 {
		return fmt.Errorf("--batch can only be used with --all")
	}
	if len(opts.Targets) == 0 {
		return fmt.Errorf("update target is required")
	}
	var failures []error
	for index, target := range opts.Targets {
		printUpdateSeparator(index)
		result, err := s.updService.UpdatePackageStatus(target, installOpts)
		if err != nil {
			ccolor.Fprintf(s.stderrWriter(), "<yellow>update_failed</> %s: %v\n", target, err)
			failures = append(failures, err)
			continue
		}
		if !result.Updated {
			ccolor.Cyanf("%s is already up to date: %s\n", target, result.InstalledTag)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d update failed", len(failures))
	}
	return nil
}

func (s *cliService) updateCandidatesForPrompt(targets []string) ([]app.OutdatedItem, error) {
	ccolor.Infoln("🚀 Checking outdated packages")
	s.printOutdatedProxyNotice()
	reporter := newOutdatedProgressReporter(s.stderrWriter(), !appVerbose())
	cacheNotices := &apiCacheNoticeCounter{}
	restoreNotices := suppressOutdatedNetworkNotices(cacheNotices)
	defer restoreNotices()
	prevOnDone := s.updService.OnCheckDone
	s.updService.OnCheckDone = reporter.OnCheckDone
	defer func() {
		s.updService.OnCheckDone = prevOnDone
	}()

	var (
		items    []app.OutdatedItem
		failures []app.OutdatedCheckFailure
		checked  int
		err      error
	)
	if len(targets) > 0 {
		items, failures, checked, err = s.updService.ListUpdateCandidatesForTargets(targets)
	} else {
		items, failures, checked, err = s.updService.ListUpdateCandidates()
	}
	reporter.Finish()
	if err != nil {
		return nil, err
	}
	s.printAPICacheSummary(cacheNotices.Count())
	ccolor.Successf("✅ Checked %d packages\n", checked)

	for _, failure := range failures {
		ccolor.Fprintf(os.Stderr, "<yellow>check_failed</> %s (%s): %v\n", failure.Name, failure.Repo, failure.Error)
	}
	return items, nil
}

func selectInteractiveUpdateCandidates(items []app.OutdatedItem) ([]app.OutdatedItem, error) {
	choices := make([]string, 0, len(items))
	for _, item := range items {
		choices = append(choices, fmt.Sprintf("%s  %s -> %s", item.Name, item.InstalledTag, item.LatestTag))
	}
	indexes, err := prompts.MultiSelect("Select packages to update", "Filter packages", choices)
	if err != nil {
		return nil, err
	}
	selected := make([]app.OutdatedItem, 0, len(indexes))
	for _, index := range indexes {
		if index >= 0 && index < len(items) {
			selected = append(selected, items[index])
		}
	}
	return selected, nil
}

func printUpdateSeparator(index int) {
	if index > 0 {
		ccolor.Grayln("---")
	}
}

func (s *cliService) handleUpdateCheckTargets(targets []string) error {
	ccolor.Infoln("🚀 Checking outdated packages")
	s.printOutdatedProxyNotice()
	reporter := newOutdatedProgressReporter(s.stderrWriter(), !appVerbose())
	cacheNotices := &apiCacheNoticeCounter{}
	restoreNotices := suppressOutdatedNetworkNotices(cacheNotices)
	defer restoreNotices()
	prevOnDone := s.updService.OnCheckDone
	s.updService.OnCheckDone = reporter.OnCheckDone
	defer func() {
		s.updService.OnCheckDone = prevOnDone
	}()

	items, failures, checked, err := s.updService.ListUpdateCandidatesForTargets(targets)
	reporter.Finish()
	if err != nil {
		return err
	}
	s.printAPICacheSummary(cacheNotices.Count())
	ccolor.Successf("✅ Checked %d packages\n", checked)

	for _, failure := range failures {
		ccolor.Fprintf(os.Stderr, "<yellow>check_failed</> %s (%s): %v\n", failure.Name, failure.Repo, failure.Error)
	}
	if len(items) == 0 {
		ccolor.Cyanln("🎉 No outdated packages found")
		return nil
	}

	ccolor.Infoln("Outdated Packages:")
	cols := []string{"Name", "Repo", "Version", "Latest version", "Published At"}
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, []any{item.Name, item.Repo, item.InstalledTag, item.LatestTag, formatOutdatedTime(item.PublishedAt)})
	}
	ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	return nil
}

func (s *cliService) selfUpdateSource(opts *UpdateOptions) string {
	if opts != nil && opts.SelfSource != "" {
		return opts.SelfSource
	}
	lookupEnv := os.LookupEnv
	if s != nil && s.lookupEnv != nil {
		lookupEnv = s.lookupEnv
	}
	if value, ok := lookupEnv("EGET_SELF_UPDATE_SOURCE"); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func printSelfUpdateCheckSource(source string) {
	if strings.TrimSpace(source) == "" {
		ccolor.Infof("Checking self update from: github.com (%s)\n", app.SelfUpdateRepo)
		return
	}
	host := hostFromURL(source)
	if host == "" {
		host = source
	}
	ccolor.Infof("Checking self update from: %s (%s)\n", host, source)
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func printSelfUpdateResult(result app.SelfUpdateResult) {
	if !result.Outdated && !result.Updated {
		ccolor.Cyanf("eget is already up to date: %s\n", result.CurrentVersion)
		return
	}
	if result.Updated && result.Deferred {
		ccolor.Successf("eget update downloaded. It will be replaced after this process exits: %s\n", result.LatestVersion)
		return
	}
	if result.Updated {
		ccolor.Successf("eget updated: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
		return
	}
	ccolor.Infof("eget update available: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
}
