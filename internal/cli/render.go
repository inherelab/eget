package cli

import (
	"fmt"
	"time"

	"github.com/gookit/cliui/show"
	"github.com/gookit/goutil/cliutil"
	"github.com/gookit/goutil/mathutil"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/inherelab/eget/internal/app"
)

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
