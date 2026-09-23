package cli

import (
	"fmt"
	"os"

	"github.com/gookit/goutil/x/ccolor"
	appcache "github.com/inherelab/eget/internal/app/cache"
	"github.com/inherelab/eget/internal/cli/prompts"
	"github.com/inherelab/eget/internal/cli/render"
	"golang.org/x/term"
)

func cleanOptionsFromCLI(opts *CacheCleanOptions) (appcache.CleanOptions, error) {
	if opts.KeepLatest {
		if opts.Older != "" || opts.All || opts.API || opts.SDK || opts.SDKIndex || opts.Partial {
			return appcache.CleanOptions{}, fmt.Errorf("--keep-latest conflicts with --older, --all, and non-package cache kinds")
		}
		return appcache.CleanOptions{
			KeepLatest: true,
			DryRun:     opts.DryRun,
			Yes:        opts.Yes,
			Kinds:      []appcache.Kind{appcache.KindPkg},
		}, nil
	}
	olderText := opts.Older
	if olderText == "" {
		olderText = "3d"
	}
	older, err := appcache.ParseOlderDuration(olderText)
	if err != nil {
		return appcache.CleanOptions{}, err
	}
	kinds := make([]appcache.Kind, 0, 5)
	if opts.Pkg {
		kinds = append(kinds, appcache.KindPkg)
	}
	if opts.API {
		kinds = append(kinds, appcache.KindAPI)
	}
	if opts.SDK {
		kinds = append(kinds, appcache.KindSDK)
	}
	if opts.SDKIndex {
		kinds = append(kinds, appcache.KindSDKIndex)
	}
	if opts.Partial {
		kinds = append(kinds, appcache.KindPartial)
	}
	return appcache.CleanOptions{
		Older:  older,
		All:    opts.All,
		DryRun: opts.DryRun,
		Yes:    opts.Yes,
		Kinds:  kinds,
	}, nil
}

func (s *cliService) handleCacheList(opts *CacheListOptions) error {
	result, err := s.cacheService.List("", appcache.ListOptions{Root: opts.Root})
	if err != nil {
		return err
	}
	if opts.JSON {
		return render.PrintJSON(result)
	}

	ccolor.Fprintf(s.stderrWriter(), "Cache files: %d (<green>%s</>)\n", result.TotalFiles, formatBytes(result.TotalSize))
	ccolor.Fprintf(s.stderrWriter(), "- cache dir: %s\n", result.CacheDir)
	for _, file := range result.Files {
		ccolor.Fprintf(s.stderrWriter(), " <mga>%-8s</> <green>%-10s</>  %s\n", file.Kind, formatBytes(file.Size), file.Path)
	}
	return nil
}

func (s *cliService) handleCacheStatus(opts *CacheStatusOptions) error {
	result, err := s.cacheService.Status("")
	if err != nil {
		return err
	}
	if opts.JSON {
		return render.PrintJSON(result)
	}

	ccolor.Fprintln(s.stderrWriter(), "<ylw1>Cache status</>")
	ccolor.Fprintf(s.stderrWriter(), " • cache dir: %s\n", result.CacheDir)
	ccolor.Fprintf(s.stderrWriter(), " • total files: <mga>%d</>\n", result.TotalFiles)
	ccolor.Fprintf(s.stderrWriter(), " • total size: <green>%s</>\n", formatBytes(result.TotalSize))
	for _, kind := range []string{"pkg", "api", "sdk", "sdk-index", "partial"} {
		summary := result.Kinds[kind]
		ccolor.Fprintf(s.stderrWriter(), "   <mga>%-9s</> %-3d files, <green>%s</>\n", kind, summary.Files, formatBytes(summary.Size))
	}
	ccolor.Fprintf(s.stderrWriter(), "\n • run serve: <gray>%s</>\n", result.ServeCommand)
	ccolor.Fprintf(s.stderrWriter(), " • cache mirror: enabled=%v url=%s fallback=%v timeout=%ds\n",
		result.CacheMirror.Enable,
		result.CacheMirror.URL,
		result.CacheMirror.Fallback,
		result.CacheMirror.Timeout,
	)
	return nil
}

func (s *cliService) handleCacheClean(opts *CacheCleanOptions) error {
	cleanOpts, err := cleanOptionsFromCLI(opts)
	if err != nil {
		return err
	}
	preview, err := s.cacheService.PreviewClean("", cleanOpts)
	if err != nil {
		return err
	}
	if cleanOpts.DryRun {
		if opts.JSON {
			return render.PrintJSON(preview)
		}
		ccolor.Fprintln(s.stderrWriter(), "<green>Dry run</>: eget cache clean")
		ccolor.Fprintf(s.stderrWriter(), " - cache dir: %s\n", preview.CacheDir)
		ccolor.Fprintf(s.stderrWriter(), " - matched files: <mga>%d</>\n", preview.MatchedFiles)
		ccolor.Fprintf(s.stderrWriter(), " - matched size: <mga>%s</>\n", formatBytes(preview.MatchedSize))
		if opts.KeepLatest {
			ccolor.Fprintf(s.stderrWriter(), " - kept latest files: <mga>%d</>\n", preview.KeptLatestFiles)
			ccolor.Fprintf(s.stderrWriter(), " - unrecognized files: <mga>%d</>\n", preview.UnrecognizedFiles)
		}
		return nil
	}
	if preview.NeedsConfirmation() && !opts.Yes {
		if !stdinIsTerminal() {
			return fmt.Errorf("cache clean matched %d files (%s); rerun with --yes to confirm", preview.MatchedFiles, formatBytes(preview.MatchedSize))
		}
		ccolor.Fprintf(s.stderrWriter(), "Cache clean matched %d files (%s)\n", preview.MatchedFiles, formatBytes(preview.MatchedSize))
		ccolor.Fprint(s.stderrWriter(), "Continue? [y/N]: ")
		confirmed, err := prompts.ConfirmDefaultNo()
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("cache clean cancelled")
		}
	}
	result, err := s.cacheService.ApplyClean(preview)
	if err != nil {
		return err
	}
	if opts.JSON {
		return render.PrintJSON(result)
	}
	ccolor.Fprintln(s.stderrWriter(), "Cleaned eget cache")
	ccolor.Fprintf(s.stderrWriter(), " - cache dir: %s\n", result.CacheDir)
	ccolor.Fprintf(s.stderrWriter(), " - removed files: <mga>%d</>\n", result.RemovedFiles)
	ccolor.Fprintf(s.stderrWriter(), " - freed size: <green>%s</>\n", formatBytes(result.RemovedSize))
	ccolor.Fprintf(s.stderrWriter(), " - skipped files: <mga>%d</>\n", len(result.Skipped))
	if opts.KeepLatest {
		ccolor.Fprintf(s.stderrWriter(), " - kept latest files: <mga>%d</>\n", result.KeptLatestFiles)
		ccolor.Fprintf(s.stderrWriter(), " - unrecognized files: <mga>%d</>\n", result.UnrecognizedFiles)
	}
	if len(result.Skipped) > 0 {
		ccolor.Fprintln(s.stderrWriter(), "Skipped:")
		for _, skipped := range result.Skipped {
			ccolor.Fprintf(s.stderrWriter(), " - %s: %s\n", skipped.Path, skipped.Reason)
		}
	}
	return nil
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func stdinIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
