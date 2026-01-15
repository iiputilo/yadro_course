package core

import "context"

type Searcher interface {
	Search(ctx context.Context, params SearchParams) (SearchResult, error)
	ISearch(ctx context.Context, params SearchParams) (SearchResult, error)
	RebuildIndex(ctx context.Context) error
}

type Storage interface {
	SearchComics(ctx context.Context, words []string, limit int) ([]Comic, int, error)
	LoadIndexData(ctx context.Context) (map[int][]string, error)
	GetComicsByIDs(ctx context.Context, ids []int) ([]Comic, error)
}

type Words interface {
	Norm(ctx context.Context, phrase string) ([]string, error)
}
