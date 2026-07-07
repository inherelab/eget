package app

import (
	"sync"

	"github.com/inherelab/eget/internal/install"
)

type fakeInstallService struct {
	mu        sync.Mutex
	targets   []string
	options   []install.Options
	result    RunResult
	err       error
	errs      map[string]error
	active    int
	maxActive int
	block     chan struct{}
}

func (f *fakeInstallService) InstallTarget(target string, opts install.Options, extras ...InstallExtras) (RunResult, error) {
	f.mu.Lock()
	f.targets = append(f.targets, target)
	f.options = append(f.options, opts)
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	block := f.block
	f.mu.Unlock()

	if block != nil {
		<-block
	}

	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	if f.errs != nil {
		if err := f.errs[target]; err != nil {
			return RunResult{}, err
		}
	}
	return f.result, f.err
}

func (f *fakeInstallService) currentMaxActive() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxActive
}
