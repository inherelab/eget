package sdk

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func (s Service) SearchIndex(name string, opts SearchOptions) ([]SearchResult, error) {
	if err := validateSearchSort(opts.Sort); err != nil {
		return nil, err
	}
	cfg, err := s.resolveConfig(name)
	if err != nil {
		return nil, err
	}
	index, err := s.IndexCache.LoadForSource(cfg.Name, cfg.IndexURL)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0)
	for _, item := range index.Items {
		if !opts.IncludePrerelease && !item.Stable {
			continue
		}
		for _, file := range item.Files {
			result := SearchResult{
				SDK:      index.SDK,
				Version:  item.Version,
				Stable:   item.Stable,
				OS:       file.OS,
				Arch:     file.Arch,
				Ext:      file.Ext,
				Filename: file.Filename,
				URL:      file.URL,
			}
			matched, err := searchResultMatches(result, opts.Keywords)
			if err != nil {
				return nil, err
			}
			if matched {
				results = append(results, result)
			}
		}
	}
	sortSearchResults(results, opts.Sort)
	return limitSearchResults(results, opts.Number), nil
}

func searchResultMatches(result SearchResult, keywords []string) (bool, error) {
	keywords = normalizeSearchKeywords(keywords)
	fields := searchResultFields(result)
	haystack := strings.ToLower(strings.Join(fields, " "))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		matched, err := searchKeywordMatches(fields, haystack, keyword)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

func searchResultFields(result SearchResult) []string {
	return []string{
		result.SDK,
		result.Version,
		fmt.Sprintf("%t", result.Stable),
		stabilityName(result.Stable),
		result.OS,
		result.Arch,
		result.Ext,
		result.Filename,
		result.URL,
	}
}

func searchKeywordMatches(fields []string, haystack, keyword string) (bool, error) {
	exclude := strings.HasPrefix(keyword, "^")
	if exclude {
		keyword = strings.TrimPrefix(keyword, "^")
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return true, nil
	}
	contains, err := searchKeywordContains(fields, haystack, keyword)
	if err != nil {
		return false, err
	}
	if exclude {
		return !contains, nil
	}
	return contains, nil
}

func searchKeywordContains(fields []string, haystack, keyword string) (bool, error) {
	if strings.HasPrefix(keyword, "REG:") {
		re, err := regexp.Compile(strings.TrimPrefix(keyword, "REG:"))
		if err != nil {
			return false, err
		}
		for _, field := range fields {
			if re.MatchString(field) {
				return true, nil
			}
		}
		return false, nil
	}
	return strings.Contains(haystack, strings.ToLower(keyword)), nil
}

func normalizeSearchKeywords(keywords []string) []string {
	normalized := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		normalized = append(normalized, strings.Fields(keyword)...)
	}
	return normalized
}

func validateSearchSort(sortValue string) error {
	switch strings.ToLower(strings.TrimSpace(sortValue)) {
	case "", "asc", "desc":
		return nil
	default:
		return fmt.Errorf("invalid sdk search sort %q", sortValue)
	}
}

func sortSearchResults(results []SearchResult, sortValue string) {
	switch strings.ToLower(strings.TrimSpace(sortValue)) {
	case "asc":
		sort.SliceStable(results, func(i, j int) bool {
			return compareVersion(results[i].Version, results[j].Version) < 0
		})
	case "", "desc":
		sort.SliceStable(results, func(i, j int) bool {
			return compareVersion(results[i].Version, results[j].Version) > 0
		})
	}
}

func limitSearchResults(results []SearchResult, number int) []SearchResult {
	if number <= 0 || len(results) <= number {
		return results
	}
	return results[:number]
}

func stabilityName(stable bool) string {
	if stable {
		return "stable"
	}
	return "prerelease"
}
