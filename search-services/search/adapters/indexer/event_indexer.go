package indexer

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"yadro.com/course/search/core"
)

const (
	defaultDebounce = 10 * time.Second
)

type EventIndexer struct {
	log      *slog.Logger
	svc      core.Searcher
	nc       *nats.Conn
	debounce time.Duration

	cancel context.CancelFunc
	sub    *nats.Subscription

	pending int32
}

func NewEventIndexer(log *slog.Logger, svc core.Searcher, nc *nats.Conn) *EventIndexer {
	return &EventIndexer{
		log:      log,
		svc:      svc,
		nc:       nc,
		debounce: defaultDebounce,
	}
}

func (i *EventIndexer) Start(ctx context.Context) error {
	if i.svc == nil || i.nc == nil {
		return core.ErrNilDependency
	}

	ctx, cancel := context.WithCancel(ctx)
	i.cancel = cancel

	ch := make(chan *nats.Msg, 16)

	sub, err := i.nc.ChanSubscribe("xkcd.db.updated", ch)
	if err != nil {
		return err
	}
	i.sub = sub

	go func() {
		defer func() {
			if err := sub.Unsubscribe(); err != nil {
				i.log.Error("failed to unsubscribe from nats", "error", err)
			}
			close(ch)
			i.log.Info("event indexer stopped")
		}()

		ticker := time.NewTicker(i.debounce)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				if atomic.LoadInt32(&i.pending) == 1 {
					i.log.Info("rebuilding index after db update events")
					if err := i.svc.RebuildIndex(ctx); err != nil {
						i.log.Error("index rebuild failed", "error", err)
					}
					atomic.StoreInt32(&i.pending, 0)
				}

			case _, ok := <-ch:
				if !ok {
					return
				}
				atomic.StoreInt32(&i.pending, 1)
			}
		}
	}()

	return nil
}

func (i *EventIndexer) Stop() {
	if i.cancel != nil {
		i.cancel()
	}
}
