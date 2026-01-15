package core

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

const (
	maxWordsPhraseLen = 4096
	placeholderURL    = "missing"
)

type Events interface {
	PublishDBUpdated()
}

type Service struct {
	log         *slog.Logger
	db          DB
	xkcd        XKCD
	words       Words
	events      Events
	concurrency int

	mu     sync.Mutex
	status atomic.Value
}

func NewService(
	log *slog.Logger, db DB, xkcd XKCD, words Words, events Events, concurrency int,
) (*Service, error) {
	if concurrency < 1 {
		return nil, errors.New("wrong concurrency specified")
	}
	s := &Service{
		log:         log,
		db:          db,
		xkcd:        xkcd,
		words:       words,
		events:      events,
		concurrency: concurrency,
	}
	s.status.Store(StatusIdle)
	return s, nil
}

func (s *Service) Update(ctx context.Context) (err error) {
	if !s.mu.TryLock() {
		return ErrAlreadyExists
	}
	s.status.Store(StatusRunning)
	defer func() {
		s.status.Store(StatusIdle)
		s.mu.Unlock()
		if err == nil && s.events != nil {
			s.log.Info("publishing db updated event")
			s.events.PublishDBUpdated()
		}
	}()

	last, err := s.xkcd.LastID(ctx)
	if err != nil {
		return err
	}
	existing, err := s.db.IDs(ctx)
	if err != nil {
		return err
	}
	exists := make(map[int]struct{}, len(existing))
	for _, id := range existing {
		exists[id] = struct{}{}
	}

	jobs := make(chan int, s.concurrency*2)
	var wg sync.WaitGroup
	worker := func() {
		for id := range jobs {
			select {
			case <-ctx.Done():
				return
			default:
			}

			info, err := s.xkcd.Get(ctx, id)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					if dbErr := s.db.Add(ctx, Comics{ID: id, URL: placeholderURL, Words: []string{}}); dbErr != nil {
						s.log.Warn("db add placeholder failed", "id", id, "error", dbErr)
					}
					continue
				}
				s.log.Warn("xkcd get failed", "id", id, "error", err)
				continue
			}

			phrase := info.Title + " " + info.Description
			phrase = truncateUTF8ToBytes(phrase, maxWordsPhraseLen)

			ws, err := s.words.Norm(ctx, phrase)
			if err != nil {
				s.log.Warn("words normalize failed", "id", id, "error", err)
				ws = []string{}
			}
			if err := s.db.Add(ctx, Comics{ID: info.ID, URL: info.URL, Words: ws}); err != nil {
				s.log.Warn("db add failed", "id", id, "error", err)
				continue
			}
		}
	}

	for i := 0; i < s.concurrency; i++ {
		wg.Go(worker)
	}
	for id := 1; id <= last; id++ {
		if _, ok := exists[id]; ok {
			continue
		}
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- id:
		}
	}
	close(jobs)
	wg.Wait()
	return nil
}

func (s *Service) Stats(ctx context.Context) (ServiceStats, error) {
	dbst, err := s.db.Stats(ctx)
	if err != nil {
		return ServiceStats{}, err
	}
	total, err := s.xkcd.LastID(ctx)
	if err != nil {
		return ServiceStats{}, err
	}
	return ServiceStats{
		DBStats:     dbst,
		ComicsTotal: total,
	}, nil
}

func (s *Service) Status(context.Context) ServiceStatus {
	v := s.status.Load()
	if v == nil {
		return StatusIdle
	}
	return v.(ServiceStatus)
}

func (s *Service) Drop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Drop(ctx)
}

func truncateUTF8ToBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	i := 0
	for i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		if i+size > limit {
			break
		}
		i += size
	}
	return s[:i]
}
