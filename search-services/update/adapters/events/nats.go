package events

import (
	"log/slog"

	"github.com/nats-io/nats.go"
)

type Publisher struct {
	log *slog.Logger
	nc  *nats.Conn
}

func NewPublisher(log *slog.Logger, addr string) (*Publisher, error) {
	nc, err := nats.Connect(addr)
	if err != nil {
		log.Error("failed to connect to nats", "address", addr, "error", err)
		return nil, err
	}
	return &Publisher{log: log, nc: nc}, nil
}

func (p *Publisher) Close() {
	if p.nc != nil {
		p.nc.Close()
	}
}

func (p *Publisher) PublishDBUpdated() {
	if p.nc == nil {
		return
	}
	if err := p.nc.Publish("xkcd.db.updated", nil); err != nil {
		p.log.Error("failed to publish db updated", "error", err)
		return
	}
	if err := p.nc.Flush(); err != nil {
		p.log.Warn("failed to flush nats connection", "error", err)
	}
}
