package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gookit/rux/v2/pkg/sse"
	"github.com/inherelab/eget/internal/util/atomicfile"
)

const (
	taskQueueSize  = 32
	taskHistoryMax = 200
	taskLogMax     = 50
	taskHubBufSize = 64

	taskEventLog      = "log"
	taskEventProgress = "progress"
	taskEventStatus   = "status"
	taskEventDone     = "done"
)

// TaskStatus is the lifecycle state of a console task.
type TaskStatus string

const (
	StatusQueued    TaskStatus = "queued"
	StatusRunning   TaskStatus = "running"
	StatusSucceeded TaskStatus = "succeeded"
	StatusFailed    TaskStatus = "failed"
	StatusCanceled  TaskStatus = "canceled"
	// StatusInterrupted marks a task that was still running when the process
	// exited; the engine cannot resume it after a restart.
	StatusInterrupted TaskStatus = "interrupted"
)

type LogLine struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type TaskProgress struct {
	Percent float64 `json:"percent"`
	Phase   string  `json:"phase,omitempty"`
	Current int64   `json:"current,omitempty"`
	Total   int64   `json:"total,omitempty"`
}

type Task struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Params     map[string]any `json:"params,omitempty"`
	Status     TaskStatus     `json:"status"`
	CreatedAt  time.Time      `json:"createdAt"`
	StartedAt  *time.Time     `json:"startedAt,omitempty"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	Progress   TaskProgress   `json:"progress"`
	Logs       []LogLine      `json:"logs,omitempty"`
	Result     any            `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// TaskRunner performs one queued task. It must honor ctx cancellation and
// return a JSON-serializable result.
type TaskRunner func(ctx context.Context, params map[string]any, report *TaskReporter) (any, error)

// TaskReporter is how a running task pushes log lines and progress.
type TaskReporter struct {
	engine *Engine
	taskID string
}

func (r *TaskReporter) Logf(level, format string, args ...any) {
	r.engine.appendLog(r.taskID, level, fmt.Sprintf(format, args...))
}

func (r *TaskReporter) Info(format string, args ...any) { r.Logf("info", format, args...) }

func (r *TaskReporter) Error(format string, args ...any) { r.Logf("error", format, args...) }

func (r *TaskReporter) Progress(percent float64, phase string) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	r.engine.setProgress(r.taskID, TaskProgress{Percent: percent, Phase: phase})
}

func (r *TaskReporter) ProgressBytes(current, total int64, phase string) {
	percent := 0.0
	if total > 0 {
		percent = float64(current) / float64(total) * 100
	}
	if percent > 100 {
		percent = 100
	}
	r.engine.setProgress(r.taskID, TaskProgress{Percent: percent, Phase: phase, Current: current, Total: total})
}

// Engine runs one task at a time — the store and external managers do not
// tolerate concurrent writers — and publishes every state change to SSE
// subscribers through a keyed hub.
type Engine struct {
	mu        sync.Mutex
	tasks     map[string]*Task
	order     []string
	queue     chan *queuedTask
	runners   map[string]TaskRunner
	storePath string
	now       func() time.Time
	closed    bool

	hub *sse.Hub

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc

	workerWG sync.WaitGroup
}

type queuedTask struct {
	id     string
	runner TaskRunner
	params map[string]any
}

// NewEngine restores persisted task history and starts the single worker.
func NewEngine(storePath string, runners map[string]TaskRunner) *Engine {
	engine := &Engine{
		tasks:     map[string]*Task{},
		queue:     make(chan *queuedTask, taskQueueSize),
		runners:   runners,
		storePath: storePath,
		now:       time.Now,
		hub:       sse.NewHub(taskHubBufSize),
		cancels:   map[string]context.CancelFunc{},
	}
	engine.load()
	engine.workerWG.Add(1)
	go engine.worker()
	return engine
}

// Hub exposes the SSE hub used by /api/tasks/{id}/events.
func (e *Engine) Hub() *sse.Hub {
	return e.hub
}

// Close cancels running tasks and stops the worker. Queued tasks are dropped.
func (e *Engine) Close() {
	e.cancelMu.Lock()
	for _, cancel := range e.cancels {
		cancel()
	}
	e.cancelMu.Unlock()

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	close(e.queue)
	e.mu.Unlock()

	e.workerWG.Wait()
}

// Submit queues a task of the given kind and returns its id.
func (e *Engine) Submit(kind string, params map[string]any) (string, error) {
	runner, ok := e.runners[kind]
	if !ok {
		return "", fmt.Errorf("unknown task kind %q", kind)
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return "", fmt.Errorf("the task engine is shutting down")
	}
	task := &Task{
		ID:        e.newTaskIDLocked(),
		Kind:      kind,
		Params:    params,
		Status:    StatusQueued,
		CreatedAt: e.now(),
	}
	e.tasks[task.ID] = task
	e.order = append(e.order, task.ID)
	e.trimLocked()
	e.mu.Unlock()
	e.persist()

	select {
	case e.queue <- &queuedTask{id: task.ID, runner: runner, params: params}:
	default:
		e.mu.Lock()
		e.finishLocked(task.ID, StatusFailed, nil, "the task queue is full")
		e.mu.Unlock()
		e.persist()
		e.publishDone(task.ID)
		return "", fmt.Errorf("the task queue is full; wait for the running task to finish")
	}

	e.publishStatus(task.ID)
	return task.ID, nil
}

// Cancel stops a queued or running task.
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	task, ok := e.tasks[id]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("task %s not found", id)
	}
	status := task.Status
	e.mu.Unlock()

	switch status {
	case StatusQueued:
		e.mu.Lock()
		if current, ok := e.tasks[id]; ok && current.Status == StatusQueued {
			e.finishLocked(id, StatusCanceled, nil, "canceled before it started")
		}
		e.mu.Unlock()
		e.persist()
		e.publishDone(id)
		return nil
	case StatusRunning:
		e.cancelMu.Lock()
		cancel := e.cancels[id]
		e.cancelMu.Unlock()
		if cancel != nil {
			cancel()
		}
		return nil
	default:
		return fmt.Errorf("task %s already finished", id)
	}
}

// Get returns a copy of one task.
func (e *Engine) Get(id string) (Task, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	task, ok := e.tasks[id]
	if !ok {
		return Task{}, false
	}
	return copyTask(task), true
}

// List returns tasks newest first, capped at limit when limit > 0.
func (e *Engine) List(limit int) []Task {
	e.mu.Lock()
	defer e.mu.Unlock()

	tasks := e.snapshotLocked()
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[len(tasks)-limit:]
	}
	for i, j := 0, len(tasks)-1; i < j; i, j = i+1, j-1 {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	}
	return tasks
}

// Counts reports running and queued task counts for /api/overview.
func (e *Engine) Counts() (running int, queued int) {
	if e == nil {
		return 0, 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, task := range e.tasks {
		switch task.Status {
		case StatusRunning:
			running++
		case StatusQueued:
			queued++
		}
	}
	return running, queued
}

func (e *Engine) worker() {
	defer e.workerWG.Done()
	for job := range e.queue {
		e.run(job)
	}
}

func (e *Engine) run(job *queuedTask) {
	ctx, cancel := context.WithCancel(context.Background())
	e.cancelMu.Lock()
	e.cancels[job.id] = cancel
	e.cancelMu.Unlock()
	defer func() {
		e.cancelMu.Lock()
		delete(e.cancels, job.id)
		e.cancelMu.Unlock()
		cancel()
	}()

	e.mu.Lock()
	task, ok := e.tasks[job.id]
	if !ok || task.Status == StatusCanceled {
		e.mu.Unlock()
		return
	}
	started := e.now()
	task.Status = StatusRunning
	task.StartedAt = &started
	e.mu.Unlock()
	e.persist()
	e.publishStatus(job.id)

	report := &TaskReporter{engine: e, taskID: job.id}
	result, err := job.runner(ctx, job.params, report)

	e.mu.Lock()
	switch {
	case ctx.Err() != nil:
		e.finishLocked(job.id, StatusCanceled, nil, "canceled while running")
	case err != nil:
		e.finishLocked(job.id, StatusFailed, nil, err.Error())
	default:
		e.finishLocked(job.id, StatusSucceeded, result, "")
	}
	e.mu.Unlock()
	e.persist()
	e.publishDone(job.id)
}

func (e *Engine) appendLog(id, level, message string) {
	e.mu.Lock()
	task, ok := e.tasks[id]
	if !ok {
		e.mu.Unlock()
		return
	}
	task.Logs = append(task.Logs, LogLine{Time: e.now(), Level: level, Message: message})
	if len(task.Logs) > taskLogMax {
		task.Logs = append([]LogLine(nil), task.Logs[len(task.Logs)-taskLogMax:]...)
	}
	e.mu.Unlock()

	e.publish(taskEventLog, id, map[string]any{"level": level, "message": message})
}

func (e *Engine) setProgress(id string, progress TaskProgress) {
	e.mu.Lock()
	task, ok := e.tasks[id]
	if !ok {
		e.mu.Unlock()
		return
	}
	task.Progress = progress
	e.mu.Unlock()

	e.publish(taskEventProgress, id, progress)
}

func (e *Engine) finishLocked(id string, status TaskStatus, result any, message string) {
	task, ok := e.tasks[id]
	if !ok {
		return
	}
	finished := e.now()
	task.Status = status
	task.FinishedAt = &finished
	task.Result = result
	task.Error = message
	switch status {
	case StatusSucceeded:
		task.Progress = TaskProgress{Percent: 100, Phase: "done"}
	case StatusCanceled, StatusInterrupted:
		task.Progress.Phase = string(status)
	}
}

func (e *Engine) publishStatus(id string) {
	e.mu.Lock()
	task, ok := e.tasks[id]
	if !ok {
		e.mu.Unlock()
		return
	}
	payload := map[string]any{"taskId": id, "kind": task.Kind, "status": task.Status}
	e.mu.Unlock()
	e.publish(taskEventStatus, id, payload)
}

func (e *Engine) publishDone(id string) {
	e.mu.Lock()
	task, ok := e.tasks[id]
	if !ok {
		e.mu.Unlock()
		return
	}
	payload := map[string]any{
		"taskId": id,
		"status": task.Status,
		"result": task.Result,
		"error":  task.Error,
	}
	e.mu.Unlock()
	e.publish(taskEventStatus, id, payload)
	e.publish(taskEventDone, id, payload)
}

func (e *Engine) publish(name, id string, payload any) {
	if e == nil || e.hub == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	e.hub.Send(id, sse.Event{Name: name, Data: string(data)})
}

func (e *Engine) snapshotLocked() []Task {
	out := make([]Task, 0, len(e.order))
	for _, id := range e.order {
		if task, ok := e.tasks[id]; ok {
			out = append(out, copyTask(task))
		}
	}
	return out
}

func copyTask(task *Task) Task {
	copied := *task
	copied.Logs = append([]LogLine(nil), task.Logs...)
	return copied
}

func (e *Engine) trimLocked() {
	if len(e.order) <= taskHistoryMax {
		return
	}
	drop := len(e.order) - taskHistoryMax
	for _, id := range e.order[:drop] {
		delete(e.tasks, id)
	}
	e.order = append([]string(nil), e.order[drop:]...)
}

func (e *Engine) newTaskIDLocked() string {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("t_%d", e.now().UnixNano())
	}
	return fmt.Sprintf("t_%d_%s", e.now().Unix(), hex.EncodeToString(buf))
}

type persistedTasks struct {
	Schema int    `json:"schema"`
	Tasks  []Task `json:"tasks"`
}

func (e *Engine) load() {
	if e.storePath == "" {
		return
	}
	data, err := os.ReadFile(e.storePath)
	if err != nil {
		return
	}
	var stored persistedTasks
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for index := range stored.Tasks {
		task := stored.Tasks[index]
		if task.ID == "" {
			continue
		}
		if task.Status == StatusRunning || task.Status == StatusQueued {
			task.Status = StatusInterrupted
			task.Progress.Phase = string(StatusInterrupted)
			if task.Error == "" {
				task.Error = "eget web stopped while this task was running"
			}
		}
		copied := task
		e.tasks[copied.ID] = &copied
		e.order = append(e.order, copied.ID)
	}
	e.trimLocked()
}

func (e *Engine) persist() {
	if e.storePath == "" {
		return
	}
	e.mu.Lock()
	tasks := e.snapshotLocked()
	e.mu.Unlock()

	data, err := json.MarshalIndent(persistedTasks{Schema: 1, Tasks: tasks}, "", "  ")
	if err != nil {
		return
	}
	_ = atomicfile.WriteFile(e.storePath, append(data, '\n'), 0o644)
}
