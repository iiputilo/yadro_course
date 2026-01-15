package core

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

const (
	defaultLimit = 10
	maxPhraseLen = 4096
)

type Service struct {
	log   *slog.Logger
	store Storage
	words Words

	mu    sync.RWMutex
	index Index
}

func NewService(log *slog.Logger, store Storage, words Words) (*Service, error) {
	if log == nil || store == nil || words == nil {
		return nil, ErrNilDependency
	}
	return &Service{
		log:   log,
		store: store,
		words: words,
		index: make(Index),
	}, nil
}

func (s *Service) Search(ctx context.Context, params SearchParams) (SearchResult, error) {
	limit, err := normalizeLimit(params.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	phrase, err := sanitizePhrase(params.Phrase)
	if err != nil {
		return SearchResult{}, err
	}

	ws, err := s.words.Norm(ctx, phrase)
	if err != nil {
		return SearchResult{}, err
	}
	if len(ws) == 0 {
		return SearchResult{}, nil
	}

	comics, total, err := s.store.SearchComics(ctx, ws, limit)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Comics: comics, Total: total}, nil
}

func (s *Service) ISearch(ctx context.Context, params SearchParams) (SearchResult, error) {
	limit, err := normalizeLimit(params.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	phrase, err := sanitizePhrase(params.Phrase)
	if err != nil {
		return SearchResult{}, err
	}

	ws, err := s.words.Norm(ctx, phrase)
	if err != nil {
		return SearchResult{}, err
	}
	ws = deduplicateWords(ws)
	if len(ws) == 0 {
		return SearchResult{}, nil
	}

	ranked := s.rankIDs(ws)
	total := len(ranked)
	if total == 0 {
		return SearchResult{}, nil
	}

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	comics, err := s.store.GetComicsByIDs(ctx, ranked)
	if err != nil {
		return SearchResult{}, err
	}
	ordered := orderComics(comics, ranked)

	return SearchResult{
		Comics: ordered,
		Total:  total,
	}, nil
}

func (s *Service) RebuildIndex(ctx context.Context) error {
	data, err := s.store.LoadIndexData(ctx)
	if err != nil {
		return err
	}

	newIndex := make(Index, len(data))
	for id, words := range data {
		if len(words) == 0 {
			continue
		}
		seen := make(map[string]struct{}, len(words))
		for _, word := range words {
			if word == "" {
				continue
			}
			if _, ok := seen[word]; ok {
				continue
			}
			seen[word] = struct{}{}
			newIndex[word] = append(newIndex[word], id)
		}
	}

	for word, ids := range newIndex {
		sort.Ints(ids)
		newIndex[word] = ids
	}

	s.mu.Lock()
	s.index = newIndex
	s.mu.Unlock()

	s.log.Info("index rebuilt", "entries", len(newIndex))
	return nil
}

func normalizeLimit(limit int) (int, error) {
	if limit < 0 {
		return 0, ErrBadArguments
	}
	if limit == 0 {
		return defaultLimit, nil
	}
	return limit, nil
}

func sanitizePhrase(phrase string) (string, error) {
	p := strings.TrimSpace(phrase)
	if p == "" {
		return "", ErrBadArguments
	}
	if len(p) > maxPhraseLen {
		return "", ErrRequestTooLarge
	}
	return p, nil
}

func deduplicateWords(words []string) []string {
	if len(words) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(words))
	result := make([]string, 0, len(words))
	for _, w := range words {
		if w == "" {
			continue
		}
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		result = append(result, w)
	}
	return result
}

func (s *Service) rankIDs(words []string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.index) == 0 || len(words) == 0 {
		return nil
	}

	counts := make(map[int]int)
	for _, word := range words {
		ids, ok := s.index[word]
		if !ok {
			continue
		}
		for _, id := range ids {
			counts[id]++
		}
	}

	ranked := make([]struct {
		id      int
		matches int
	}, 0, len(counts))

	for id, matches := range counts {
		ranked = append(ranked, struct {
			id      int
			matches int
		}{id: id, matches: matches})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].matches == ranked[j].matches {
			return ranked[i].id < ranked[j].id
		}
		return ranked[i].matches > ranked[j].matches
	})

	ids := make([]int, len(ranked))
	for i, r := range ranked {
		ids[i] = r.id
	}
	return ids
}

func orderComics(source []Comic, order []int) []Comic {
	if len(source) == 0 || len(order) == 0 {
		return nil
	}

	m := make(map[int]Comic, len(source))
	for _, comic := range source {
		m[comic.ID] = comic
	}

	result := make([]Comic, 0, len(order))
	for _, id := range order {
		if c, ok := m[id]; ok {
			result = append(result, c)
		}
	}
	return result
}
