package app

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/inherelab/eget/internal/install"
)

func (s UpdateService) UpdateCandidates(candidates []OutdatedItem, cli install.Options) ([]UpdateResult, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	if !isAppInstallService(s.Install) {
		cli = applyConfigNetworkOptions(cfg, cli)
	}
	if err := validateRawConcurrencyOptions(cli); err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})

	rawBatch := cli.BatchConcurrency
	if !cli.BatchConcurrencySet && rawBatch <= 0 {
		rawBatch = 0
	}
	if err := validateConcurrencyOptions(install.Options{BatchConcurrency: rawBatch}); err != nil {
		return nil, err
	}
	batch := effectiveBatchConcurrency(rawBatch, len(candidates))
	if batch > 1 {
		return s.updateCandidatesConcurrent(candidates, cli, batch)
	}

	results := make([]UpdateResult, 0, len(candidates))
	var failures []error
	for index, item := range candidates {
		if s.OnUpdateStart != nil {
			s.OnUpdateStart(index, len(candidates), item.Name)
		}
		result, err := s.UpdatePackage(item.Name, cli)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", item.Name, err))
			continue
		}
		results = append(results, UpdateResult{
			Name:   item.Name,
			Target: item.Repo,
			Result: result,
		})
	}
	return results, updateCandidatesError(failures)
}

func isAppInstallService(installer Installer) bool {
	switch installer.(type) {
	case Service, *Service:
		return true
	default:
		return false
	}
}

func (s UpdateService) updateCandidatesConcurrent(candidates []OutdatedItem, cli install.Options, batch int) ([]UpdateResult, error) {
	type job struct {
		index int
		item  OutdatedItem
	}
	results := make([]UpdateResult, len(candidates))
	ok := make([]bool, len(candidates))
	var failures []error
	var mu sync.Mutex
	jobs := make(chan job)

	var wg sync.WaitGroup
	for i := 0; i < batch; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range jobs {
				result, err := s.UpdatePackage(work.item.Name, cli)
				if err != nil {
					mu.Lock()
					failures = append(failures, fmt.Errorf("%s: %w", work.item.Name, err))
					mu.Unlock()
					continue
				}
				mu.Lock()
				results[work.index] = UpdateResult{Name: work.item.Name, Target: work.item.Repo, Result: result}
				ok[work.index] = true
				mu.Unlock()
			}
		}()
	}

	for index, item := range candidates {
		jobs <- job{index: index, item: item}
	}
	close(jobs)
	wg.Wait()

	out := make([]UpdateResult, 0, len(candidates)-len(failures))
	for i, result := range results {
		if ok[i] {
			out = append(out, result)
		}
	}
	return out, updateCandidatesError(failures)
}

func updateCandidatesError(failures []error) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%d update failed: %w", len(failures), errors.Join(failures...))
}
