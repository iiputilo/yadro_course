package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"yadro.com/course/api/core"
)

type testNormalizer struct{}

func (testNormalizer) Norm(ctx context.Context, s string) ([]string, error) {
	return []string{s}, nil
}

type testPinger struct {
	called bool
}

func (p *testPinger) Ping(ctx context.Context) error {
	p.called = true
	return nil
}

type testUpdater struct{}

func (testUpdater) Update(ctx context.Context) error                       { return nil }
func (testUpdater) Stats(ctx context.Context) (core.UpdateStats, error)   { return core.UpdateStats{}, nil }
func (testUpdater) Status(ctx context.Context) (core.UpdateStatus, error) { return core.StatusUpdateIdle, nil }
func (testUpdater) Drop(ctx context.Context) error                        { return nil }

type testSearcher struct{}

func (testSearcher) Search(ctx context.Context, phrase string, limit int) (core.SearchResult, error) {
	return core.SearchResult{Total: limit}, nil
}

func (testSearcher) ISearch(ctx context.Context, phrase string, limit int) (core.SearchResult, error) {
	return core.SearchResult{Total: limit}, nil
}

func TestInterfacesImplemented(t *testing.T) {
	var _ core.Normalizer = testNormalizer{}
	var _ core.Pinger = (*testPinger)(nil)
	var _ core.Updater = testUpdater{}
	var _ core.Searcher = testSearcher{}
}

func TestTestPingerCalled(t *testing.T) {
	ctx := context.Background()
	p := &testPinger{}
	require.False(t, p.called)
	err := p.Ping(ctx)
	require.NoError(t, err)
	require.True(t, p.called)
}

func TestTestSearcher(t *testing.T) {
	ctx := context.Background()
	s := testSearcher{}
	res, err := s.Search(ctx, "linux", 5)
	require.NoError(t, err)
	require.Equal(t, 5, res.Total)

	ires, err := s.ISearch(ctx, "linux", 3)
	require.NoError(t, err)
	require.Equal(t, 3, ires.Total)
}
