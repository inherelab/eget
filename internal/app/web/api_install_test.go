package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

func authed(r *http.Request) {
	r.Header.Set("Authorization", "Bearer secret")
}

func TestSubmitInstallValidatesInput(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	cases := []struct{ name, body string }{
		{"missing target", `{}`},
		{"flag-like target", `{"target":"-rf"}`},
		{"traversal target", `{"target":"owner/../etc"}`},
		{"control char in output", "{\"target\":\"owner/repo\",\"output\":\"a\\nb\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, server, "/api/install", tc.body)
			assert.Eq(t, http.StatusUnprocessableEntity, rec.Code)
		})
	}
}

func TestSubmitInstallQueuesTask(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	rec := postJSON(t, server, "/api/install",
		`{"target":"owner/repo","version":"v1.2.3","asset":"tool.zip","extractAll":true,"addToConfig":true}`)

	assert.Eq(t, http.StatusAccepted, rec.Code)
	var accepted taskAccepted
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &accepted))
	assert.Eq(t, "install", accepted.Kind)
}

func TestInstallCandidatesEndpoint(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	// Without a provider the endpoint reports it is unavailable.
	assert.Eq(t, http.StatusNotImplemented,
		get(t, server, "/api/install/candidates?target=owner/repo", authed).Code)

	server.deps.AssetCandidates = func(_ context.Context, target string) ([]string, error) {
		assert.Eq(t, "owner/repo", target)
		return []string{"tool-x86_64.msi", "tool-x86_64.zip"}, nil
	}

	rec := get(t, server, "/api/install/candidates?target=owner/repo", authed)
	assert.Eq(t, http.StatusOK, rec.Code)
	var resp installCandidatesResponse
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Eq(t, 2, len(resp.Candidates))
	assert.True(t, resp.NeedsChoice)

	assert.Eq(t, http.StatusUnprocessableEntity,
		get(t, server, "/api/install/candidates?target=-rf", authed).Code)
}

func TestSubmitSDKTaskValidatesTargets(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	assert.Eq(t, http.StatusUnprocessableEntity, postJSON(t, server, "/api/sdk/install", `{}`).Code)
	assert.Eq(t, http.StatusUnprocessableEntity, postJSON(t, server, "/api/sdk/download", `{"targets":["-x"]}`).Code)

	rec := postJSON(t, server, "/api/sdk/install", `{"target":"node@20","targets":["go@1.22"]}`)
	assert.Eq(t, http.StatusAccepted, rec.Code)
	var accepted taskAccepted
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &accepted))
	assert.Eq(t, "sdk.install", accepted.Kind)
}
