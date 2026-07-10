package cachemirror

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

type DownloadResult struct {
	Hit  bool
	Size int64
}

type progressFinisher interface {
	Finish(...string)
}

func DownloadToFile(ctx context.Context, opts Options, key, target string, getbars ...func(size int64) io.Writer) (DownloadResult, error) {
	opts = NormalizeOptions(opts)
	if !opts.Active() {
		return DownloadResult{}, nil
	}
	downloadURL, err := DownloadURL(opts.URL, key)
	if err != nil {
		return DownloadResult{}, err
	}
	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: opts.Timeout,
		}).DialContext,
		TLSHandshakeTimeout:   opts.Timeout,
		ResponseHeaderTimeout: opts.Timeout,
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return DownloadResult{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return DownloadResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return DownloadResult{Hit: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return DownloadResult{}, fmt.Errorf("cache mirror download error: %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return DownloadResult{}, err
	}
	partPath := target + ".mirror-part"
	out, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return DownloadResult{}, err
	}
	bar := downloadProgressWriter(resp.ContentLength, getbars...)
	size, copyErr := io.Copy(io.MultiWriter(out, bar), resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(partPath)
		return DownloadResult{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(partPath)
		return DownloadResult{}, closeErr
	}
	if err := os.Rename(partPath, target); err != nil {
		_ = os.Remove(partPath)
		return DownloadResult{}, err
	}
	if finisher, ok := bar.(progressFinisher); ok {
		finisher.Finish()
	}
	return DownloadResult{Hit: true, Size: size}, nil
}

func downloadProgressWriter(size int64, getbars ...func(size int64) io.Writer) io.Writer {
	if len(getbars) == 0 || getbars[0] == nil {
		return io.Discard
	}
	bar := getbars[0](size)
	if bar == nil {
		return io.Discard
	}
	return bar
}
