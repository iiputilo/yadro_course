package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	wordspb "yadro.com/course/proto/words"
)

func TestNorm_EmptyPhrase(t *testing.T) {
	s := &server{}
	resp, err := s.Norm(context.Background(), &wordspb.WordsRequest{Phrase: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Words) != 0 {
		t.Fatalf("expected 0 words, got %d", len(resp.Words))
	}
}

func TestNorm_OnlySpaces(t *testing.T) {
	s := &server{}
	resp, err := s.Norm(context.Background(), &wordspb.WordsRequest{Phrase: "   \t\n "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Words) != 0 {
		t.Fatalf("expected 0 words, got %d", len(resp.Words))
	}
}

func TestNorm_MultipleWordsWithPunctuation(t *testing.T) {
	s := &server{}
	resp, err := s.Norm(context.Background(), &wordspb.WordsRequest{Phrase: "Hello,   world!  Go-lang."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Words) == 0 {
		t.Fatalf("expected non-empty words")
	}
}

func TestNorm_ErrorCodes(t *testing.T) {
	s := &server{}

	_, err := s.Norm(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}

	long := make([]byte, maxPhraseLen+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err = s.Norm(context.Background(), &wordspb.WordsRequest{Phrase: string(long)})
	if err == nil {
		t.Fatalf("expected error")
	}
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", st.Code())
	}
}

func TestPing_OK(t *testing.T) {
	s := &server{}
	_, err := s.Ping(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestBuildConfig_EnvOnly(t *testing.T) {
	_ = os.Unsetenv("WORDS_GRPC_PORT")
	_ = os.Setenv("WORDS_GRPC_PORT", "9000")
	t.Cleanup(func() { _ = os.Unsetenv("WORDS_GRPC_PORT") })

	cfg, err := buildConfig("")
	if err != nil {
		t.Fatalf("buildConfig failed: %v", err)
	}
	if cfg.GRPCPort != "9000" {
		t.Fatalf("expected port 9000, got %s", cfg.GRPCPort)
	}
}

func TestBuildConfig_FromFileAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	content := []byte("grpc_port: 7000\n")
	if err := os.WriteFile(cfgPath, content, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_ = os.Setenv("WORDS_GRPC_PORT", "8000")
	t.Cleanup(func() { _ = os.Unsetenv("WORDS_GRPC_PORT") })

	cfg, err := buildConfig(cfgPath)
	if err != nil {
		t.Fatalf("buildConfig failed: %v", err)
	}
	if cfg.GRPCPort != "8000" {
		t.Fatalf("expected env override to 8000, got %s", cfg.GRPCPort)
	}
}

func TestListenAddr_AddsColon(t *testing.T) {
	lis, addr, err := listenAddr("0")
	if err != nil {
		t.Fatalf("listenAddr failed: %v", err)
	}
	defer func() {
		if cerr := lis.Close(); cerr != nil {
			t.Fatalf("failed to close listener: %v", cerr)
		}
	}()

	if addr[0] != ':' {
		t.Fatalf("expected addr to start with ':', got %s", addr)
	}
}

func TestListenAddr_WithColon(t *testing.T) {
	lis, addr, err := listenAddr(":0")
	if err != nil {
		t.Fatalf("listenAddr failed: %v", err)
	}
	defer func() {
		if cerr := lis.Close(); cerr != nil {
			t.Fatalf("failed to close listener: %v", cerr)
		}
	}()

	if addr[0] != ':' {
		t.Fatalf("expected addr to start with ':', got %s", addr)
	}
}

func TestGRPC_EndToEnd(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := lis.Addr().String()

	go func() {
		_ = runServer(lis)
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			t.Fatalf("failed to close conn: %v", cerr)
		}
	}()

	client := wordspb.NewWordsClient(conn)

	if _, err = client.Ping(ctx, &emptypb.Empty{}); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	resp, err := client.Norm(ctx, &wordspb.WordsRequest{Phrase: "Hello,   world!"})
	if err != nil {
		t.Fatalf("Norm failed: %v", err)
	}
	if len(resp.Words) == 0 {
		t.Fatalf("expected non empty words slice")
	}
}
