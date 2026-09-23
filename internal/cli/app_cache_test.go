package cli

import (
	"bytes"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestMain_CacheCleanBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"cache", "clean", "--older", "7d", "--pkg", "--sdk", "--dry-run", "--yes"})

	assert.NoErr(t, err)
	assert.Eq(t, 1, len(calls))
	assert.Eq(t, "cache.clean", calls[0].name)
	opts, ok := calls[0].options.(*CacheCleanOptions)
	assert.True(t, ok)
	assert.Eq(t, "7d", opts.Older)
	assert.True(t, opts.Pkg)
	assert.True(t, opts.SDK)
	assert.True(t, opts.DryRun)
	assert.True(t, opts.Yes)
}

func TestMain_CacheListBindsOptions(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"cache", "list", "--root", "sdk", "--json"})

	assert.NoErr(t, err)
	assert.Eq(t, "cache.list", calls[0].name)
	opts, ok := calls[0].options.(*CacheListOptions)
	assert.True(t, ok)
	assert.Eq(t, "sdk", opts.Root)
	assert.True(t, opts.JSON)
}

func TestMain_CacheStatusBindsJSON(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"cache", "status", "--json"})

	assert.NoErr(t, err)
	assert.Eq(t, "cache.status", calls[0].name)
	opts, ok := calls[0].options.(*CacheStatusOptions)
	assert.True(t, ok)
	assert.True(t, opts.JSON)
}

func TestMain_CacheCleanBindsJSON(t *testing.T) {
	calls := make([]commandCall, 0, 1)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := newApp(handler, &stdout, &stderr).RunWithArgs([]string{"cache", "clean", "--dry-run", "--json"})

	assert.NoErr(t, err)
	assert.Eq(t, "cache.clean", calls[0].name)
	opts := calls[0].options.(*CacheCleanOptions)
	assert.True(t, opts.DryRun)
	assert.True(t, opts.JSON)
}

func TestMain_CacheCleanBindsKeepLatestAndResets(t *testing.T) {
	calls := make([]commandCall, 0, 2)
	handler := func(name string, options any) error {
		calls = append(calls, commandCall{name: name, options: options})
		return nil
	}
	var stdout, stderr bytes.Buffer
	app := newApp(handler, &stdout, &stderr)
	assert.NoErr(t, app.RunWithArgs([]string{"cache", "clean", "--keep-latest", "--older="}))
	assert.NoErr(t, app.RunWithArgs([]string{"cache", "clean"}))

	first := calls[0].options.(*CacheCleanOptions)
	second := calls[1].options.(*CacheCleanOptions)
	assert.True(t, first.KeepLatest)
	assert.Eq(t, "", first.Older)
	assert.False(t, second.KeepLatest)
	assert.Eq(t, "", second.Older)
}
