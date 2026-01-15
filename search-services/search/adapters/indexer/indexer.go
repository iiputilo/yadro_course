package indexer

import (
	"context"
	"log/slog"
	"time"

	"yadro.com/course/search/core"
)

type Indexer struct {
	log    *slog.Logger
	svc    core.Searcher
	ttl    time.Duration
	cancel context.CancelFunc
}

func New(log *slog.Logger, svc core.Searcher, ttl time.Duration) *Indexer {
	return &Indexer{
		log: log,
		svc: svc,
		ttl: ttl,
	}
}

func (i *Indexer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	i.cancel = cancel

	go func() {
		if err := i.svc.RebuildIndex(ctx); err != nil {
			i.log.Error("initial index rebuild failed", "error", err)
		}
		ticker := time.NewTicker(i.ttl)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				i.log.Info("indexer stopped")
				return
			case <-ticker.C:
				if err := i.svc.RebuildIndex(ctx); err != nil {
					i.log.Error("index rebuild failed", "error", err)
				}
			}
		}
	}()
}

func (i *Indexer) Stop() {
	if i.cancel != nil {
		i.cancel()
	}
}
