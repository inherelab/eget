package web

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func waitForStatus(t *testing.T, engine *Engine, id string, want TaskStatus) Task {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		task, ok := engine.Get(id)
		if ok && task.Status == want {
			return task
		}
		time.Sleep(5 * time.Millisecond)
	}
	task, _ := engine.Get(id)
	t.Fatalf("task %s did not reach %s (status %s)", id, want, task.Status)
	return Task{}
}

func TestEngineRunsTasksInOrderAndPersists(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "tasks.json")

	var mu sync.Mutex
	var seen []string
	runners := map[string]TaskRunner{
		"demo": func(_ context.Context, params map[string]any, report *TaskReporter) (any, error) {
			name := TaskParamString(params, "name")
			mu.Lock()
			seen = append(seen, name)
			mu.Unlock()
			report.Info("running %s", name)
			report.Progress(50, "halfway")
			return map[string]any{"name": name}, nil
		},
	}

	engine := NewEngine(storePath, runners)
	defer engine.Close()

	first, err := engine.Submit("demo", map[string]any{"name": "one"})
	assert.NoErr(t, err)
	waitForStatus(t, engine, first, StatusSucceeded)

	second, err := engine.Submit("demo", map[string]any{"name": "two"})
	assert.NoErr(t, err)
	task := waitForStatus(t, engine, second, StatusSucceeded)

	assert.Eq(t, []string{"one", "two"}, seen)
	assert.Eq(t, "demo", task.Kind)
	assert.True(t, task.Progress.Percent == 100)
	assert.Eq(t, 1, len(task.Logs))
	assert.True(t, task.FinishedAt != nil)

	// History is written to disk so a restart can show finished tasks.
	data, err := os.ReadFile(storePath)
	assert.NoErr(t, err)
	assert.StrContains(t, string(data), `"one"`)
}

func TestEngineMarksRunningTasksInterruptedAfterRestart(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "tasks.json")
	body := `{"schema":1,"tasks":[{"id":"t_1","kind":"demo","status":"running","createdAt":"2026-09-23T00:00:00Z"}]}`
	assert.NoErr(t, os.WriteFile(storePath, []byte(body), 0o644))

	engine := NewEngine(storePath, map[string]TaskRunner{
		"demo": func(context.Context, map[string]any, *TaskReporter) (any, error) { return nil, nil },
	})
	defer engine.Close()

	task, ok := engine.Get("t_1")
	assert.True(t, ok)
	assert.Eq(t, StatusInterrupted, task.Status)
	assert.StrContains(t, task.Error, "stopped")
}

func TestEngineCancelCancelableTask(t *testing.T) {
	dir := t.TempDir()
	started := make(chan struct{})
	runners := map[string]TaskRunner{
		"block": func(ctx context.Context, _ map[string]any, _ *TaskReporter) (any, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	engine := NewEngine(filepath.Join(dir, "tasks.json"), runners)
	defer engine.Close()

	id, err := engine.Submit("block", nil)
	assert.NoErr(t, err)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("task never started")
	}

	assert.NoErr(t, engine.Cancel(id))
	waitForStatus(t, engine, id, StatusCanceled)
}

// A runner that ignores its context (a prompt on stdin, a package manager
// command) must not hold the queue: cancel has to take effect, the next task has
// to run, and the blocked runner's late outcome must be dropped.
func TestEngineCancelReleasesTheQueueWhenTheRunnerIgnoresIt(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runners := map[string]TaskRunner{
		"block": func(context.Context, map[string]any, *TaskReporter) (any, error) {
			close(started)
			<-release
			return "late", nil
		},
		"quick": func(context.Context, map[string]any, *TaskReporter) (any, error) { return "done", nil },
	}
	engine := NewEngine("", runners)
	defer engine.Close()

	first, err := engine.Submit("block", nil)
	assert.NoErr(t, err)
	second, err := engine.Submit("quick", nil)
	assert.NoErr(t, err)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked task never started")
	}

	assert.NoErr(t, engine.Cancel(first))
	canceled, ok := engine.Get(first)
	assert.True(t, ok)
	assert.Eq(t, StatusCanceled, canceled.Status)

	// The queue moves on even though the blocked runner never returned.
	waitForStatus(t, engine, second, StatusSucceeded)

	// When it finally returns, its outcome is ignored.
	close(release)
	time.Sleep(50 * time.Millisecond)
	still, ok := engine.Get(first)
	assert.True(t, ok)
	assert.Eq(t, StatusCanceled, still.Status)
}

func TestEngineRejectsUnknownKindAndCancelOfFinished(t *testing.T) {
	engine := NewEngine("", map[string]TaskRunner{
		"demo": func(context.Context, map[string]any, *TaskReporter) (any, error) { return nil, nil },
	})
	defer engine.Close()

	_, err := engine.Submit("nope", nil)
	assert.Err(t, err)
	assert.StrContains(t, err.Error(), "unknown task kind")

	id, err := engine.Submit("demo", nil)
	assert.NoErr(t, err)
	waitForStatus(t, engine, id, StatusSucceeded)

	assert.Err(t, engine.Cancel(id))
}

func TestEngineFailedTaskRecordsError(t *testing.T) {
	engine := NewEngine("", map[string]TaskRunner{
		"boom": func(context.Context, map[string]any, *TaskReporter) (any, error) {
			return nil, os.ErrPermission
		},
	})
	defer engine.Close()

	id, err := engine.Submit("boom", nil)
	assert.NoErr(t, err)
	task := waitForStatus(t, engine, id, StatusFailed)
	assert.StrContains(t, task.Error, "permission")
}

func TestEngineCounts(t *testing.T) {
	release := make(chan struct{})
	engine := NewEngine("", map[string]TaskRunner{
		"block": func(context.Context, map[string]any, *TaskReporter) (any, error) {
			<-release
			return nil, nil
		},
	})
	defer func() {
		close(release)
		engine.Close()
	}()

	id, err := engine.Submit("block", nil)
	assert.NoErr(t, err)
	second, err := engine.Submit("block", nil)
	assert.NoErr(t, err)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		running, queued := engine.Counts()
		if running == 1 && queued == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("unexpected counts for tasks %s and %s", id, second)
}
