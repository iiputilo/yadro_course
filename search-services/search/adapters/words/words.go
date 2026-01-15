package words

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	wordspb "yadro.com/course/proto/words"
	"yadro.com/course/search/core"
)

type Client struct {
	log    *slog.Logger
	client wordspb.WordsClient
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		log:    log,
		client: wordspb.NewWordsClient(conn),
	}, nil
}

func (c *Client) Norm(ctx context.Context, phrase string) ([]string, error) {
	if phrase == "" {
		return nil, core.ErrBadArguments
	}

	resp, err := c.client.Norm(ctx, &wordspb.WordsRequest{Phrase: phrase})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.InvalidArgument:
				return nil, core.ErrBadArguments
			case codes.ResourceExhausted:
				return nil, core.ErrRequestTooLarge
			default:
				return nil, err
			}
		}
		return nil, err
	}
	return resp.GetWords(), nil
}
