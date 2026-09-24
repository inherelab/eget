package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gookit/goutil/x/assert"
)

// The browser furniture must answer without a token: the tab icon loads on the
// unauthenticated token page, and browsers fetch a web manifest with credentials
// omitted. The declared content types are part of the contract because Go's
// built-in mime table knows neither .ico nor .webmanifest, and the wrong type
// makes a browser drop the icon or the manifest.
func TestWebServesBrowserIconsWithoutToken(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	for path, icon := range browserIcons {
		t.Run(path, func(t *testing.T) {
			rec := get(t, server, path, nil)

			assert.Eq(t, http.StatusOK, rec.Code)
			assert.Eq(t, icon.typ, rec.Header().Get("Content-Type"))
			assert.True(t, rec.Body.Len() > 0)
		})
	}
}

// The manifest advertises the install icons by relative path; every one of them
// has to be servable or the install prompt and the tab icon degrade to blanks.
func TestWebBrowserIconsMatchManifest(t *testing.T) {
	server := testServer(t, Options{Token: "secret"})

	rec := get(t, server, "/site.webmanifest", nil)
	var manifest struct {
		Name  string `json:"name"`
		Icons []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
		} `json:"icons"`
	}
	assert.NoErr(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
	assert.Eq(t, "eget", manifest.Name)
	assert.Require(t, assert.True(t, len(manifest.Icons) > 0))

	for _, icon := range manifest.Icons {
		src := "/" + strings.TrimPrefix(icon.Src, "./")
		t.Run(src, func(t *testing.T) {
			iconRec := get(t, server, src, nil)

			assert.Eq(t, http.StatusOK, iconRec.Code)
			assert.Contains(t, iconRec.Header().Get("Content-Type"), "image/png")
		})
	}
}
