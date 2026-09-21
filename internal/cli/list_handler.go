package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/gookit/cliui/show"
	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	clirender "github.com/inherelab/eget/internal/cli/render"
	"github.com/inherelab/eget/internal/install"
)

func (s *cliService) handleList(opts *ListOptions) error {
	if opts != nil && opts.Outdated && opts.Info != "" {
		return fmt.Errorf("list --outdated and --info cannot be used together")
	}
	if opts != nil && opts.Outdated && opts.NoInstalled {
		return fmt.Errorf("list --outdated and --no-installed cannot be used together")
	}
	if opts != nil && opts.NoInstalled && opts.Info != "" {
		return fmt.Errorf("list --no-installed and --info cannot be used together")
	}

	selection, err := s.listManagersSelection(opts)
	if err != nil {
		return err
	}
	restoreSelection := s.applyListManagersSelection(selection)
	defer restoreSelection()

	if opts != nil && opts.Info != "" {
		item, err := s.listService.FindPackage(opts.Info)
		if err != nil {
			return err
		}
		show.AList("Package Info", clirender.ListItemToDisplay(*item))
		return nil
	}

	if opts != nil && opts.Outdated {
		ccolor.Infoln("🚀 Checking outdated packages")
		s.printOutdatedProxyNotice()
		reporter := newOutdatedProgressReporter(s.stderrWriter(), !appVerbose())
		cacheNotices := &apiCacheNoticeCounter{}
		restoreNotices := suppressOutdatedNetworkNotices(cacheNotices)
		defer restoreNotices()
		prevOnDone := s.listService.OnCheckDone
		s.listService.OnCheckDone = reporter.OnCheckDone
		defer func() {
			s.listService.OnCheckDone = prevOnDone
		}()
		items, failures, checked, err := s.listService.ListOutdatedPackages()
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

		printListTitle("Outdated Packages", len(items))
		cols := []string{"Name", "Repo", "Version", "Latest version", "Published At"}
		rows := make([][]any, 0, len(items))
		for _, item := range items {
			rows = append(rows, []any{item.Name, item.Repo, item.InstalledTag, item.LatestTag, formatOutdatedTime(item.PublishedAt)})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
		return nil
	}

	var items []app.ListItem
	if opts != nil && opts.NoInstalled {
		items, err = s.listService.ListPackages()
		items = filterNoInstalledListItems(items)
		if opts.GUI {
			items = filterGUIListItems(items)
		}
	} else if opts != nil && opts.GUI {
		items, err = s.listService.ListGUIPackages(opts.All)
	} else if opts != nil && opts.All {
		items, err = s.listService.ListPackages()
	} else {
		items, err = s.listService.ListInstalledPackages()
	}
	if err != nil {
		return err
	}
	if len(items) == 0 {
		if opts != nil && opts.GUI {
			ccolor.Infoln("no GUI packages found")
		} else if opts != nil && opts.All {
			ccolor.Infoln("no managed packages found")
		} else {
			ccolor.Infoln("no installed packages found")
		}
		return nil
	}

	switch {
	case opts != nil && opts.NoInstalled:
		printListTitle("Not Installed Packages", len(items))
	case opts != nil && opts.GUI:
		printListTitle("GUI Packages", len(items))
	case opts != nil && opts.All:
		printListTitle("Managed Packages", len(items))
	case opts == nil || !opts.GUI:
		printListTitle("Installed Packages", len(items))
	}
	cols := []string{"Name", "Repo", "Version", "Source", "Update Time"}
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, listPackageRow(item))
	}
	ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	return nil
}

func printListTitle(title string, count int) {
	ccolor.Infof("%s (%d):\n", title, count)
}

func (s *cliService) handleShow(opts *ShowOptions) error {
	if opts == nil || opts.Target == "" {
		return fmt.Errorf("show target is required")
	}
	result, err := s.showService.ShowPackage(opts.Target)
	if err != nil {
		return err
	}
	show.AList("Package Details", clirender.ShowResultToDisplay(result))
	return nil
}

func filterNoInstalledListItems(items []app.ListItem) []app.ListItem {
	filtered := make([]app.ListItem, 0, len(items))
	for _, item := range items {
		if !item.Installed {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterGUIListItems(items []app.ListItem) []app.ListItem {
	filtered := make([]app.ListItem, 0, len(items))
	for _, item := range items {
		if item.IsGUI {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func listPackageRow(item app.ListItem) []any {
	return []any{
		item.Name,
		item.Repo,
		listPackageVersion(item),
		packageSource(item),
		formatListTime(item.InstalledAt),
	}
}

func listPackageVersion(item app.ListItem) string {
	if item.Version != "" {
		return item.Version
	}
	if !item.Installed {
		return "<mga>No-Install</>"
	}
	return ""
}

func packageSource(item app.ListItem) string {
	// Packages owned by an external manager show the manager name.
	if item.Manager != "" {
		return item.Manager
	}
	switch install.DetectTargetKind(item.Repo) {
	case install.TargetRepo, install.TargetGitHubURL:
		return "github"
	case install.TargetSourceForge:
		return "sourceforge"
	case install.TargetForge:
		return "forge"
	case install.TargetTemplate:
		return "template"
	case install.TargetPkgTemplate:
		return "pkg-template"
	case install.TargetDirectURL:
		return "url"
	case install.TargetLocalFile:
		return "local"
	default:
		if item.Repo == "" {
			return "-"
		}
		return "unknown"
	}
}

func formatListTime(value time.Time) string {
	return clirender.CompactTime(value)
}

func formatOutdatedTime(value time.Time) string {
	return formatListTime(value)
}

func appVerbose() bool {
	return cliApp != nil && cliApp.Verbose()
}
