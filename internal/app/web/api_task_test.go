package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gookit/goutil/x/assert"
)

func postJSON(t *testing.T, server *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func TestTaskEndpointsRequireMutations(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := postJSON(t, server, "/api/update", `{"targets":["sharkdp/fd"]}`)

	assert.Eq(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "read_only")
}

func TestSubmitUpdateValidatesTargets(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	cases := []struct {
		name string
		body string
		code int
	}{
		{"flag-like target", `{"targets":["-rf"]}`, http.StatusUnprocessableEntity},
		{"path traversal", `{"targets":["a/../../etc"]}`, http.StatusUnprocessableEntity},
		{"backslash", `{"targets":["a\\b"]}`, http.StatusUnprocessableEntity},
		{"nothing to do", `{}`, http.StatusUnprocessableEntity},
		{"invalid json", `{`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, server, "/api/update", tc.body)
			assert.Eq(t, tc.code, rec.Code)
		})
	}
}

func TestSubmitUpdateQueuesTaskAndExposesIt(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	rec := postJSON(t, server, "/api/update", `{"targets":["sharkdp/fd","npm:typescript"]}`)
	assert.Eq(t, http.StatusAccepted, rec.Code)

	var accepted taskAccepted
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &accepted))
	assert.Eq(t, "update", accepted.Kind)
	assert.Eq(t, StatusQueued, accepted.Status)
	assert.True(t, accepted.TaskID != "")

	// The task finishes quickly; the detail endpoint must report the result.
	var task Task
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		detail := get(t, server, "/api/tasks/"+accepted.TaskID, func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer secret")
		})
		assert.Eq(t, http.StatusOK, detail.Code)
		assert.NoErr(t, json.Unmarshal(detail.Body.Bytes(), &task))
		if task.Status == StatusSucceeded || task.Status == StatusFailed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.Eq(t, StatusSucceeded, task.Status)
	assert.Eq(t, "update", task.Kind)

	list := get(t, server, "/api/tasks", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer secret")
	})
	assert.Eq(t, http.StatusOK, list.Code)
	var listed tasksResponse
	assert.NoErr(t, json.Unmarshal(list.Body.Bytes(), &listed))
	assert.Eq(t, 1, listed.Total)
}

func TestTaskDetailAndCancelErrors(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})
	auth := func(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") }

	assert.Eq(t, http.StatusNotFound, get(t, server, "/api/tasks/missing", auth).Code)

	rec := postJSON(t, server, "/api/update", `{"all":true}`)
	assert.Eq(t, http.StatusAccepted, rec.Code)
	var accepted taskAccepted
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &accepted))

	// Cancel of an already finished task is a conflict, not a crash.
	time.Sleep(30 * time.Millisecond)
	cancelRec := postJSON(t, server, "/api/tasks/"+accepted.TaskID+"/cancel", `{}`)
	assert.Eq(t, http.StatusConflict, cancelRec.Code)
}

func TestSubmitExtUpgradeValidatesManager(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	unknown := postJSON(t, server, "/api/ext/upgrade", `{"manager":"nope","names":["x"]}`)
	assert.Eq(t, http.StatusUnprocessableEntity, unknown.Code)

	// Manager names are matched case-insensitively against the registry and the
	// canonical spelling is queued.
	ok := postJSON(t, server, "/api/ext/upgrade", `{"manager":"NPM","names":["typescript"]}`)
	assert.Eq(t, http.StatusAccepted, ok.Code)
}

func TestSubmitCacheCleanValidatesRequest(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	badMode := postJSON(t, server, "/api/cache/clean", `{"mode":"nuke"}`)
	assert.Eq(t, http.StatusUnprocessableEntity, badMode.Code)

	badKind := postJSON(t, server, "/api/cache/clean", `{"kinds":["everything"]}`)
	assert.Eq(t, http.StatusUnprocessableEntity, badKind.Code)

	badDays := postJSON(t, server, "/api/cache/clean", `{"days":99999}`)
	assert.Eq(t, http.StatusUnprocessableEntity, badDays.Code)

	ok := postJSON(t, server, "/api/cache/clean", `{"mode":"older","days":7,"kinds":["pkg","sdk"],"dryRun":true}`)
	assert.Eq(t, http.StatusAccepted, ok.Code)
}

func TestSubmitUninstallValidatesTarget(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})

	assert.Eq(t, http.StatusUnprocessableEntity, postJSON(t, server, "/api/uninstall", `{"target":""}`).Code)
	assert.Eq(t, http.StatusAccepted, postJSON(t, server, "/api/uninstall", `{"target":"fd","purge":true}`).Code)
}

func TestUnknownTaskKindIsRejected(t *testing.T) {
	server := testServer(t, Options{Token: "secret", AllowMutations: true})
	server.tasks.runners = map[string]TaskRunner{}

	rec := postJSON(t, server, "/api/update", `{"all":true}`)

	assert.Eq(t, http.StatusConflict, rec.Code)
}
