package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("APP_HTTP_ADDR", ":8081")

	type settings struct {
		HTTPAddr string `mapstructure:"http_addr"`
	}
	got, err := Load[settings](Options{EnvPrefix: "APP"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.HTTPAddr != ":8081" {
		t.Fatalf("HTTPAddr = %q, want :8081", got.HTTPAddr)
	}
}

func TestLoadFileWithEnvironmentOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("http_addr: ':8080'\n"), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("APP_HTTP_ADDR", ":8081")

	type settings struct {
		HTTPAddr string `mapstructure:"http_addr"`
	}
	got, err := Load[settings](Options{File: path, EnvPrefix: "APP"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.HTTPAddr != ":8081" {
		t.Fatalf("HTTPAddr = %q, want environment override :8081", got.HTTPAddr)
	}
}
