package config

import (
	"os"
	"testing"
)

func TestLoadFromEnv_Defaults(t *testing.T) {
	os.Clearenv()
	cfg := LoadFromEnv()

	if cfg.ListenAddr != ":8130" {
		t.Fatalf("expected :8130, got %s", cfg.ListenAddr)
	}
	if cfg.ServiceToken != "" {
		t.Fatalf("expected empty token, got %s", cfg.ServiceToken)
	}
	if cfg.MaxBodySize != "10M" {
		t.Fatalf("expected 10M, got %s", cfg.MaxBodySize)
	}
}

func TestLoadFromEnv_Custom(t *testing.T) {
	os.Clearenv()
	t.Setenv("LISTEN_ADDR", ":9999")
	t.Setenv("SERVICE_TOKEN", "mytoken")
	t.Setenv("MAX_BODY_SIZE", "50M")

	cfg := LoadFromEnv()

	if cfg.ListenAddr != ":9999" {
		t.Fatalf("expected :9999, got %s", cfg.ListenAddr)
	}
	if cfg.ServiceToken != "mytoken" {
		t.Fatalf("expected mytoken, got %s", cfg.ServiceToken)
	}
	if cfg.MaxBodySize != "50M" {
		t.Fatalf("expected 50M, got %s", cfg.MaxBodySize)
	}
}
