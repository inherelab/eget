package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gookit/cliui/show"
	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/mathutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func printConfigList(out io.Writer, path string, exists bool, cfg *cfgpkg.File) {
	fmt.Fprintf(out, "# %s, exists: %t\n", path, exists)
	if cfg.Global.Target != nil || cfg.Global.System != nil || cfg.Global.CacheDir != nil || cfg.Global.ProxyURL != nil {
		fmt.Fprintln(out, "[global]")
		if cfg.Global.Target != nil {
			fmt.Fprintf(out, "target = %s\n", *cfg.Global.Target)
		}
		if cfg.Global.System != nil {
			fmt.Fprintf(out, "system = %s\n", *cfg.Global.System)
		}
		if cfg.Global.CacheDir != nil {
			fmt.Fprintf(out, "cache_dir = %s\n", *cfg.Global.CacheDir)
		}
		if cfg.Global.ProxyURL != nil {
			fmt.Fprintf(out, "proxy_url = %s\n", *cfg.Global.ProxyURL)
		}
	}
	if cfg.ApiCache.Enable != nil || cfg.ApiCache.CacheTime != nil {
		fmt.Fprintln(out, "\n[api_cache]")
		if cfg.ApiCache.Enable != nil {
			fmt.Fprintf(out, "enable = %t\n", *cfg.ApiCache.Enable)
		}
		if cfg.ApiCache.CacheTime != nil {
			fmt.Fprintf(out, "cache_time = %d\n", *cfg.ApiCache.CacheTime)
		}
	}
	if cfg.Ghproxy.Enable != nil || cfg.Ghproxy.HostURL != nil || cfg.Ghproxy.SupportAPI != nil || len(cfg.Ghproxy.Fallbacks) > 0 {
		fmt.Fprintln(out, "\n[ghproxy]")
		if cfg.Ghproxy.Enable != nil {
			fmt.Fprintf(out, "enable = %t\n", *cfg.Ghproxy.Enable)
		}
		if cfg.Ghproxy.HostURL != nil {
			fmt.Fprintf(out, "host_url = %s\n", *cfg.Ghproxy.HostURL)
		}
		if cfg.Ghproxy.SupportAPI != nil {
			fmt.Fprintf(out, "support_api = %t\n", *cfg.Ghproxy.SupportAPI)
		}
		if len(cfg.Ghproxy.Fallbacks) > 0 {
			fmt.Fprintf(out, "fallbacks = %s\n", strings.Join(cfg.Ghproxy.Fallbacks, ", "))
		}
	}

	if len(cfg.Repos) > 0 {
		names := make([]string, 0, len(cfg.Repos))
		for name := range cfg.Repos {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(out, "[repo.%s]\n", name)
		}
	}

	if len(cfg.Packages) > 0 {
		names := make([]string, 0, len(cfg.Packages))
		for name := range cfg.Packages {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			section := cfg.Packages[name]
			fmt.Fprintf(out, "[packages.%s]\n", name)
			if section.Repo != nil {
				fmt.Fprintf(out, "repo = %s\n", *section.Repo)
			}
			if section.Target != nil {
				fmt.Fprintf(out, "target = %s\n", *section.Target)
			}
			if section.CacheDir != nil {
				fmt.Fprintf(out, "cache_dir = %s\n", *section.CacheDir)
			}
		}
	}
}

func printListItemDetails(item *app.ListItem) {
	fmt.Printf("name: %s\n", item.Name)
	fmt.Printf("repo: %s\n", item.Repo)
	if item.Target != "" {
		fmt.Printf("target: %s\n", item.Target)
	}
	if item.Tag != "" {
		fmt.Printf("tag: %s\n", item.Tag)
	}
	fmt.Printf("installed: %s\n", map[bool]string{true: "yes", false: "no"}[item.Installed])
	fmt.Printf("is_gui: %s\n", map[bool]string{true: "yes", false: "no"}[item.IsGUI])
	if item.InstallMode != "" {
		fmt.Printf("install_mode: %s\n", item.InstallMode)
	}
	if item.Version != "" {
		fmt.Printf("version: %s\n", item.Version)
	}
	if item.InstalledTag != "" {
		fmt.Printf("installed_tag: %s\n", item.InstalledTag)
	}
	if !item.InstalledAt.IsZero() {
		fmt.Printf("installed_at: %s\n", item.InstalledAt.Format(time.RFC3339))
	}
	if item.Asset != "" {
		fmt.Printf("asset: %s\n", item.Asset)
	}
	if item.URL != "" {
		fmt.Printf("url: %s\n", item.URL)
	}
}

func printQueryResult(result app.QueryResult) {
	fmt.Printf("action: %s\n", result.Action)
	fmt.Printf("repo: %s\n", result.Repo)
	if result.Tag != "" {
		fmt.Printf("version: %s\n", result.Tag)
	}

	if result.Info != nil {
		show.AList("Repo Info", result.Info)
		return
	}

	if result.Latest != nil {
		show.AList("Latest Release", result.Latest)
		return
	}

	if len(result.Releases) > 0 {
		cols := []string{"Tag", "Name", "Published at", "Prerelease", "Assets Count"}
		rows := make([][]any, 0, len(result.Releases))
		for _, item := range result.Releases {
			rows = append(rows, []any{
				item.Tag,
				item.Name,
				item.PublishedAt.Format(time.RFC3339),
				item.Prerelease,
				item.AssetsCount,
			})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
		return
	}
	if len(result.Assets) > 0 {
		cols := []string{"Name", "Size", "Download Count"}
		rows := make([][]any, 0, len(result.Assets))
		for _, item := range result.Assets {
			rows = append(rows, []any{
				item.Name,
				mathutil.DataSize(uint64(item.Size)),
				item.DownloadCount,
			})
		}
		ccolor.Print(cliutil.FormatTable(cols, rows, cliutil.MinimalStyle))
	}
}

func printSearchResult(result app.SearchResult) {
	if len(result.Items) == 0 {
		ccolor.Infoln("no repositories found")
		return
	}

	for _, item := range result.Items {
		language := item.Language
		if language == "" {
			language = "-"
		}
		updatedAt := "-"
		if !item.UpdatedAt.IsZero() {
			updatedAt = item.UpdatedAt.Format(time.RFC3339)
		}

		ccolor.Printf("<info>%s</> ⭐%d language: %s update: %s\n", item.FullName, item.StargazersCount, language, updatedAt)
		if item.Description != "" {
			ccolor.Printf("%s\n", item.Description)
		} else {
			ccolor.Println("No description")
		}
		fmt.Println("---")
	}
}
