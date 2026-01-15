package events

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func runNATSServer(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		Host:           "127.0.0.1",
		Port:           -1,
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 256,
	}

	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create nats server: %v", err)
	}

	go s.Start()

	if !s.ReadyForConnections(5 * time.Second) {
		s.Shutdown()
		t.Fatalf("nats server not ready")
	}

	return s
}

func TestPublisher_PublishDBUpdated(t *testing.T) {
	s := runNATSServer(t)
	t.Cleanup(func() { s.Shutdown() })

	url := "nats://" + s.Addr().String()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	p, err := NewPublisher(logger, url)
	if err != nil {
		t.Fatalf("failed to create publisher: %v", err)
	}
	t.Cleanup(func() { p.Close() })

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("failed to connect subscriber: %v", err)
	}
	t.Cleanup(func() { nc.Close() })

	msgCh := make(chan *nats.Msg, 1)
	sub, err := nc.ChanSubscribe("xkcd.db.updated", msgCh)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	t.Cleanup(func() {
		if err := sub.Unsubscribe(); err != nil {
			t.Fatalf("failed to unsubscribe: %v", err)
		}
	})

	if err := nc.FlushTimeout(2 * time.Second); err != nil {
		t.Fatalf("failed to flush subscription: %v", err)
	}

	p.PublishDBUpdated()

	select {
	case <-msgCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("did not receive db updated message")
	}
}
