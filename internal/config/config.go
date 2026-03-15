package config

import "os"

type Config struct {
	ListenAddr   string
	ServiceToken string
	MaxBodySize  string
}

func LoadFromEnv() *Config {
	return &Config{
		ListenAddr:   envStr("LISTEN_ADDR", ":8130"),
		ServiceToken: envStr("SERVICE_TOKEN", ""),
		MaxBodySize:  envStr("MAX_BODY_SIZE", "10M"),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
