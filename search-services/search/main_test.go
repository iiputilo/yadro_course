package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	wordspb "yadro.com/course/proto/words"
	"yadro.com/course/search/config"
)

type mockWordsServer struct {
	wordspb.UnimplementedWordsServer
}

func (s *mockWordsServer) Norm(_ context.Context, req *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	return &wordspb.WordsReply{Words: []string{req.GetPhrase()}}, nil
}

func (s *mockWordsServer) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func startMockWordsServer(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	wordspb.RegisterWordsServer(s, &mockWordsServer{})

	go func() {
		err := s.Serve(lis)
		if err != nil && err != grpc.ErrServerStopped {
			t.Logf("mock words server error: %v", err)
		}
	}()

	t.Cleanup(s.GracefulStop)

	return lis.Addr().String()
}

func startMockNatsServer(t *testing.T) string {
	t.Helper()
	opts := &server.Options{
		Host:   "127.0.0.1",
		Port:   -1,
		NoLog:  true,
		NoSigs: true,
	}

	s, err := server.NewServer(opts)
	require.NoError(t, err)

	go s.Start()

	if !s.ReadyForConnections(4 * time.Second) {
		t.Fatal("nats server did not start")
	}

	t.Cleanup(s.Shutdown)

	return s.ClientURL()
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err, "cannot write config")

	return path
}

func TestMain_Start(t *testing.T) {
	wordsAddr := startMockWordsServer(t)
	natsAddr := startMockNatsServer(t)

	dbAddr := "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable"

	configYAML := fmt.Sprintf(`
search_address: "127.0.0.1:0"
db_address: "%s"
words_address: "%s"
broker_address: "%s"
index_ttl: "1m"
log_level: "DEBUG"
`, dbAddr, wordsAddr, natsAddr)

	configPath := writeTempConfig(t, configYAML)

	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"search", "-config", configPath}

	mainDone := make(chan struct{})
	go func() {
		defer close(mainDone)
		cfg := config.MustLoad(configPath)
		log := mustMakeLogger(cfg.LogLevel)
		_ = run(cfg, log)
	}()

	select {
	case <-mainDone:
	case <-time.After(2 * time.Second):
		t.Fatal("main function did not exit as expected")
	}
}
