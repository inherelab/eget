package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gookit/cliui/progress"
	"github.com/gookit/goutil/x/ccolor"
	"github.com/gookit/goutil/x/termenv"
	"github.com/inherelab/eget/internal/cachemirror"
)

const downloadProgressRedrawFreq = 256 * 1024

func (r *InstallRunner) downloadBody(url string, opts Options) (downloadBodyResult, error) {
	cachePath := CacheFilePathWithMeta(opts.CacheDir, url, cacheMetaFromOptions(opts))
	output := r.Stdout
	if output == nil || opts.Quiet {
		output = io.Discard
	}
	if IsLocalFile(url) {
		info, err := os.Stat(url)
		if err != nil {
			return downloadBodyResult{}, err
		}
		return downloadBodyResult{Path: url, Size: info.Size(), ModTime: info.ModTime(), Filename: assetFilename(url)}, nil
	}
	if cachePath != "" && !IsLocalFile(url) {
		if info, err := os.Stat(cachePath); err == nil {
			if !isInvalidCachedDownload(cachePath) {
				ccolor.Fprintf(output, " - Using cached file <cyan>%s</>\n", filepath.Base(cachePath))
				return downloadBodyResult{Path: cachePath, Size: info.Size(), ModTime: info.ModTime(), Filename: assetFilename(url)}, nil
			}
			verbosef("discard invalid cached archive: %s", cachePath)
		}
		mirrorProgress := r.downloadProgress(opts)
		if hit, err := tryCacheMirrorDownload(cachePath, opts, func(size int64) io.Writer {
			ccolor.Fprintf(output, " - Use cache mirror <cyan>%s</>\n", opts.CacheMirror.URL)
			return mirrorProgress(size)
		}); err != nil {
			if opts.CacheMirror.Fallback {
				printCacheMirrorFallback(output, err)
			} else {
				return downloadBodyResult{}, err
			}
		} else if hit {
			info, err := os.Stat(cachePath)
			if err != nil {
				return downloadBodyResult{}, err
			}
			if !isInvalidCachedDownload(cachePath) {
				return downloadBodyResult{Path: cachePath, Size: info.Size(), ModTime: info.ModTime(), Filename: assetFilename(url)}, nil
			}
			if !opts.CacheMirror.Fallback {
				return downloadBodyResult{}, fmt.Errorf("cache mirror returned invalid archive: %s", filepath.Base(cachePath))
			}
			verbosef("discard invalid cache mirror archive: %s", cachePath)
			_ = os.Remove(cachePath)
		}
		result, err := DownloadFile(url, cachePath, r.downloadProgress(opts), opts)
		if err != nil {
			return downloadBodyResult{}, err
		}
		modTime := parseHTTPTime(result.LastModified)
		if !modTime.IsZero() {
			_ = applyModTime(cachePath, modTime)
		}
		info, err := os.Stat(cachePath)
		if err != nil {
			return downloadBodyResult{}, err
		}
		if modTime.IsZero() {
			modTime = fileModTime(cachePath)
		}
		return downloadBodyResult{Path: cachePath, Size: info.Size(), ModTime: modTime, Filename: firstNonEmpty(result.Filename, assetFilename(url))}, nil
	}

	temp, err := os.CreateTemp("", "eget-download-*")
	if err != nil {
		return downloadBodyResult{}, err
	}
	tempPath := temp.Name()
	result, err := DownloadWithResult(url, temp, r.downloadProgress(opts), opts)
	closeErr := temp.Close()
	if err != nil {
		_ = os.Remove(tempPath)
		return downloadBodyResult{}, err
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return downloadBodyResult{}, closeErr
	}
	info, err := os.Stat(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return downloadBodyResult{}, err
	}
	return downloadBodyResult{Path: tempPath, Size: info.Size(), ModTime: parseHTTPTime(result.LastModified), Filename: firstNonEmpty(result.Filename, assetFilename(url)), Temp: true}, nil
}

func cacheMetaFromOptions(opts Options) CacheMeta {
	goos, goarch := cachePlatformFromOptions(opts)
	return CacheMeta{Name: opts.CacheName, Version: opts.CacheVersion, OS: goos, Arch: goarch}
}

func cachePlatformFromOptions(opts Options) (string, string) {
	if opts.URLTemplate.ResolvedVars != nil {
		goos := opts.URLTemplate.ResolvedVars["os"]
		goarch := opts.URLTemplate.ResolvedVars["arch"]
		if goos != "" && goarch != "" {
			return goos, goarch
		}
	}
	parts := strings.SplitN(opts.System, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1]
	}
	return "", ""
}

func tryCacheMirrorDownload(cachePath string, opts Options, progress func(int64) io.Writer) (bool, error) {
	if cachePath == "" || !opts.CacheMirror.Active() {
		return false, nil
	}
	rel, err := cachemirror.RelPath(opts.CacheDir, cachePath)
	if err != nil {
		return false, err
	}
	key := cachemirror.KeyForRelPath(rel)
	result, err := cachemirror.DownloadToFile(context.Background(), opts.CacheMirror, key, cachePath, progress)
	if err != nil {
		if opts.CacheMirror.Fallback {
			verbosef("cache mirror failed: %v", err)
			return false, err
		}
		return false, err
	}
	if !result.Hit && opts.CacheMirror.Fallback {
		verbosef("cache mirror miss: %s", key)
		return false, fmt.Errorf("cache mirror miss: %s", key)
	}
	if !result.Hit && !opts.CacheMirror.Fallback {
		return false, fmt.Errorf("cache mirror miss: %s", key)
	}
	return result.Hit, nil
}

func printCacheMirrorFallback(output io.Writer, err error) {
	if strings.Contains(err.Error(), "cache mirror miss:") {
		ccolor.Fprintf(output, " - Cache mirror miss, fallback to origin: <yellow>%v</>\n", err)
		return
	}
	ccolor.Fprintf(output, " - Cache mirror failed, fallback to origin: <yellow>%v</>\n", err)
}

func isInvalidCachedDownload(cachePath string) bool {
	ext := strings.ToLower(filepath.Ext(cachePath))
	switch ext {
	case ".zip", ".gz", ".tgz", ".xz", ".bz2", ".zst", ".7z", ".rar":
	default:
		return false
	}
	f, err := os.Open(cachePath)
	if err != nil {
		return false
	}
	defer f.Close()
	data := make([]byte, 64)
	n, err := f.Read(data)
	if err != nil && err != io.EOF {
		return false
	}
	trimmed := bytes.TrimSpace(data[:n])
	lowerPrefix := strings.ToLower(string(trimmed[:min(len(trimmed), 64)]))
	return strings.HasPrefix(lowerPrefix, "<!doctype html") || strings.HasPrefix(lowerPrefix, "<html")
}

func parseHTTPTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := http.ParseTime(value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (r *InstallRunner) downloadProgress(opts Options) func(int64) io.Writer {
	return func(size int64) io.Writer {
		pbout := r.Stdout
		if pbout == nil || opts.Quiet {
			pbout = io.Discard
		}
		return newDownloadProgress(pbout, size)
	}
}

func newDownloadProgress(out io.Writer, size int64) *progress.Progress {
	return NewDownloadProgress(out, size)
}

func NewDownloadProgress(out io.Writer, size int64) *progress.Progress {
	if out == nil {
		out = io.Discard
	}
	width, _ := termenv.GetTermSize()
	barWidth, format := downloadProgressLayout(width)
	p := progress.CustomBar(barWidth, progress.BarStyles[0], size)
	p.Out = out
	p.RedrawFreq = downloadProgressRedrawFreq
	p.Format = format
	p.Start()
	return p
}

func downloadProgressLayout(termWidth int) (int, string) {
	if termWidth <= 0 {
		termWidth = 80
	}
	format := "Downloading [{@bar}] <info>{@percent:4s}%</> {@curSize}/{@maxSize}"
	if termWidth >= 120 {
		return 40, format + " ({@elapsed}/{@remaining})"
	}
	if termWidth >= 100 {
		return 32, format + " ({@elapsed}/{@remaining})"
	}
	if termWidth >= 80 {
		return 24, format
	}
	if termWidth >= 64 {
		return 16, format
	}
	return 10, format
}
