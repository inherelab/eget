package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	app "github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/app/web"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/sdk"
)

// The console never talks to a terminal: asset questions turn into errors that
// carry the candidate list, downloaded binaries are never executed, and GUI
// installers only run when the request explicitly asks for a silent install.

func (s *cliService) webTaskInstall(ctx context.Context, params map[string]any, report *web.TaskReporter) (any, error) {
	target := strings.TrimSpace(web.TaskParamString(params, "target"))
	if target == "" {
		return nil, fmt.Errorf("target is required")
	}
	silent := web.TaskParamBool(params, "silent")

	opts := install.Options{
		Tag:            web.TaskParamString(params, "version"),
		Output:         web.TaskParamString(params, "output"),
		ExtractFile:    web.TaskParamString(params, "file"),
		All:            web.TaskParamBool(params, "extractAll"),
		DownloadOnly:   web.TaskParamBool(params, "downloadOnly"),
		Silent:         silent,
		DeferInstaller: !silent,
	}
	if asset := strings.TrimSpace(web.TaskParamString(params, "asset")); asset != "" {
		opts.Asset = []string{asset}
	}
	opts = s.applyGlobalFlags(opts)
	opts.Context = ctx
	opts.Progress = func(total int64) io.Writer {
		return &taskTransferWriter{report: report, ctx: ctx, total: total}
	}

	runner, err := s.webInstallRunner(report, silent)
	if err != nil {
		return nil, err
	}
	// Copy the app service so the console runner stays local to this task.
	service := s.appService
	service.Runner = runner

	report.Info("installing %s", target)
	result, err := service.InstallTarget(target, opts, app.InstallExtras{
		AddToConfig: web.TaskParamBool(params, "addToConfig"),
	})
	if err != nil {
		return nil, err
	}
	if installer := deferredInstaller(result, silent); installer != "" {
		report.Info("downloaded the GUI installer %s; the console does not launch it — run it yourself, or install with silent", installer)
		return result, nil
	}
	report.Info("installed %s %s", target, result.Version)
	return result, nil
}

// deferredInstaller returns the installer path when the run only downloaded a GUI
// installer: the console keeps the file and says so instead of asking.
func deferredInstaller(result install.RunResult, silent bool) string {
	if silent || !result.IsGUI || result.InstallMode != install.InstallModeInstaller {
		return ""
	}
	return result.InstallerFile
}

// webInstallRunner builds a non-interactive runner for one console task. With
// silent the installer runs unattended (MSI /qn); otherwise the console only
// downloads it and says where it landed.
func (s *cliService) webInstallRunner(report *web.TaskReporter, silent bool) (*install.InstallRunner, error) {
	if s.installService == nil {
		return nil, fmt.Errorf("the install service is unavailable")
	}
	runner := install.NewRunner(s.installService)
	runner.Stdout = newTaskLogWriter(report, "info")
	runner.Stderr = newTaskLogWriter(report, "error")
	runner.Prompt = func(title, _ string, choices []string) (int, error) {
		return 0, fmt.Errorf("%s: %d assets match, pass \"asset\" to pick one: %s",
			title, len(choices), strings.Join(choices, ", "))
	}
	// The console never asks and never launches a GUI installer on its own: with
	// silent it runs unattended, otherwise it refuses with the reason, because a
	// task has no terminal to answer a prompt on.
	runner.ConfirmLaunchInstaller = func(file string) (bool, error) {
		if silent {
			report.Info("launching installer %s unattended", file)
			return true, nil
		}
		return false, fmt.Errorf("refusing to launch the GUI installer %s from the console: pass silent to install it unattended, or run the downloaded file yourself", file)
	}
	runner.AssetRunner = func(path string, _ []string, _, _ io.Writer) error {
		return fmt.Errorf("refusing to execute the downloaded asset %s from the console", path)
	}
	return runner, nil
}

func (s *cliService) webTaskSDKInstall(ctx context.Context, params map[string]any, report *web.TaskReporter) (any, error) {
	targets := web.TaskParamStrings(params, "targets")
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one SDK target is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	report.Info("installing %s", strings.Join(targets, ", "))
	results, err := s.sdkService.InstallMany(ctx, targets, sdk.InstallOptions{})
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results}, nil
}

func (s *cliService) webTaskSDKDownload(ctx context.Context, params map[string]any, report *web.TaskReporter) (any, error) {
	targets := web.TaskParamStrings(params, "targets")
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one SDK target is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	report.Info("downloading %s", strings.Join(targets, ", "))
	results, err := s.sdkService.DownloadMany(ctx, targets, sdk.SDKDownloadOptions{})
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results}, nil
}

// taskTransferWriter reports download progress and aborts the transfer when the
// task is canceled: returning a write error stops io.Copy in the downloader.
type taskTransferWriter struct {
	report *web.TaskReporter
	ctx    context.Context
	total  int64

	current  int64
	lastSent time.Time
	lastPct  float64
}

func (w *taskTransferWriter) Write(data []byte) (int, error) {
	if w.ctx != nil {
		if err := w.ctx.Err(); err != nil {
			return 0, err
		}
	}
	w.current += int64(len(data))
	percent := 0.0
	if w.total > 0 {
		percent = float64(w.current) / float64(w.total) * 100
	}
	if time.Since(w.lastSent) >= 250*time.Millisecond || percent-w.lastPct >= 1 {
		w.lastSent = time.Now()
		w.lastPct = percent
		w.report.ProgressBytes(w.current, w.total, "download")
	}
	return len(data), nil
}

var ansiEscapePattern = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// taskLogWriter turns a CLI writer into task log lines, one line per entry and
// without terminal escape sequences.
type taskLogWriter struct {
	report *web.TaskReporter
	level  string

	mu  sync.Mutex
	buf []byte
}

func newTaskLogWriter(report *web.TaskReporter, level string) *taskLogWriter {
	return &taskLogWriter{report: report, level: level}
}

func (w *taskLogWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, data...)
	for {
		index := bytes.IndexByte(w.buf, '\n')
		if index < 0 {
			break
		}
		line := strings.TrimSpace(ansiEscapePattern.ReplaceAllString(string(w.buf[:index]), ""))
		w.buf = append([]byte(nil), w.buf[index+1:]...)
		if line != "" {
			w.report.Logf(w.level, "%s", line)
		}
	}
	// A writer that never emits a newline must not grow without bound.
	if len(w.buf) > 8*1024 {
		w.buf = w.buf[:0]
	}
	return len(data), nil
}
