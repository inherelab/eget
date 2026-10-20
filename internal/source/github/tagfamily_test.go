package github

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestCompareFamilyTagsOrdersNumericCores(t *testing.T) {
	// (a, b, want, wantOK): a > b is +1, a < b is -1; ok=false fails open.
	cases := []struct {
		a, b   string
		want   int
		wantOK bool
	}{
		// numeric comparison beats lexicographic order
		{"0.10.0", "0.2.0", 1, true},
		{"0.2.15", "0.2.9", 1, true},
		{"1.0.0", "0.99.0", 1, true},
		// stable (empty residue) sorts above its suffixed sibling
		{"0.37.0", "0.37.0-rc1", 1, true},
		{"1.0.0-beta", "1.0.0", -1, true},
		// residues of the same shape order lexicographically
		{"1.0.0-rc2", "1.0.0-rc1", 1, true},
		// differing core lengths are undefined inside the family: fail open
		{"0.37", "0.37.0", 0, false},
		// ties cannot name a single maximum: fail open
		{"0.5.0", "0.5.0", 0, false},
		{"1.0.0-rc1", "1.0.0-rc1", 0, false},
		// date-like numeric residues are ambiguous: fail open
		{"2026-10-20", "2026-10-21", 0, false},
		{"1.0.0+20260102", "1.0.0+20250102", 0, false},
	}
	for _, c := range cases {
		got, ok := compareFamilyTags(c.a, c.b)
		if ok != c.wantOK {
			t.Fatalf("compareFamilyTags(%q,%q): ok=%v, want %v", c.a, c.b, ok, c.wantOK)
		}
		if ok && got != c.want {
			t.Fatalf("compareFamilyTags(%q,%q)=%d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestTagFamilyLatestPicksMaxFromSingleResponse(t *testing.T) {
	var refs []string
	// refname-sorted like the real endpoint: the max tag is NOT first
	for _, tag := range []string{
		"cua-driver-rs-v0.1.3", "cua-driver-rs-v0.10.0", "cua-driver-rs-v0.2.0",
		"cua-driver-rs-v0.35.4", "cua-driver-rs-v0.36.0", "cua-driver-rs-v0.37.0",
		"cua-driver-rs-v0.9.1",
	} {
		refs = append(refs, fmt.Sprintf(`{"ref":"refs/tags/%s"}`, tag))
	}
	var firstURL string
	getter := HTTPGetterFunc(func(url string) (*http.Response, error) {
		if firstURL == "" {
			firstURL = url
		}
		if strings.Contains(url, "page=2") {
			// beyond-end page of the real endpoint: empty
			return jsonResponse(http.StatusOK, "[]"), nil
		}
		return jsonResponse(http.StatusOK, "["+joinComma(refs)+"]"), nil
	})

	tag, ok := TagFamilyLatest("trycua/cua", "cua-driver-rs-v", getter)
	if !ok {
		t.Fatal("expected an orderable family")
	}
	if tag != "cua-driver-rs-v0.37.0" {
		t.Fatalf("tag = %q", tag)
	}
	if firstURL != "https://api.github.com/repos/trycua/cua/git/matching-refs/tags/cua-driver-rs-v?per_page=100" {
		t.Fatalf("first url = %s", firstURL)
	}
}

func TestTagFamilyLatestFollowsPaginationWhileFresh(t *testing.T) {
	// exactly one batch-full page must trigger a follow-up; an empty second
	// page ends the loop instead of spinning
	getter := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/acme/app/git/matching-refs/tags/app-v?per_page=100":        jsonResponse(http.StatusOK, `[{"ref":"refs/tags/app-v1.1.0"},{"ref":"refs/tags/app-v1.0.0"}]`),
			"https://api.github.com/repos/acme/app/git/matching-refs/tags/app-v?per_page=100&page=2": jsonResponse(http.StatusOK, `[]`),
		},
	}

	tag, ok := TagFamilyLatest("acme/app", "app-v", getter)

	if !ok || tag != "app-v1.1.0" {
		t.Fatalf("tag=%q ok=%v", tag, ok)
	}
}

func TestTagFamilyLatestFailOpen(t *testing.T) {
	paginated := &fakeGetter{
		responses: map[string]*http.Response{
			"https://api.github.com/repos/acme/app/git/matching-refs/tags/app-v?per_page=100":        jsonResponse(http.StatusOK, `[{"ref":"refs/tags/app-v0.37"},{"ref":"refs/tags/app-v0.37.0"}]`),
			"https://api.github.com/repos/acme/app/git/matching-refs/tags/app-v?per_page=100&page=2": jsonResponse(http.StatusOK, `[]`),
		},
	}

	cases := []struct {
		name   string
		repo   string
		prefix string
		getter HTTPGetter
	}{
		{"nil getter", "acme/app", "app-v", nil},
		{"empty prefix", "acme/app", "   ", HTTPGetterFunc(func(url string) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `[{"ref":"refs/tags/app-v1.0.0"}]`), nil
		})},
		{"endpoint error", "acme/app", "app-v", HTTPGetterFunc(func(url string) (*http.Response, error) {
			return nil, fmt.Errorf("boom")
		})},
		{"non-200", "acme/app", "app-v", HTTPGetterFunc(func(url string) (*http.Response, error) {
			return jsonResponse(http.StatusForbidden, `{"message":"API rate limit exceeded"}`), nil
		})},
		{"no matching refs", "acme/app", "app-v", HTTPGetterFunc(func(url string) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `[{"ref":"refs/tags/nightly"}]`), nil
		})},
		{"ambiguous family", "acme/app", "app-v", paginated},
	}
	for _, c := range cases {
		if tag, ok := TagFamilyLatest(c.repo, c.prefix, c.getter); ok || tag != "" {
			t.Fatalf("%s: tag=%q ok=%v, want fail-open", c.name, tag, ok)
		}
	}
}

func joinComma(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ","
		}
		out += item
	}
	return out
}
