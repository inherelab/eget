package app

import (
	"fmt"
	"strings"
	"time"
)

type SearchRepo struct {
	FullName        string    `json:"full_name,omitempty"`
	Description     string    `json:"description,omitempty"`
	HTMLURL         string    `json:"html_url,omitempty"`
	Homepage        string    `json:"homepage,omitempty"`
	Language        string    `json:"language,omitempty"`
	StargazersCount int       `json:"stargazers_count,omitempty"`
	ForksCount      int       `json:"forks_count,omitempty"`
	OpenIssuesCount int       `json:"open_issues_count,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
	Archived        bool      `json:"archived,omitempty"`
	Private         bool      `json:"private,omitempty"`
}

type SearchOptions struct {
	Keyword string
	Extras  []string
	Limit   int
	Sort    string
	Order   string
}

type SearchResult struct {
	Query      string       `json:"query"`
	TotalCount int          `json:"total_count"`
	Items      []SearchRepo `json:"items,omitempty"`
}

type SearchClient interface {
	SearchRepositories(query string, limit int, sort, order string) (SearchResult, error)
}

type SearchService struct {
	Client SearchClient
}

func (s SearchService) Search(opts SearchOptions) (SearchResult, error) {
	if s.Client == nil {
		return SearchResult{}, fmt.Errorf("search client is required")
	}

	query := strings.TrimSpace(opts.Keyword)
	if query == "" {
		return SearchResult{}, fmt.Errorf("search keyword is required")
	}
	if len(opts.Extras) > 0 {
		parts := []string{query}
		for _, extra := range opts.Extras {
			extra = strings.TrimSpace(extra)
			if extra != "" {
				parts = append(parts, extra)
			}
		}
		query = strings.Join(parts, " ")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	if opts.Sort != "" && opts.Sort != "stars" && opts.Sort != "updated" {
		return SearchResult{}, fmt.Errorf("invalid search sort %q", opts.Sort)
	}
	if opts.Order != "" && opts.Order != "desc" && opts.Order != "asc" {
		return SearchResult{}, fmt.Errorf("invalid search order %q", opts.Order)
	}

	result, err := s.Client.SearchRepositories(query, limit, opts.Sort, opts.Order)
	if err != nil {
		return SearchResult{}, err
	}
	result.Query = query
	return result, nil
}
