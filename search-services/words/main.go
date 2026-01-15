package main

import (
	"context"
	"flag"
	"log"
	"net"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	wordspb "yadro.com/course/proto/words"
	normalize "yadro.com/course/words/words"
)

const maxPhraseLen = 4096

type server struct {
	wordspb.UnimplementedWordsServer
}

type Config struct {
	GRPCPort string `yaml:"grpc_port" env:"WORDS_GRPC_PORT" env-default:"8080"`
}

func (s *server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *server) Norm(_ context.Context, in *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if len(in.Phrase) > maxPhraseLen {
		return nil, status.Error(codes.ResourceExhausted, "request exceeds 4 KiB")
	}
	words := normalize.Normalize(in.Phrase)
	return &wordspb.WordsReply{Words: words}, nil
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "", "path to config file")
	flag.Parse()

	var cfg Config
	if cfgPath != "" {
		if err := cleanenv.ReadConfig(cfgPath, &cfg); err != nil {
			log.Fatalf("failed to read config file: %v", err)
		}
	}
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("failed to read env: %v", err)
	}

	addr := cfg.GRPCPort
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	wordspb.RegisterWordsServer(s, &server{})
	reflection.Register(s)

	log.Printf("words service listening on %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
