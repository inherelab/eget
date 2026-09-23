package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
	app "github.com/inherelab/eget/internal/app"
)

func sendJSON(t *testing.T, server *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func TestConfigUpdateRequiresMutations(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := sendJSON(t, server, http.MethodPut, "/api/config", `{"set":{"global.target":"~/.local/bin"}}`)

	assert.Eq(t, http.StatusForbidden, rec.Code)
}

func TestConfigUpdateValidatesInput(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	cases := []struct{ name, body string }{
		{"console-owned key", `{"set":{"web.token":"x"}}`},
		{"secret key", `{"set":{"global.api_token":"x"}}`},
		{"unknown section", `{"set":{"nope":"x"}}`},
		{"empty set", `{"set":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := sendJSON(t, server, http.MethodPut, "/api/config", tc.body)
			assert.Eq(t, http.StatusUnprocessableEntity, rec.Code)
		})
	}
}

func TestConfigUpdateAppliesAllowedKeys(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})
	cfg := &fakeConfig{info: app.ConfigInfoResult{Path: "/tmp/eget.toml", Exists: true}, content: "[global]\n"}
	server.deps.Config = cfg

	rec := sendJSON(t, server, http.MethodPut, "/api/config",
		`{"set":{"global.proxy_url":"http://127.0.0.1:10801","ext.npm.bin":"npm-custom"}}`)
	assert.Eq(t, http.StatusOK, rec.Code)

	var resp configUpdateResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Eq(t, 2, len(resp.Applied))
	// Keys are applied in a stable order so the response is deterministic.
	assert.Eq(t, "ext.npm.bin", resp.Applied[0].Key)
	assert.Eq(t, "npm-custom", cfg.values["ext.npm.bin"])
	assert.Eq(t, "http://127.0.0.1:10801", cfg.values["global.proxy_url"])
	assert.Eq(t, "/tmp/eget.toml", resp.Path)
}

func TestConfigValidateDoesNotWrite(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})
	cfg := &fakeConfig{info: app.ConfigInfoResult{Path: "/tmp/eget.toml", Exists: true}}
	server.deps.Config = cfg

	rec := sendJSON(t, server, http.MethodPost, "/api/config/validate", `{"set":{"global.target":"~/.local/bin"}}`)

	assert.Eq(t, http.StatusOK, rec.Code)
	assert.Eq(t, 0, len(cfg.values))
	var resp configUpdateResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Eq(t, "global.target", resp.Applied[0].Key)
}
