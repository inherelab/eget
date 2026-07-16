package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inherelab/eget/internal/cachemirror"
	"github.com/inherelab/eget/internal/util"
)

func resolvedAPICachePath(opts Options, rawURL string, parsed *url.URL) (string, bool) {
	if (!opts.APICacheEnabled && !opts.CacheMirror.Active()) || !isProviderMetadataRequest(parsed) {
		return "", false
	}
	cacheDir := opts.APICacheDir
	if cacheDir == "" {
		return "", false
	}
	expanded, err := util.Expand(cacheDir)
	if err != nil {
		verbosef("api cache expand error: %v", err)
		return "", false
	}
	return APICacheFilePath(expanded, rawURL), true
}

func tryAPICacheMirror(cachePath string, opts cachemirror.Options) (bool, error) {
	cacheRoot := filepath.Dir(filepath.Dir(cachePath))
	rel, err := cachemirror.RelPath(cacheRoot, cachePath)
	if err != nil {
		return false, err
	}
	key := cachemirror.KeyForRelPath(rel)
	result, err := cachemirror.DownloadToFile(context.Background(), opts, key, cachePath)
	if err != nil {
		return false, fmt.Errorf("cache mirror metadata %s: %w", rel, err)
	}
	if !result.Hit && !opts.Fallback {
		return false, fmt.Errorf("cache mirror metadata miss: %s (fallback disabled)", rel)
	}
	return result.Hit, nil
}

func loadAPICacheResponse(path string, cacheTime int) (*http.Response, bool, error) {
	if path == "" {
		return nil, false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if cacheTime > 0 && time.Since(info.ModTime()) > time.Duration(cacheTime)*time.Second {
		verbosef("api cache expired: %s", path)
		return nil, false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Body:          io.NopCloser(strings.NewReader(string(body))),
		ContentLength: int64(len(body)),
		Header:        make(http.Header),
	}, true, nil
}

func storeAPICacheResponse(path string, resp *http.Response) (*http.Response, error) {
	if path == "" || resp == nil || resp.Body == nil {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(strings.NewReader(string(body)))
	resp.ContentLength = int64(len(body))
	return resp, nil
}
