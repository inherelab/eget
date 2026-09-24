package web

import (
	"net/http"
	"testing"

	"github.com/gookit/goutil/x/assert"
	app "github.com/inherelab/eget/internal/app"
)

func TestOutdatedScopeSelectsManagerSelection(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	var seen []app.ManagersSelection
	server.deps.ListWithManagers = func(selection app.ManagersSelection) ListProvider {
		seen = append(seen, selection)
		return server.deps.List
	}

	// Default checks everything, matching the console's own package list.
	assert.Eq(t, http.StatusOK, get(t, server, "/api/outdated", authed).Code)
	assert.Eq(t, app.ManagersModeWith, seen[0].Mode)

	// eget-only skips the external managers entirely.
	assert.Eq(t, http.StatusOK, get(t, server, "/api/outdated?scope=eget", authed).Code)
	assert.Eq(t, app.ManagersModeOff, seen[1].Mode)

	// A named manager is canonicalized against the registry.
	assert.Eq(t, http.StatusOK, get(t, server, "/api/outdated?scope=ext&manager=NPM", authed).Code)
	assert.Eq(t, app.ManagersModeOnly, seen[2].Mode)
	assert.Eq(t, []string{"npm"}, seen[2].Managers)

	// Without a name, every configured manager takes part.
	assert.Eq(t, http.StatusOK, get(t, server, "/api/outdated?scope=ext", authed).Code)
	assert.Eq(t, app.ManagersModeOnly, seen[3].Mode)
	assert.Eq(t, 0, len(seen[3].Managers))

	assert.Eq(t, http.StatusBadRequest, get(t, server, "/api/outdated?scope=nope", authed).Code)
	assert.Eq(t, http.StatusUnprocessableEntity, get(t, server, "/api/outdated?scope=ext&manager=nope", authed).Code)
}
