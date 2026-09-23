package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gookit/rux/v2"
	app "github.com/inherelab/eget/internal/app"
	appcache "github.com/inherelab/eget/internal/app/cache"
)

// extCommandTimeout bounds one external manager command run while serving a
// read-only request, so a hung manager cannot pin the HTTP handler forever.
const extCommandTimeout = 30 * time.Second

func (s *Server) handleOverview(c *rux.Context) {
	if s.deps.List == nil || s.deps.Config == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	info, err := s.deps.Config.ConfigInfo()
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, "config_unavailable", err)
		return
	}
	items, err := s.deps.List.ListPackages()
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, "list_failed", err)
		return
	}
	installed := 0
	for _, item := range items {
		if item.Installed {
			installed++
		}
	}

	running, queued := s.opts.TaskCounts()
	resp := overviewResponse{
		Version:      s.opts.Version,
		ConfigPath:   info.Path,
		ConfigExists: info.Exists,
		Packages:     len(items),
		Installed:    installed,
		Ext:          s.extBriefs(),
		Tasks:        taskCounts{Running: running, Queued: queued},
	}
	if s.deps.Cache != nil {
		if status, statusErr := s.deps.Cache.Status(""); statusErr == nil {
			resp.Cache = &cacheBrief{Dir: status.CacheDir, Files: status.TotalFiles, Size: status.TotalSize}
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) extBriefs() []extBrief {
	if s.deps.Ext == nil {
		return nil
	}
	names := s.deps.Ext.Names()
	out := make([]extBrief, 0, len(names))
	for _, name := range names {
		brief := extBrief{Manager: name}
		if manager, ok := s.deps.Ext.Manager(name); ok {
			if bin, ok := s.deps.Ext.Path(manager); ok {
				brief.Available = true
				brief.Bin = bin
			}
		}
		out = append(out, brief)
	}
	return out
}

func (s *Server) handlePackages(c *rux.Context) {
	if s.deps.List == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	scope := strings.TrimSpace(c.Query("scope"))
	switch scope {
	case "", "all", "eget", "ext":
	default:
		s.writeError(c, http.StatusBadRequest, "bad_scope",
			fmt.Errorf("unknown scope %q: use all, eget or ext", scope))
		return
	}

	items, err := s.deps.List.ListPackages()
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, "list_failed", err)
		return
	}

	keyword := strings.ToLower(strings.TrimSpace(c.Query("q")))
	manager := strings.TrimSpace(c.Query("manager"))
	onlyInstalled := queryBool(c, "installed")

	filtered := make([]app.ListItem, 0, len(items))
	for _, item := range items {
		if onlyInstalled && !item.Installed {
			continue
		}
		switch scope {
		case "eget":
			if item.Manager != "" {
				continue
			}
		case "ext":
			if item.Manager == "" {
				continue
			}
		}
		if manager != "" && !strings.EqualFold(item.Manager, manager) {
			continue
		}
		if keyword != "" &&
			!strings.Contains(strings.ToLower(item.Name), keyword) &&
			!strings.Contains(strings.ToLower(item.Repo), keyword) {
			continue
		}
		filtered = append(filtered, item)
	}

	c.JSON(http.StatusOK, packagesResponse{Total: len(filtered), Items: newPackageItems(filtered)})
}

func (s *Server) handlePackageDetail(c *rux.Context) {
	if s.deps.Show == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		s.writeError(c, http.StatusBadRequest, "missing_name", nil)
		return
	}
	result, err := s.deps.Show.ShowPackage(name)
	if err != nil {
		s.writeError(c, http.StatusNotFound, "package_not_found", err)
		return
	}
	c.JSON(http.StatusOK, newPackageDetail(result))
}

func (s *Server) handleOutdated(c *rux.Context) {
	if s.deps.List == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	items, failures, checked, err := s.deps.List.ListOutdatedPackages()
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, "outdated_failed", err)
		return
	}
	c.JSON(http.StatusOK, outdatedResponse{
		Checked:  checked,
		Items:    newOutdatedItems(items),
		Failures: newFailureItems(failures),
	})
}

func (s *Server) handleExtManagers(c *rux.Context) {
	if s.deps.Ext == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	ctx, cancel := context.WithTimeout(c.Req.Context(), extCommandTimeout)
	defer cancel()

	names := s.deps.Ext.Names()
	packages, failures, err := s.deps.Ext.List(ctx, names...)
	counts := make(map[string]int, len(names))
	for _, pkg := range packages {
		counts[pkg.Manager]++
	}

	managers := make([]extManagerItem, 0, len(names))
	for _, name := range names {
		item := extManagerItem{Manager: name, Packages: counts[name]}
		if manager, ok := s.deps.Ext.Manager(name); ok {
			if bin, ok := s.deps.Ext.Path(manager); ok {
				item.Available = true
				item.Bin = bin
			}
		}
		managers = append(managers, item)
	}

	resp := extManagersResponse{Managers: managers}
	if err != nil {
		resp.Failures = append(resp.Failures, failureItem{Name: "ext", Error: err.Error()})
	}
	for _, failure := range failures {
		message := ""
		if failure.Err != nil {
			message = failure.Err.Error()
		}
		resp.Failures = append(resp.Failures, failureItem{Name: failure.Manager, Error: message})
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleExtPackages(c *rux.Context) {
	if s.deps.Ext == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	manager, ok := s.deps.Ext.Manager(strings.TrimSpace(c.Param("manager")))
	if !ok {
		s.writeError(c, http.StatusNotFound, "manager_not_found",
			fmt.Errorf("unknown manager %q", c.Param("manager")))
		return
	}
	ctx, cancel := context.WithTimeout(c.Req.Context(), extCommandTimeout)
	defer cancel()

	packages, _, err := s.deps.Ext.List(ctx, manager.Name)
	if err != nil {
		s.writeError(c, http.StatusBadGateway, "ext_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, extPackagesResponse{
		Manager:  manager.Name,
		Packages: newExtPackageItems(packages),
	})
}

func (s *Server) handleCacheList(c *rux.Context) {
	if s.deps.Cache == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	root := strings.TrimSpace(c.Query("root", s.opts.CacheRoot))
	result, err := s.deps.Cache.List("", appcache.ListOptions{Root: root})
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, "cache_list_failed", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleCacheStatus(c *rux.Context) {
	if s.deps.Cache == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	status, err := s.deps.Cache.Status("")
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, "cache_status_failed", err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (s *Server) handleConfig(c *rux.Context) {
	if s.deps.Config == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	info, err := s.deps.Config.ConfigInfo()
	if err != nil {
		s.writeError(c, http.StatusInternalServerError, "config_unavailable", err)
		return
	}
	content := ""
	if info.Exists {
		var buf bytes.Buffer
		if err := s.deps.Config.ConfigExport(&buf, true); err != nil {
			s.writeError(c, http.StatusInternalServerError, "config_export_failed", err)
			return
		}
		content = buf.String()
	}
	c.JSON(http.StatusOK, configView{Path: info.Path, Exists: info.Exists, Content: content})
}

func (s *Server) handleQuery(c *rux.Context) {
	if s.deps.Query == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	target := firstQuery(c, "target", "repo")
	if target == "" {
		s.writeError(c, http.StatusBadRequest, "missing_target",
			fmt.Errorf("query target is required"))
		return
	}
	result, err := s.deps.Query.Query(app.QueryOptions{
		Repo:       target,
		Action:     strings.TrimSpace(c.Query("action")),
		Tag:        strings.TrimSpace(c.Query("tag")),
		Limit:      queryInt(c, "limit", 0),
		Prerelease: queryBool(c, "prerelease"),
	})
	if err != nil {
		s.writeError(c, http.StatusBadGateway, "query_failed", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleSearch(c *rux.Context) {
	if s.deps.Search == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	keyword := firstQuery(c, "q", "keyword")
	if keyword == "" {
		s.writeError(c, http.StatusBadRequest, "missing_keyword",
			fmt.Errorf("search keyword is required"))
		return
	}
	var extras []string
	for _, part := range strings.Split(c.Query("extras"), ",") {
		if part = strings.TrimSpace(part); part != "" {
			extras = append(extras, part)
		}
	}
	result, err := s.deps.Search.Search(app.SearchOptions{
		Keyword: keyword,
		Extras:  extras,
		Limit:   queryInt(c, "limit", 0),
		Sort:    strings.TrimSpace(c.Query("sort")),
		Order:   strings.TrimSpace(c.Query("order")),
	})
	if err != nil {
		s.writeError(c, http.StatusBadGateway, "search_failed", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) writeError(c *rux.Context, status int, code string, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	c.JSON(status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

func firstQuery(c *rux.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func queryInt(c *rux.Context, key string, def int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return value
}

func queryBool(c *rux.Context, key string) bool {
	switch strings.ToLower(strings.TrimSpace(c.Query(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
