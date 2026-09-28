package app

import (
	"time"

	"github.com/inherelab/eget/internal/cachemirror"
	cfgpkg "github.com/inherelab/eget/internal/config"
)

func CacheMirrorOptionsFromConfig(cfg *cfgpkg.File) cachemirror.Options {
	if cfg == nil {
		return cachemirror.NormalizeOptions(cachemirror.Options{Fallback: true})
	}
	opts := cachemirror.Options{Fallback: true}
	if cfg.CacheMirror.Enable != nil {
		opts.Enable = *cfg.CacheMirror.Enable
	}
	if cfg.CacheMirror.URL != nil {
		opts.URL = *cfg.CacheMirror.URL
	}
	if cfg.CacheMirror.Timeout != nil {
		opts.Timeout = time.Duration(*cfg.CacheMirror.Timeout) * time.Second
	}
	if cfg.CacheMirror.Fallback != nil {
		opts.Fallback = *cfg.CacheMirror.Fallback
	}
	if cfg.CacheMirror.Token != nil {
		opts.Token = *cfg.CacheMirror.Token
	}
	return cachemirror.NormalizeOptions(opts)
}
