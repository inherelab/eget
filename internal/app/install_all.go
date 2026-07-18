package app

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/install"
	"github.com/inherelab/eget/internal/util"
)

func (s Service) InstallAllPackages(cli install.Options) ([]InstallAllResult, error) {
	if err := validateRawConcurrencyOptions(cli); err != nil {
		return nil, err
	}
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	if len(cfg.Packages) == 0 {
		return nil, fmt.Errorf("no managed packages configured")
	}

	names := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	rawBatch := batchConcurrencyFromConfig(cfg, cli)
	if err := validateConcurrencyOptions(install.Options{BatchConcurrency: rawBatch}); err != nil {
		return nil, err
	}
	batch := effectiveBatchConcurrency(rawBatch, len(names))
	if batch > 1 {
		return s.installAllPackagesConcurrent(cfg, names, cli, batch)
	}

	results := make([]InstallAllResult, 0, len(names))
	var failures []error
	for _, name := range names {
		pkg := cfg.Packages[name]
		repo := util.DerefString(pkg.Repo)
		if repo == "" {
			failures = append(failures, fmt.Errorf("package %q has no repo", name))
			continue
		}
		runTarget, recordTarget, opts, err := s.resolveInstallRequestWithConfig(cfg, name, cli, false)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if err := validateConcurrencyOptions(opts); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
			continue
		}
		opts = applyDefaultInstallTarget(opts)
		opts = normalizeExtractionOptions(opts)
		result, err := s.installResolvedTarget(runTarget, recordTarget, opts)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
			continue
		}
		results = append(results, InstallAllResult{
			Name:   name,
			Target: runTarget,
			Result: result,
		})
	}
	if len(failures) > 0 {
		return results, fmt.Errorf("%d install failed: %w", len(failures), errors.Join(failures...))
	}
	return results, nil
}

func (s Service) installAllPackagesConcurrent(cfg *cfgpkg.File, names []string, cli install.Options, batch int) ([]InstallAllResult, error) {
	type job struct {
		index int
		name  string
	}
	results := make([]InstallAllResult, len(names))
	ok := make([]bool, len(names))
	jobs := make(chan job)
	var failures []error
	var mu sync.Mutex
	recordFailure := func(name string, err error) {
		mu.Lock()
		failures = append(failures, fmt.Errorf("%s: %w", name, err))
		mu.Unlock()
	}

	var wg sync.WaitGroup
	for i := 0; i < batch; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				runTarget, recordTarget, opts, err := s.resolveInstallRequestWithConfig(cfg, item.name, cli, false)
				if err != nil {
					recordFailure(item.name, err)
					continue
				}
				if err := validateConcurrencyOptions(opts); err != nil {
					recordFailure(item.name, err)
					continue
				}
				opts = applyDefaultInstallTarget(opts)
				opts = normalizeExtractionOptions(opts)
				result, err := s.installResolvedTarget(runTarget, recordTarget, opts)
				if err != nil {
					recordFailure(item.name, err)
					continue
				}
				results[item.index] = InstallAllResult{Name: item.name, Target: runTarget, Result: result}
				ok[item.index] = true
			}
		}()
	}

	for index, name := range names {
		jobs <- job{index: index, name: name}
	}
	close(jobs)
	wg.Wait()

	out := make([]InstallAllResult, 0, len(names)-len(failures))
	for index, result := range results {
		if ok[index] {
			out = append(out, result)
		}
	}
	if len(failures) > 0 {
		return out, fmt.Errorf("%d install failed: %w", len(failures), errors.Join(failures...))
	}
	return out, nil
}
