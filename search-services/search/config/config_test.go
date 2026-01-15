package config

import (
	"os"
	"testing"
	"time"
)

func TestMustLoad(t *testing.T) {
	tmp, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	data := []byte(`log_level: INFO
search_address: ":9090"
db_address: "db:5432"
words_address: "words:8081"
index_ttl: 30s
broker_address: "nats://example:4223"`)

	if _, err := tmp.Write(data); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	cfg := MustLoad(tmp.Name())

	if cfg.LogLevel != "INFO" ||
		cfg.Address != ":9090" ||
		cfg.DBAddress != "db:5432" ||
		cfg.WordsAddress != "words:8081" ||
		cfg.IndexTTL != 30*time.Second ||
		cfg.BrokerAddress != "nats://example:4223" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
