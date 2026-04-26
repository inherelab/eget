package app

import (
	"fmt"
	"strings"

	"github.com/inherelab/eget/internal/client"
)

type SearchRepo = client.SearchRepo
type SearchResult = client.SearchResult

type SearchOptions struct {
	Keyword string
	Extras  []string
	Limit   int
	Sort    string
	Order   string
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
