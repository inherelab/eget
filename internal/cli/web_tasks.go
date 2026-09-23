package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	app "github.com/inherelab/eget/internal/app"
	appcache "github.com/inherelab/eget/internal/app/cache"
	"github.com/inherelab/eget/internal/app/web"
	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
)

// webTaskRunners wires the console's write endpoints to the same app services
// the CLI uses. Every runner receives already-validated structural parameters.
func (s *cliService) webTaskRunners() map[string]web.TaskRunner {
	return map[string]web.TaskRunner{
		"install":      s.webTaskInstall,
		"update":       s.webTaskUpdate,
		"uninstall":    s.webTaskUninstall,
		"ext.upgrade":  s.webTaskExtUpgrade,
		"cache.clean":  s.webTaskCacheClean,
		"sdk.install":  s.webTaskSDKInstall,
		"sdk.download": s.webTaskSDKDownload,
	}
}

// webTaskStorePath keeps task history beside the config file.
func webTaskStorePath() string {
	configPath, err := cfgpkg.ResolveWritablePath()
	if err != nil || configPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configPath), "tasks.json")
}

func (s *cliService) webTaskUpdate(ctx context.Context, params map[string]any, report *web.TaskReporter) (any, error) {
	targets := web.TaskParamStrings(params, "targets")
	all := web.TaskParamBool(params, "all")
	opts := s.applyGlobalFlags(install.Options{})

	// Copy the service value so per-task callbacks cannot leak into another
	// request or into the CLI's shared instance.
	updateService := s.updService
	updateService.OnUpdateStart = func(index, total int, name string) {
		report.Progress(float64(index)/float64(maxInt(total, 1))*100, "update")
		report.Info("[%d/%d] updating %s", index+1, total, name)
	}
	updateService.OnUpdateDone = func(item app.OutdatedItem, _ app.RunResult, err error) {
		if err != nil {
			report.Error("%s: %v", item.Name, err)
			return
		}
		label := item.Name
		if item.Manager != "" {
			label = item.Manager + ":" + item.Name
		}
		if item.LatestTag != "" {
			report.Info("updated %s %s -> %s", label, item.InstalledTag, item.LatestTag)
			return
		}
		report.Info("updated %s", label)
	}

	if all {
		report.Info("updating every outdated package")
		results, err := updateService.UpdateAllPackages(opts)
		if err != nil {
			return nil, err
		}
		return map[string]any{"results": results}, nil
	}

	if len(targets) == 1 {
		report.Info("updating %s", targets[0])
		result, err := updateService.UpdatePackageStatus(targets[0], opts)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	results := make([]app.UpdatePackageResult, 0, len(targets))
	for index, target := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		report.Progress(float64(index)/float64(len(targets))*100, "update")
		report.Info("updating %s", target)
		result, err := updateService.UpdatePackageStatus(target, opts)
		if err != nil {
			return map[string]any{"results": results}, fmt.Errorf("%s: %w", target, err)
		}
		results = append(results, result)
	}
	return map[string]any{"results": results}, nil
}

func (s *cliService) webTaskUninstall(_ context.Context, params map[string]any, report *web.TaskReporter) (any, error) {
	target := web.TaskParamString(params, "target")
	purge := web.TaskParamBool(params, "purge")
	report.Info("uninstalling %s", target)

	result, err := s.uninstallService.UninstallWithOptions(target, app.UninstallOptions{Purge: purge})
	if err != nil {
		return nil, err
	}
	report.Info("removed %d file(s)", len(result.RemovedFiles))
	if result.PurgedConfig != "" {
		report.Info("purged package config %s", result.PurgedConfig)
	}
	return result, nil
}

func (s *cliService) webTaskExtUpgrade(ctx context.Context, params map[string]any, report *web.TaskReporter) (any, error) {
	manager := web.TaskParamString(params, "manager")
	names := web.TaskParamStrings(params, "names")
	if manager == "" {
		return nil, fmt.Errorf("manager is required")
	}
	label := "all packages"
	if len(names) > 0 {
		label = fmt.Sprintf("%d package(s)", len(names))
	}
	report.Info("upgrading %s of %s", label, manager)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	result, err := s.extPkg.Upgrade(ctx, manager, names)
	if err != nil {
		return nil, err
	}
	for _, name := range result.Names {
		report.Info("upgraded %s", name)
	}
	if result.Output != "" {
		report.Info("%s", result.Output)
	}
	return result, nil
}

func (s *cliService) webTaskCacheClean(_ context.Context, params map[string]any, report *web.TaskReporter) (any, error) {
	mode := web.TaskParamString(params, "mode")
	days := web.TaskParamInt(params, "days")
	confirm := web.TaskParamBool(params, "confirm")

	cleanOpts := appcache.CleanOptions{Yes: true}
	switch mode {
	case "all":
		cleanOpts.All = true
	case "keep-latest":
		cleanOpts.KeepLatest = true
	default:
		if days <= 0 {
			days = 3
		}
		cleanOpts.Older = time.Duration(days) * 24 * time.Hour
	}
	for _, kind := range web.TaskParamStrings(params, "kinds") {
		cleanOpts.Kinds = append(cleanOpts.Kinds, appcache.Kind(kind))
	}

	preview, err := s.cacheService.PreviewClean("", cleanOpts)
	if err != nil {
		return nil, err
	}
	report.Progress(50, "preview")
	report.Info("matched %d file(s), %s", preview.MatchedFiles, formatBytes(preview.MatchedSize))
	if !confirm {
		return preview, nil
	}
	result, err := s.cacheService.ApplyClean(preview)
	if err != nil {
		return nil, err
	}
	report.Info("removed %d file(s), freed %s", result.RemovedFiles, formatBytes(result.RemovedSize))
	return result, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
