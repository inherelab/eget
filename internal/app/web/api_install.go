package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gookit/rux/v2"
)

type installRequest struct {
	Target       string `json:"target"`
	Version      string `json:"version"`
	Asset        string `json:"asset"`
	Output       string `json:"output"`
	File         string `json:"file"`
	ExtractAll   bool   `json:"extractAll"`
	DownloadOnly bool   `json:"downloadOnly"`
	AddToConfig  bool   `json:"addToConfig"`
}

type installCandidatesResponse struct {
	Target      string   `json:"target"`
	Candidates  []string `json:"candidates"`
	NeedsChoice bool     `json:"needsChoice"`
}

type sdkRequest struct {
	Target  string   `json:"target"`
	Targets []string `json:"targets"`
}

// handleInstallCandidates resolves matching assets without downloading, so the
// console can ask which one to install instead of guessing.
func (s *Server) handleInstallCandidates(c *rux.Context) {
	if s.deps.AssetCandidates == nil {
		s.writeError(c, http.StatusNotImplemented, "not_available", nil)
		return
	}
	target := strings.TrimSpace(c.Query("target"))
	if err := validateTarget(target); err != nil {
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_target", err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Req.Context(), extCommandTimeout)
	defer cancel()

	candidates, err := s.deps.AssetCandidates(ctx, target)
	if err != nil {
		s.writeError(c, http.StatusBadGateway, "candidates_failed", err)
		return
	}
	c.JSON(http.StatusOK, installCandidatesResponse{
		Target:      target,
		Candidates:  candidates,
		NeedsChoice: len(candidates) > 1,
	})
}

func (s *Server) handleSubmitInstall(c *rux.Context) {
	if !s.requireMutations(c) {
		return
	}
	var body installRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	target := strings.TrimSpace(body.Target)
	if err := validateTarget(target); err != nil {
		s.writeError(c, http.StatusUnprocessableEntity, "invalid_target", err)
		return
	}
	for field, value := range map[string]string{
		"version": body.Version,
		"asset":   body.Asset,
		"output":  body.Output,
		"file":    body.File,
	} {
		if err := validateOptionalText(value, field); err != nil {
			s.writeError(c, http.StatusUnprocessableEntity, "invalid_"+field, err)
			return
		}
	}
	s.submitTask(c, "install", map[string]any{
		"target":       target,
		"version":      strings.TrimSpace(body.Version),
		"asset":        strings.TrimSpace(body.Asset),
		"output":       strings.TrimSpace(body.Output),
		"file":         strings.TrimSpace(body.File),
		"extractAll":   body.ExtractAll,
		"downloadOnly": body.DownloadOnly,
		"addToConfig":  body.AddToConfig,
	})
}

func (s *Server) handleSubmitSDKInstall(c *rux.Context) {
	s.submitSDKTask(c, "sdk.install")
}

func (s *Server) handleSubmitSDKDownload(c *rux.Context) {
	s.submitSDKTask(c, "sdk.download")
}

func (s *Server) submitSDKTask(c *rux.Context, kind string) {
	if !s.requireMutations(c) {
		return
	}
	var body sdkRequest
	if err := c.BindJSON(&body); err != nil {
		s.writeError(c, http.StatusBadRequest, "invalid_body", err)
		return
	}
	targets := append([]string(nil), body.Targets...)
	if strings.TrimSpace(body.Target) != "" {
		targets = append(targets, body.Target)
	}
	clean := make([]string, 0, len(targets))
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if err := validateTarget(target); err != nil {
			s.writeError(c, http.StatusUnprocessableEntity, "invalid_target", err)
			return
		}
		clean = append(clean, target)
	}
	if len(clean) == 0 {
		s.writeError(c, http.StatusUnprocessableEntity, "missing_target",
			fmt.Errorf("at least one SDK target is required"))
		return
	}
	s.submitTask(c, kind, map[string]any{"targets": clean})
}

func validateOptionalText(value, field string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\x00\n\r\t") {
		return fmt.Errorf("%s contains an invalid character", field)
	}
	if len(value) > 500 {
		return fmt.Errorf("%s is too long", field)
	}
	return nil
}
