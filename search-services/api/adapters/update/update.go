package update

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"yadro.com/course/api/core"
	updatepb "yadro.com/course/proto/update"
)

type Client struct {
	log    *slog.Logger
	client updatepb.UpdateClient
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial update service: %w", err)
	}
	return &Client{
		client: updatepb.NewUpdateClient(conn),
		log:    log,
	}, nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &emptypb.Empty{})
	return mapErr(err)
}

func (c Client) Status(ctx context.Context) (core.UpdateStatus, error) {
	resp, err := c.client.Status(ctx, &emptypb.Empty{})
	if err != nil {
		return core.StatusUpdateUnknown, mapErr(err)
	}
	switch resp.GetStatus() {
	case updatepb.Status_STATUS_IDLE:
		return core.StatusUpdateIdle, nil
	case updatepb.Status_STATUS_RUNNING:
		return core.StatusUpdateRunning, nil
	default:
		return core.StatusUpdateUnknown, nil
	}
}

func (c Client) Stats(ctx context.Context) (core.UpdateStats, error) {
	resp, err := c.client.Stats(ctx, &emptypb.Empty{})
	if err != nil {
		return core.UpdateStats{}, mapErr(err)
	}
	return core.UpdateStats{
		WordsTotal:    int(resp.GetWordsTotal()),
		WordsUnique:   int(resp.GetWordsUnique()),
		ComicsFetched: int(resp.GetComicsFetched()),
		ComicsTotal:   int(resp.GetComicsTotal()),
	}, nil
}

func (c Client) Update(ctx context.Context) error {
	_, err := c.client.Update(ctx, &emptypb.Empty{})
	return mapErr(err)
}

func (c Client) Drop(ctx context.Context) error {
	_, err := c.client.Drop(ctx, &emptypb.Empty{})
	return mapErr(err)
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	s, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch s.Code() {
	case codes.InvalidArgument:
		return core.ErrBadArguments
	case codes.AlreadyExists:
		return core.ErrAlreadyExists
	default:
		return err
	}
}
