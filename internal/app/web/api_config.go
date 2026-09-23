package web

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gookit/rux/v2"
)

type configUpdateRequest struct {
	Set map[string]string `json:"set"`
}

type configChange struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Current string `json:"current,omitempty"`
}

type configUpdateResponse struct {
	Applied []configChange `json:"applied"`
	Path    string         `json:"path,omitempty"`
}

// handleConfigValidate reports what an update would change without writing.
func (s *Server) handleConfigValidate(c *rux.Context) {
	if !s.requireMutations(c) {
		return
	}
	if s.deps.Config == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	var body configUpdateRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	changes, err := s.previewConfigChanges(body.Set)
	if err != nil {
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_config", err)
		return
	}
	c.JSON(http.StatusOK, configUpdateResponse{Applied: changes})
}

// handleConfigUpdate applies key/value edits. Only known sections may be
// touched and secrets never travel through the API.
func (s *Server) handleConfigUpdate(c *rux.Context) {
	if !s.requireMutations(c) {
		return
	}
	if s.deps.Config == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	var body configUpdateRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	changes, err := s.previewConfigChanges(body.Set)
	if err != nil {
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_config", err)
		return
	}
	for _, change := range changes {
		if err := s.deps.Config.ConfigSet(change.Key, change.Value); err != nil {
			s.writeError(c, http.StatusInternalServerError, "config_set_failed", err)
			return
		}
	}
	resp := configUpdateResponse{Applied: changes}
	if info, infoErr := s.deps.Config.ConfigInfo(); infoErr == nil {
		resp.Path = info.Path
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) previewConfigChanges(set map[string]string) ([]configChange, error) {
	if len(set) == 0 {
		return nil, fmt.Errorf(`provide at least one key in "set"`)
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	changes := make([]configChange, 0, len(keys))
	for _, key := range keys {
		if !configKeyAllowed(key) {
			return nil, fmt.Errorf("config key %q is not editable from the console", key)
		}
		value := set[key]
		if strings.ContainsAny(value, "\x00\n\r") {
			return nil, fmt.Errorf("config value for %q contains an invalid character", key)
		}
		if len(value) > 2000 {
			return nil, fmt.Errorf("config value for %q is too long", key)
		}
		change := configChange{Key: key, Value: value}
		if current, err := s.deps.Config.ConfigGet(key); err == nil && current != nil {
			change.Current = fmt.Sprint(current)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

// configKeyAllowed restricts edits to the sections the CLI already exposes.
// Console-owned and secret keys stay out of reach.
func configKeyAllowed(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 200 {
		return false
	}
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") ||
		strings.HasPrefix(lower, "meta.") ||
		strings.HasPrefix(lower, "web.") {
		return false
	}

	root := lower
	if index := strings.IndexByte(lower, '.'); index >= 0 {
		root = lower[:index]
	}
	switch root {
	case "global", "http_proxy", "api_cache", "ghproxy", "cache_mirror",
		"packages", "pkg", "pkgs", "pkg_templates", "sdk", "ext":
		return true
	default:
		return false
	}
}
