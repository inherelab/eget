package cachemirror

import (
	"strings"
	"time"
)

const defaultTimeoutSeconds = 5

type Options struct {
	Enable   bool
	URL      string
	Timeout  time.Duration
	Fallback bool
	// Token is sent as a bearer token: the mirror endpoints of eget web are
	// protected by the console token, so a remote mirror needs it.
	Token string
}

func NormalizeOptions(opts Options) Options {
	opts.URL = strings.TrimRight(strings.TrimSpace(opts.URL), "/")
	opts.Token = strings.TrimSpace(opts.Token)
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeoutSeconds * time.Second
	}
	return opts
}

func (opts Options) Active() bool {
	opts = NormalizeOptions(opts)
	return opts.Enable && opts.URL != ""
}
