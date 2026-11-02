package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDependencyConfigDefaults(t *testing.T) {
	for _, key := range []string{
		"REDIS_DB", "KAFKA_BROKERS", "POSTGRES_DSN", "MONGODB_URI",
		"MONGODB_DATABASE", "MONGODB_USERNAME", "MONGODB_PASSWORD",
		"MONGODB_AUTH_SOURCE", "MONGODB_COLLECTIONS", "REDIS_ADDR",
		"REDIS_PASSWORD", "KAFKA_TOPIC",
	} {
		t.Setenv(key, "")
	}

	cfg, err := LoadDependencyConfig()
	if err != nil {
		t.Fatalf("LoadDependencyConfig() error = %v", err)
	}
	if cfg.PostgresDSN == "" || cfg.MongoURI == "" || cfg.MongoDatabase == "" {
		t.Fatal("database connection defaults must not be empty")
	}
	if cfg.MongoAuthSource != "admin" {
		t.Fatalf("MongoDB auth source = %q, want admin", cfg.MongoAuthSource)
	}
	if len(cfg.MongoCollections) != 1 || cfg.MongoCollections[0] != "items" {
		t.Fatalf("MongoDB collections = %v, want [items]", cfg.MongoCollections)
	}
	if cfg.RedisAddress != "127.0.0.1:6379" || cfg.RedisDB != 0 {
		t.Fatalf("unexpected Redis defaults: address=%q db=%d", cfg.RedisAddress, cfg.RedisDB)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "127.0.0.1:9092" {
		t.Fatalf("unexpected Kafka brokers: %v", cfg.KafkaBrokers)
	}
}

func TestLoadDependencyConfigMongoCredentials(t *testing.T) {
	t.Setenv("MONGODB_USERNAME", "app-user")
	t.Setenv("MONGODB_PASSWORD", "secret")
	t.Setenv("MONGODB_AUTH_SOURCE", "boilerplate")

	cfg, err := LoadDependencyConfig()
	if err != nil {
		t.Fatalf("LoadDependencyConfig() error = %v", err)
	}
	if cfg.MongoUsername != "app-user" || cfg.MongoPassword != "secret" || cfg.MongoAuthSource != "boilerplate" {
		t.Fatal("MongoDB auth config was not loaded correctly")
	}
}

func TestLoadDependencyConfigRejectsIncompleteMongoCredentials(t *testing.T) {
	t.Setenv("MONGODB_USERNAME", "app-user")
	t.Setenv("MONGODB_PASSWORD", "")
	if _, err := LoadDependencyConfig(); err == nil {
		t.Fatal("LoadDependencyConfig() accepted incomplete MongoDB credentials")
	}
}

func TestLoadDependencyConfigKafkaBrokers(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", " broker1:9092, ,broker2:9092 ")
	cfg, err := LoadDependencyConfig()
	if err != nil {
		t.Fatalf("LoadDependencyConfig() error = %v", err)
	}
	if len(cfg.KafkaBrokers) != 2 || cfg.KafkaBrokers[0] != "broker1:9092" || cfg.KafkaBrokers[1] != "broker2:9092" {
		t.Fatalf("Kafka brokers = %v, want trimmed broker list", cfg.KafkaBrokers)
	}
}

func TestLoadDependencyConfigRejectsInvalidRedisDB(t *testing.T) {
	t.Setenv("REDIS_DB", "invalid")
	if _, err := LoadDependencyConfig(); err == nil {
		t.Fatal("LoadDependencyConfig() accepted invalid REDIS_DB")
	}
}

func TestLoadDependencyConfigReadsEnvExample(t *testing.T) {
	tempDir := t.TempDir()
	envFile := "POSTGRES_DSN=postgres://file-value\nREDIS_ADDR=redis-from-file:6379\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".env.example"), []byte(envFile), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(workingDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	t.Setenv("POSTGRES_DSN", "postgres://environment-value")
	t.Setenv("REDIS_ADDR", "")
	cfg, err := LoadDependencyConfig()
	if err != nil {
		t.Fatalf("LoadDependencyConfig() error = %v", err)
	}
	if cfg.PostgresDSN != "postgres://environment-value" {
		t.Fatalf("Postgres DSN = %q, want environment value", cfg.PostgresDSN)
	}
	if cfg.RedisAddress != "redis-from-file:6379" {
		t.Fatalf("Redis address = %q, want value from .env.example", cfg.RedisAddress)
	}
}

func TestLoadDependencyConfigRejectsMalformedEnvExample(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, ".env.example"), []byte("not-a-key-value-pair\n"), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(workingDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	if _, err := LoadDependencyConfig(); err == nil {
		t.Fatal("LoadDependencyConfig() accepted malformed .env.example")
	}
}
