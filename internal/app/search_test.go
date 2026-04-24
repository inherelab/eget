package app

import (
	"errors"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
)

type fakeSearchClient struct {
	result    SearchResult
	err       error
	callCount int
	lastQuery string
	lastLimit int
	lastSort  string
	lastOrder string
}

func (f *fakeSearchClient) SearchRepositories(query string, limit int, sort, order string) (SearchResult, error) {
	f.callCount++
	f.lastQuery = query
	f.lastLimit = limit
	f.lastSort = sort
	f.lastOrder = order
	if f.err != nil {
		return SearchResult{}, f.err
	}
	return f.result, nil
}

func TestSearchServiceSearch(t *testing.T) {
	t.Run("默认 limit 并组合 keyword 与 extras", func(t *testing.T) {
		client := &fakeSearchClient{}
		svc := SearchService{Client: client}

		result, err := svc.Search(SearchOptions{
			Keyword: "ripgrep",
			Extras:  []string{"language:go", "topic:cli"},
			Sort:    "stars",
			Order:   "desc",
		})

		assert.Nil(t, err)
		assert.Eq(t, 1, client.callCount)
		assert.Eq(t, "ripgrep language:go topic:cli", client.lastQuery)
		assert.Eq(t, 10, client.lastLimit)
		assert.Eq(t, "stars", client.lastSort)
		assert.Eq(t, "desc", client.lastOrder)
		assert.Eq(t, "ripgrep language:go topic:cli", result.Query)
	})

	t.Run("空 keyword 报错", func(t *testing.T) {
		client := &fakeSearchClient{}
		svc := SearchService{Client: client}

		_, err := svc.Search(SearchOptions{
			Keyword: "",
			Extras:  []string{"language:go"},
		})

		assert.NotNil(t, err)
		assert.Eq(t, 0, client.callCount)
	})

	t.Run("非法 sort 或 order 报错", func(t *testing.T) {
		client := &fakeSearchClient{}
		svc := SearchService{Client: client}

		_, err := svc.Search(SearchOptions{Keyword: "ripgrep", Sort: "forks"})
		assert.NotNil(t, err)

		_, err = svc.Search(SearchOptions{Keyword: "ripgrep", Order: "down"})
		assert.NotNil(t, err)
		assert.Eq(t, 0, client.callCount)
	})

	t.Run("client 返回结果后写回 Query", func(t *testing.T) {
		client := &fakeSearchClient{
			result: SearchResult{
				Items:      []SearchRepo{{FullName: "BurntSushi/ripgrep"}},
				TotalCount: 1,
			},
		}
		svc := SearchService{Client: client}

		result, err := svc.Search(SearchOptions{
			Keyword: "ripgrep",
			Extras:  []string{"language:rust"},
			Limit:   5,
			Sort:    "updated",
			Order:   "asc",
		})

		assert.Nil(t, err)
		assert.Eq(t, "ripgrep language:rust", result.Query)
		assert.Eq(t, 1, result.TotalCount)
		assert.Eq(t, "BurntSushi/ripgrep", result.Items[0].FullName)
	})

	t.Run("client 错误透传", func(t *testing.T) {
		wantErr := errors.New("boom")
		client := &fakeSearchClient{err: wantErr}
		svc := SearchService{Client: client}

		_, err := svc.Search(SearchOptions{Keyword: "ripgrep"})

		assert.NotNil(t, err)
		assert.Eq(t, wantErr.Error(), err.Error())
	})
}
