package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	searchpb "yadro.com/course/proto/search"
	"yadro.com/course/search/core"
)

func NewServer(s core.Searcher) *Server {
	return &Server{service: s}
}

type Server struct {
	searchpb.UnimplementedSearchServer
	service core.Searcher
}

func (s *Server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *Server) Search(ctx context.Context, req *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	params := core.SearchParams{
		Phrase: req.GetPhrase(),
		Limit:  int(req.GetLimit()),
	}

	res, err := s.service.Search(ctx, params)
	if err != nil {
		switch err {
		case core.ErrBadArguments:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case core.ErrRequestTooLarge:
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	resp := &searchpb.SearchReply{
		Total: uint32(res.Total),
	}

	resp.Comics = make([]*searchpb.Comic, 0, len(res.Comics))
	for _, c := range res.Comics {
		resp.Comics = append(resp.Comics, &searchpb.Comic{
			Id:  int32(c.ID),
			Url: c.URL,
		})
	}

	return resp, nil
}

func (s *Server) ISearch(ctx context.Context, req *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	params := core.SearchParams{
		Phrase: req.GetPhrase(),
		Limit:  int(req.GetLimit()),
	}

	res, err := s.service.ISearch(ctx, params)
	if err != nil {
		switch err {
		case core.ErrBadArguments:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case core.ErrRequestTooLarge:
			return nil, status.Error(codes.ResourceExhausted, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	resp := &searchpb.SearchReply{
		Total: uint32(res.Total),
	}

	resp.Comics = make([]*searchpb.Comic, 0, len(res.Comics))
	for _, c := range res.Comics {
		resp.Comics = append(resp.Comics, &searchpb.Comic{
			Id:  int32(c.ID),
			Url: c.URL,
		})
	}

	return resp, nil
}
