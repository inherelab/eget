package install

import (
	"context"
	"errors"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func TestRunStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner := &InstallRunner{Service: &Service{}}
	_, err := runner.Run("owner/repo", Options{Context: ctx})

	assert.Err(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestContextErrIsNilWithoutContext(t *testing.T) {
	assert.NoErr(t, Options{}.contextErr())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Err(t, Options{Context: ctx}.contextErr())
}
