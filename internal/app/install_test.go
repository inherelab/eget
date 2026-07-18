package app

import (
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
)

type fakeRunner struct {
	target string
	opts   install.Options
	result RunResult
	err    error
	calls  int
}

func (f *fakeRunner) Run(target string, opts install.Options) (RunResult, error) {
	f.calls++
	f.target = target
	f.opts = opts
	return f.result, f.err
}

type fakeInstalledStore struct {
	target string
	entry  storepkg.Entry
	err    error
	calls  int
}

func (f *fakeInstalledStore) Record(target string, entry storepkg.Entry) error {
	f.calls++
	f.target = target
	f.entry = entry
	return f.err
}

type fakeConfigRecorder struct {
	repo  string
	name  string
	opts  install.Options
	err   error
	calls int
}

func (f *fakeConfigRecorder) AddPackage(repo, name string, opts install.Options) error {
	f.calls++
	f.repo = repo
	f.name = name
	f.opts = opts
	return f.err
}

func (f *fakeBatchRunner) Run(target string, opts install.Options) (RunResult, error) {
	f.mu.Lock()
	f.targets = append(f.targets, target)
	if f.opts == nil {
		f.opts = map[string]install.Options{}
	}
	f.opts[target] = opts
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

	if err := f.errs[target]; err != nil {
		return RunResult{}, err
	}
	if result, ok := f.results[target]; ok {
		return result, nil
	}
	return RunResult{}, nil
}

func (f *fakeBatchRunner) currentMaxActive() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxActive
}
