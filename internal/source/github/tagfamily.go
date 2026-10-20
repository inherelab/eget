package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// familyCore matches the leading dotted numeric core of a tag suffix; whatever
// follows is the text residue (e.g. "-rc1" of "0.37.0-rc1").
var familyCore = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*`)

// unorderableResidue marks residues whose ordering sits on ambiguous ground: a
// separator followed by digits reads like dates ("-10-20") or numeric build
// markers, so such a family is never ordered here and fails open.
var unorderableResidue = regexp.MustCompile(`^[-_.+][0-9]`)

type familySuffix struct {
	core    []int
	residue string
}

// parseFamilySuffix splits a tag suffix (the part after the literal prefix)
// into its numeric core and text residue. ok=false marks an unorderable suffix.
func parseFamilySuffix(s string) (familySuffix, bool) {
	m := familyCore.FindString(s)
	if unorderableResidue.MatchString(s[len(m):]) {
		return familySuffix{}, false
	}
	if m == "" {
		// A tag that names the prefix exactly has an empty suffix; a residue
		// starting with a letter or separator (not sep+digit) orders lexically.
		return familySuffix{residue: s}, true
	}
	var core []int
	for _, tok := range strings.Split(m, ".") {
		n, err := strconv.Atoi(tok)
		if err != nil {
			// numeric token out of range: family stays unordered
			return familySuffix{}, false
		}
		core = append(core, n)
	}
	return familySuffix{core: core, residue: s[len(m):]}, true
}

// compareFamilyTags orders two tags that share one literal prefix by suffix:
// the dotted numeric core is compared component-wise, and identical cores fall
// to the residue where empty (a stable release) sorts above a non-empty one
// and otherwise the lexicographic order decides ("-rc2" above "-rc1").
//
// ok=false means the pair cannot be ordered conclusively — differing core
// lengths ("0.37" vs "0.37.0"), ties, date-like residues — and callers must
// fail open to the release scan rather than guess.
func compareFamilyTags(a, b string) (int, bool) {
	fa, oka := parseFamilySuffix(a)
	fb, okb := parseFamilySuffix(b)
	if !oka || !okb {
		return 0, false
	}
	for i := 0; i < len(fa.core) && i < len(fb.core); i++ {
		if fa.core[i] != fb.core[i] {
			if fa.core[i] < fb.core[i] {
				return -1, true
			}
			return 1, true
		}
	}
	if len(fa.core) != len(fb.core) {
		// numerically equal cores of different length have no defined order
		// within the family
		return 0, false
	}
	switch {
	case fa.residue == "" && fb.residue == "":
		return 0, false
	case fa.residue == "":
		return 1, true
	case fb.residue == "":
		return -1, true
	case fa.residue == fb.residue:
		return 0, false
	default:
		if fa.residue < fb.residue {
			return -1, true
		}
		return 1, true
	}
}

// TagFamilyLatest reports the version-maximum tag among the repository's refs
// that share the literal prefix, via the matching-refs endpoint (one small
// response even for monorepos that publish hundreds of releases).
//
// ok=false means the family cannot be ordered conclusively — endpoint failure,
// no matching refs, ambiguous tags or a tie — and the caller must fall back to
// the release scan instead of assuming the family is exhausted.
func TagFamilyLatest(repo, prefix string, getter HTTPGetter) (string, bool) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || getter == nil {
		return "", false
	}
	base := fmt.Sprintf("https://api.github.com/repos/%s/git/matching-refs/tags/%s", repo, strings.ReplaceAll(prefix, " ", ""))

	var best, bestSuffix string
	seen := make(map[string]bool)
	// the response is refname-sorted (v0.10.0 before v0.2.0), so ordering is
	// redone client-side; pages are only followed while they keep adding refs
	for page := 1; ; page++ {
		rawURL := base
		if page > 1 {
			rawURL = fmt.Sprintf("%s?page=%d", base, page)
		}
		refs, err := fetchFamilyRefs(rawURL, getter)
		if err != nil {
			return "", false
		}
		fresh := 0
		for _, ref := range refs {
			tag := strings.TrimPrefix(ref, "refs/tags/")
			if !strings.HasPrefix(tag, prefix) {
				continue
			}
			if seen[tag] {
				continue
			}
			seen[tag] = true
			fresh++
			suffix := tag[len(prefix):]
			if best == "" {
				best, bestSuffix = tag, suffix
				continue
			}
			cmp, orderable := compareFamilyTags(suffix, bestSuffix)
			if !orderable {
				return "", false
			}
			if cmp > 0 {
				best, bestSuffix = tag, suffix
			}
		}
		if len(refs) == 0 || fresh == 0 {
			break
		}
	}
	return best, best != ""
}

func fetchFamilyRefs(rawURL string, getter HTTPGetter) ([]string, error) {
	verbosef("github tag family request: %s", rawURL)
	resp, err := getter.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("tag family request: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	verbosef("github tag family response: %s", truncateBody(body))
	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(body, &refs); err != nil {
		return nil, err
	}
	items := make([]string, 0, len(refs))
	for _, item := range refs {
		items = append(items, item.Ref)
	}
	return items, nil
}
