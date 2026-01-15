package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"yadro.com/course/search/adapters/indexer"

	searchpb "yadro.com/course/proto/search"
	searchdb "yadro.com/course/search/adapters/db"
	searchgrpc "yadro.com/course/search/adapters/grpc"
	searchwords "yadro.com/course/search/adapters/words"
	"yadro.com/course/search/config"
	"yadro.com/course/search/core"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	cfg := config.MustLoad(configPath)
	log := mustMakeLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	log.Info("starting search server")
	log.Debug("debug messages are enabled")

	// Graceful shutdown using Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// DB adapter
	store, err := searchdb.New(log, cfg.DBAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to db: %w", err)
	}

	// Words adapter
	wordsClient, err := searchwords.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed to create words client: %w", err)
	}

	// Core service
	svc, err := core.NewService(log, store, wordsClient)
	if err != nil {
		return fmt.Errorf("failed to create search service: %w", err)
	}

	// nats service
	nc, err := nats.Connect(cfg.BrokerAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to nats: %w", err)
	}
	defer func() {
		if err := nc.Drain(); err != nil {
			log.Error("failed to drain nats connection", "error", err)
		}
	}()

	// Ticker indexer service
	tickerIdx := indexer.New(log, svc, cfg.IndexTTL)
	tickerIdx.Start(ctx)
	defer tickerIdx.Stop()

	// Event indexer service
	eventIndexer := indexer.NewEventIndexer(log, svc, nc)
	if err := eventIndexer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start event indexer: %w", err)
	}

	// gRPC server
	lis, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s := grpc.NewServer()
	searchpb.RegisterSearchServer(s, searchgrpc.NewServer(svc))
	reflection.Register(s)

	go func() {
		<-ctx.Done()
		log.Debug("shutting down search server")
		s.GracefulStop()
	}()

	if err := s.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func mustMakeLogger(levelStr string) *slog.Logger {
	var level slog.Level
	switch levelStr {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "ERROR":
		level = slog.LevelError
	default:
		panic("unknown log level: " + levelStr)
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
