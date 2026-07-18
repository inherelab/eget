package app

import (
	"fmt"
	"strings"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/util"
)

func inferAddPackageName(target string, opts install.Options, extras []InstallExtras) (install.Options, []InstallExtras) {
	if len(extras) == 0 || !extras[0].AddToConfig || opts.Name != "" || extras[0].PackageName != "" {
		return opts, extras
	}
	name, err := ResolvePackageName(target, "")
	if err != nil || name == "" {
		return opts, extras
	}
	opts.Name = name
	extras[0].PackageName = name
	extras[0].PackageOpts.Name = name
	return opts, extras
}

func applyDefaultInstallTarget(opts install.Options) install.Options {
	if opts.Output == "" {
		opts.Output = defaultInstallTarget()
		opts.OutputExplicit = false
	}
	return opts
}

func defaultInstallTarget() string {
	target, err := util.Expand("~/.local/bin")
	if err != nil || target == "" {
		return "~/.local/bin"
	}
	return target
}

func normalizeExtractionOptions(opts install.Options) install.Options {
	if hasMultipleExtractPatterns(opts.ExtractFile) {
		opts.All = true
	}
	if opts.ExtractFile != "" || opts.All {
		opts.DownloadOnly = false
	}
	return opts
}

func hasMultipleExtractPatterns(value string) bool {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(value, ",") {
			return true
		}
		if strings.ContainsAny(part, "*?[{") {
			return true
		}
	}
	return false
}

func validateConcurrencyOptions(opts install.Options) error {
	if opts.ChunkConcurrency < 0 || opts.ChunkConcurrency > maxChunkConcurrency {
		return fmt.Errorf("chunk concurrency must be between 0 and %d", maxChunkConcurrency)
	}
	if opts.BatchConcurrency < 0 || opts.BatchConcurrency > maxBatchConcurrency {
		return fmt.Errorf("batch concurrency must be between 0 and %d", maxBatchConcurrency)
	}
	return nil
}

func validateRawConcurrencyOptions(opts install.Options) error {
	if opts.ChunkConcurrency < -1 || opts.BatchConcurrency < -1 {
		return fmt.Errorf("concurrency must be 0 auto, 1 serial/single connection, or greater than 1")
	}
	return nil
}

func batchConcurrencyFromConfig(cfg *cfgpkg.File, cli install.Options) int {
	if cli.BatchConcurrencySet || cli.BatchConcurrency > 0 {
		return cli.BatchConcurrency
	}
	if cfg != nil && cfg.Global.BatchConcurrency != nil {
		return *cfg.Global.BatchConcurrency
	}
	return 0
}

func effectiveBatchConcurrency(value, total int) int {
	if total <= 1 {
		return 1
	}
	if value == 1 {
		return 1
	}
	if value <= 0 {
		if total < defaultAutoBatchConcurrency {
			return total
		}
		return defaultAutoBatchConcurrency
	}
	if value > total {
		return total
	}
	return value
}

func mapOpt(value map[string]string) *map[string]string {
	if len(value) == 0 {
		return nil
	}
	cloned := util.CloneStringMap(value)
	return &cloned
}

func expandPath(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return util.Expand(value)
}
