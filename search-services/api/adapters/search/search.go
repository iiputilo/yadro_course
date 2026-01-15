package search

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"yadro.com/course/api/core"
	searchpb "yadro.com/course/proto/search"
)

type Client struct {
	log    *slog.Logger
	client searchpb.SearchClient
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		client: searchpb.NewSearchClient(conn),
		log:    log,
	}, nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, nil)
	return err
}

func (c Client) Search(ctx context.Context, phrase string, limit int) (core.SearchResult, error) {
	if phrase == "" {
		return core.SearchResult{}, core.ErrBadPhrase
	}
	if limit <= 0 {
		return core.SearchResult{}, core.ErrBadLimit
	}

	resp, err := c.client.Search(ctx, &searchpb.SearchRequest{
		Phrase: phrase,
		Limit:  uint32(limit),
	})
	if err != nil {
		c.log.Warn("search rpc failed", "error", err)
		return core.SearchResult{}, err
	}

	out := core.SearchResult{
		Comics: make([]core.Comics, 0, len(resp.GetComics())),
		Total:  int(resp.GetTotal()),
	}
	for _, cpb := range resp.GetComics() {
		out.Comics = append(out.Comics, core.Comics{
			ID:  int(cpb.GetId()),
			URL: cpb.GetUrl(),
		})
	}
	return out, nil
}

func (c Client) ISearch(ctx context.Context, phrase string, limit int) (core.SearchResult, error) {
	if phrase == "" {
		return core.SearchResult{}, core.ErrBadPhrase
	}
	if limit <= 0 {
		return core.SearchResult{}, core.ErrBadLimit
	}

	resp, err := c.client.ISearch(ctx, &searchpb.SearchRequest{
		Phrase: phrase,
		Limit:  uint32(limit),
	})
	if err != nil {
		c.log.Warn("search rpc failed", "error", err)
		return core.SearchResult{}, err
	}

	out := core.SearchResult{
		Comics: make([]core.Comics, 0, len(resp.GetComics())),
		Total:  int(resp.GetTotal()),
	}
	for _, cpb := range resp.GetComics() {
		out.Comics = append(out.Comics, core.Comics{
			ID:  int(cpb.GetId()),
			URL: cpb.GetUrl(),
		})
	}
	return out, nil
}
