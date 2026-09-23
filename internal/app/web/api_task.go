package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gookit/rux/v2"
	"github.com/gookit/rux/v2/pkg/sse"
	appcache "github.com/inherelab/eget/internal/app/cache"
)

// Write endpoints accept structured JSON only. Nothing here reaches a shell:
// targets and manager names pass strict validation and manager names must exist
// in the external-manager registry, so a value can never smuggle extra argv
// into npm/cargo/scoop and friends.

type updateRequest struct {
	Targets []string `json:"targets"`
	All     bool     `json:"all"`
}

type uninstallRequest struct {
	Target string `json:"target"`
	Purge  bool   `json:"purge"`
}

type extUpgradeRequest struct {
	Manager string   `json:"manager"`
	Names   []string `json:"names"`
}

type cacheCleanRequest struct {
	Mode   string   `json:"mode"`
	Days   int      `json:"days"`
	Kinds  []string `json:"kinds"`
	DryRun bool     `json:"dryRun"`
}

type taskAccepted struct {
	TaskID string     `json:"taskId"`
	Kind   string     `json:"kind"`
	Status TaskStatus `json:"status"`
}

type tasksResponse struct {
	Total int    `json:"total"`
	Items []Task `json:"items"`
}

func (s *Server) handleSubmitUpdate(c *rux.Context) {
	if !s.requireMutations(c) {
		return
	}
	var body updateRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	targets := make([]string, 0, len(body.Targets))
	for _, target := range body.Targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if err := validateTarget(target); err != nil {
			s.writeError(c, http.StatusUnprocessableEntity, "invalid_target", err)
			return
		}
		targets = append(targets, target)
	}
	if len(targets) == 0 && !body.All {
		s.writeError(c, http.StatusUnprocessableEntity, "missing_target",
			fmt.Errorf("provide targets, or set all to true"))
		return
	}
	s.submitTask(c, "update", map[string]any{"targets": targets, "all": body.All})
}

func (s *Server) handleSubmitUninstall(c *rux.Context) {
	if !s.requireMutations(c) {
		return
	}
	var body uninstallRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	target := strings.TrimSpace(body.Target)
	if err := validateTarget(target); err != nil {
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_target", err)
		return
	}
	s.submitTask(c, "uninstall", map[string]any{"target": target, "purge": body.Purge})
}

func (s *Server) handleSubmitExtUpgrade(c *rux.Context) {
	if !s.requireMutations(c) {
		return
	}
	var body extUpgradeRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	manager, err := s.externalManager(strings.TrimSpace(body.Manager))
	if err != nil {
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_manager", err)
		return
	}
	names := make([]string, 0, len(body.Names))
	for _, name := range body.Names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := validateTarget(name); err != nil {
			s.writeError(c, http.StatusUnprocessableEntity, "invalid_package", err)
			return
		}
		names = append(names, name)
	}
	s.submitTask(c, "ext.upgrade", map[string]any{"manager": manager, "names": names})
}

func (s *Server) handleSubmitCacheClean(c *rux.Context) {
	if !s.requireMutations(c) {
		return
	}
	var body cacheCleanRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	mode := strings.TrimSpace(body.Mode)
	switch mode {
	case "", "older", "all", "keep-latest":
	default:
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_mode",
			fmt.Errorf("mode must be older, all or keep-latest, got %q", mode))
		return
	}
	kinds := make([]string, 0, len(body.Kinds))
	for _, kind := range body.Kinds {
		kind = strings.TrimSpace(kind)
		if kind == "" {
			continue
		}
		if !isCleanKind(kind) {
			s.writeError(c, http.StatusUnprocessableEntity, "invalid_kind",
				fmt.Errorf("unknown cache kind %q", kind))
			return
		}
		kinds = append(kinds, kind)
	}
	days := body.Days
	if days < 0 || days > 3650 {
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_days",
			fmt.Errorf("days must be between 0 and 3650, got %d", days))
		return
	}
	s.submitTask(c, "cache.clean", map[string]any{
		"mode":  mode,
		"days":  days,
		"kinds": kinds,
		// The console always previews before it deletes: the runner only
		// removes files when confirm is true.
		"confirm": !body.DryRun,
	})
}

func (s *Server) handleTasks(c *rux.Context) {
	if s.tasks == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	limit := queryInt(c, "limit", 50)
	if limit < 0 {
		limit = 0
	}
	items := s.tasks.List(limit)
	c.JSON(http.StatusOK, tasksResponse{Total: len(items), Items: items})
}

func (s *Server) handleTask(c *rux.Context) {
	if s.tasks == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	task, ok := s.tasks.Get(strings.TrimSpace(c.Param("id")))
	if !ok {
		s.writeError(c, http.StatusNotFound, "task_not_found",
			fmt.Errorf("task %s not found", c.Param("id")))
		return
	}
	c.JSON(http.StatusOK, task)
}

func (s *Server) handleTaskCancel(c *rux.Context) {
	if s.tasks == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	if err := s.tasks.Cancel(strings.TrimSpace(c.Param("id"))); err != nil {
		s.writeError(c, http.StatusConflict, "cancel_failed", err)
		return
	}
	c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// handleTaskEvents streams task events. The engine pushes progress and log
// frames through a hub keyed by task id, so several tabs can watch one task.
func (s *Server) handleTaskEvents(c *rux.Context) {
	if s.tasks == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if _, ok := s.tasks.Get(id); !ok {
		s.writeError(c, http.StatusNotFound, "task_not_found", fmt.Errorf("task %s not found", id))
		return
	}
	if err := sse.StreamWith(c, &sse.Options{
		SendConnected:     true,
		KeepaliveInterval: 25 * time.Second,
	}, sse.HubProducer(s.tasks.Hub(), id)); err != nil {
		s.logf("task stream %s: %v", id, err)
	}
}

func (s *Server) submitTask(c *rux.Context, kind string, params map[string]any) {
	if s.tasks == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available",
			fmt.Errorf("task kind %q is not available in this build", kind))
		return
	}
	id, err := s.tasks.Submit(kind, params)
	if err != nil {
		s.writeError(c, http.StatusConflict, "submit_failed", err)
		return
	}
	c.JSON(http.StatusAccepted, taskAccepted{TaskID: id, Kind: kind, Status: StatusQueued})
}

func (s *Server) requireMutations(c *rux.Context) bool {
	if s.MutationsEnabled() {
		return true
	}
	s.writeError(c, http.StatusForbidden, "read_only",
		fmt.Errorf("this console is read-only; start it with --allow-mutations on a non-loopback host"))
	return false
}

func (s *Server) externalManager(name string) (string, error) {
	if s.deps.Ext == nil {
		return "", fmt.Errorf("no external manager is available")
	}
	if name == "" {
		return "", fmt.Errorf("manager is required")
	}
	for _, candidate := range s.deps.Ext.Names() {
		if strings.EqualFold(candidate, name) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unknown manager %q", name)
}

// validateTarget keeps user input from being read as a flag or escaping into
// the filesystem. Repo targets ("owner/repo") and manager refs ("npm:pkg") are
// both allowed, so only clearly dangerous shapes are rejected.
func validateTarget(target string) error {
	if target == "" {
		return fmt.Errorf("target is required")
	}
	if strings.HasPrefix(target, "-") {
		return fmt.Errorf("target %q must not start with '-'", target)
	}
	if strings.ContainsAny(target, "\\\x00\n\r\t") {
		return fmt.Errorf("target %q contains an invalid character", target)
	}
	if strings.Contains(target, "..") {
		return fmt.Errorf("target %q must not contain '..'", target)
	}
	if len(target) > 300 {
		return fmt.Errorf("target is too long")
	}
	return nil
}

func isCleanKind(kind string) bool {
	switch appcache.Kind(kind) {
	case appcache.KindPkg, appcache.KindAPI, appcache.KindSDK, appcache.KindSDKIndex, appcache.KindPartial:
		return true
	default:
		return false
	}
}

// TaskParamStrings reads a []string parameter that arrived as JSON.
func TaskParamStrings(params map[string]any, key string) []string {
	value, ok := params[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

// TaskParamString reads a string parameter that arrived as JSON.
func TaskParamString(params map[string]any, key string) string {
	if value, ok := params[key].(string); ok {
		return value
	}
	return ""
}

// TaskParamBool reads a boolean parameter that arrived as JSON.
func TaskParamBool(params map[string]any, key string) bool {
	if value, ok := params[key].(bool); ok {
		return value
	}
	return false
}

// TaskParamInt reads a numeric parameter that arrived as JSON (json numbers
// decode into float64).
func TaskParamInt(params map[string]any, key string) int {
	switch typed := params[key].(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		if parsed, err := strconv.Atoi(typed); err == nil {
			return parsed
		}
	}
	return 0
}
