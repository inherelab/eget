package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type URLInfo struct {
	RequestedURL string `json:"requested_url" mapstructure:"requested_url"`
	FinalURL     string `json:"final_url" mapstructure:"final_url"`
	Status       string `json:"status" mapstructure:"status"`
	StatusCode   int    `json:"status_code" mapstructure:"status_code"`
	FileName     string `json:"file_name,omitempty" mapstructure:"file_name"`
	Size         *int64 `json:"size,omitempty" mapstructure:"size"`
	ContentType  string `json:"content_type,omitempty" mapstructure:"content_type"`
	LastModified string `json:"last_modified,omitempty" mapstructure:"last_modified"`
	ETag         string `json:"etag,omitempty" mapstructure:"etag"`
	AcceptRanges string `json:"accept_ranges,omitempty" mapstructure:"accept_ranges"`
}

func QueryURL(rawURL string, opts Options) (URLInfo, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return URLInfo{}, fmt.Errorf("query URL must use HTTP or HTTPS")
	}

	resp, headErr := requestWithOptions(http.MethodHead, rawURL, "", opts)
	if headErr == nil {
		info := urlInfoFromResponse(rawURL, resp)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotImplemented &&
			(resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || info.Size != nil) {
			return info, nil
		}
	}

	resp, err = requestWithOptions(http.MethodGet, rawURL, "bytes=0-0", opts)
	if err != nil {
		return URLInfo{}, err
	}
	defer resp.Body.Close()
	info := urlInfoFromResponse(rawURL, resp)
	if resp.StatusCode != http.StatusOK {
		info.Size = nil
	}
	if size, ok := contentRangeSize(resp.Header.Get("Content-Range")); ok {
		info.Size = &size
	}
	return info, nil
}

func contentRangeSize(value string) (int64, bool) {
	slash := strings.LastIndexByte(value, '/')
	if slash < 0 || slash == len(value)-1 || value[slash+1:] == "*" {
		return 0, false
	}
	size, err := strconv.ParseInt(value[slash+1:], 10, 64)
	return size, err == nil && size >= 0
}

func urlInfoFromResponse(rawURL string, resp *http.Response) URLInfo {
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	info := URLInfo{
		RequestedURL: rawURL,
		FinalURL:     finalURL,
		Status:       resp.Status,
		StatusCode:   resp.StatusCode,
		FileName:     responseFilename(resp, finalURL),
		ContentType:  resp.Header.Get("Content-Type"),
		LastModified: resp.Header.Get("Last-Modified"),
		ETag:         resp.Header.Get("ETag"),
		AcceptRanges: strings.TrimSpace(resp.Header.Get("Accept-Ranges")),
	}
	if resp.ContentLength >= 0 {
		size := resp.ContentLength
		info.Size = &size
	}
	return info
}
