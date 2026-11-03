package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MayurJ1609/common-lib/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestKafkaTopicReady(t *testing.T) {
	tests := []struct {
		name     string
		metadata *kafka.MetadataResponse
		want     bool
	}{
		{
			name: "topic and partition are ready",
			metadata: &kafka.MetadataResponse{
				Topics: []kafka.Topic{{
					Name:       "events",
					Partitions: []kafka.Partition{{ID: 0}},
				}},
			},
			want: true,
		},
		{
			name: "topic not found",
			metadata: &kafka.MetadataResponse{
				Topics: []kafka.Topic{{Name: "another-topic"}},
			},
		},
		{
			name: "topic still has metadata error",
			metadata: &kafka.MetadataResponse{
				Topics: []kafka.Topic{{Name: "events", Error: kafka.UnknownTopicOrPartition}},
			},
		},
		{
			name: "partition still has metadata error",
			metadata: &kafka.MetadataResponse{
				Topics: []kafka.Topic{{
					Name: "events",
					Partitions: []kafka.Partition{{
						ID:    0,
						Error: kafka.LeaderNotAvailable,
					}},
				}},
			},
		},
		{
			name: "topic has no partitions",
			metadata: &kafka.MetadataResponse{
				Topics: []kafka.Topic{{Name: "events"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kafkaTopicReady(tt.metadata, "events"); got != tt.want {
				t.Fatalf("kafkaTopicReady() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	cause := errors.New("request failed")
	got := wrapError("last Kafka metadata request", cause)
	if !errors.Is(got, cause) {
		t.Fatalf("wrapError() = %v, want wrapped cause", got)
	}
	if wrapError("last Kafka metadata request", nil) != nil {
		t.Fatal("wrapError(nil) should return nil")
	}
}

func TestConnectValidatesRequiredConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.DependencyConfig
		want string
	}{
		{name: "brokers required", cfg: config.DependencyConfig{MongoDatabase: "app", KafkaTopic: "events"}, want: "Kafka broker"},
		{name: "database required", cfg: config.DependencyConfig{KafkaBrokers: []string{"localhost:9092"}, KafkaTopic: "events"}, want: "MongoDB database"},
		{name: "topic required", cfg: config.DependencyConfig{KafkaBrokers: []string{"localhost:9092"}, MongoDatabase: "app"}, want: "Kafka topic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients, err := Connect(context.Background(), tt.cfg)
			if clients != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Connect() = (%v, %v), want validation error containing %q", clients, err, tt.want)
			}
		})
	}
}

func TestOpenPostgresRejectsInvalidConfiguration(t *testing.T) {
	if _, err := OpenPostgres(context.Background(), PostgresConfig{}); err == nil {
		t.Fatal("OpenPostgres() accepted an empty DSN")
	}
	if _, err := OpenPostgres(context.Background(), PostgresConfig{DSN: "not a PostgreSQL DSN"}); err == nil {
		t.Fatal("OpenPostgres() accepted a malformed DSN")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := OpenPostgres(ctx, PostgresConfig{DSN: "postgres://user:pass@127.0.0.1:5432/test"}); err == nil {
		t.Fatal("OpenPostgres() succeeded with a canceled context")
	}
}

func TestEnsureMongoCollectionValidation(t *testing.T) {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatalf("create MongoDB client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("disconnect MongoDB client: %v", err)
		}
	})

	database := client.Database("test")
	if err := ensureMongoCollection(context.Background(), database, ""); err == nil {
		t.Fatal("ensureMongoCollection() accepted an empty collection name")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ensureMongoCollection(ctx, database, "items"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ensureMongoCollection() error = %v, want canceled context", err)
	}
}

func TestClientsCloseEmptyAndMongoDatabase(t *testing.T) {
	if err := (&Clients{}).Close(context.Background()); err != nil {
		t.Fatalf("Close() on empty clients error = %v", err)
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatalf("create MongoDB client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("disconnect MongoDB client: %v", err)
		}
	})
	clients := &Clients{Mongo: client, mongoDatabase: "test"}
	if got := clients.MongoDatabase().Name(); got != "test" {
		t.Fatalf("MongoDatabase().Name() = %q, want test", got)
	}
	if err := (&Clients{}).fail(errors.New("setup failed")); err == nil || !strings.Contains(err.Error(), "setup failed") {
		t.Fatalf("fail() error = %v, want original failure", err)
	}
}

func TestCacheAndPublishReturnErrorsForCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	clients := &Clients{
		Redis: redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"}),
		Kafka: &kafka.Writer{Addr: kafka.TCP("127.0.0.1:9092"), Topic: "events"},
	}
	t.Cleanup(func() {
		if err := clients.Redis.Close(); err != nil {
			t.Errorf("close Redis client: %v", err)
		}
		if err := clients.Kafka.Close(); err != nil {
			t.Errorf("close Kafka writer: %v", err)
		}
	})

	if _, found, err := clients.GetCache(ctx, "missing"); err == nil || found {
		t.Fatalf("GetCache() = (_, %t, %v), want canceled-context error", found, err)
	}
	if err := clients.SetCache(ctx, "key", "value", time.Minute); err == nil {
		t.Fatal("SetCache() succeeded with a canceled context")
	}
	if err := clients.Publish(ctx, []byte("key"), []byte("value")); err == nil {
		t.Fatal("Publish() succeeded with a canceled context")
	}
}

func TestPingReturnsDependencyErrors(t *testing.T) {
	poolConfig, err := pgxpool.ParseConfig("postgres://user:pass@127.0.0.1:5432/test")
	if err != nil {
		t.Fatalf("parse PostgreSQL config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		pool.Close()
		t.Fatalf("create MongoDB client: %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() {
		pool.Close()
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("disconnect MongoDB client: %v", err)
		}
		if err := redisClient.Close(); err != nil {
			t.Errorf("close Redis client: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	clients := &Clients{
		Postgres:     pool,
		Mongo:        client,
		Redis:        redisClient,
		kafkaBrokers: []string{"127.0.0.1:1"},
	}
	if err := clients.Ping(ctx); err == nil {
		t.Fatal("Ping() succeeded with canceled dependency checks")
	}
}

func TestEnsureKafkaTopicReturnsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ensureKafkaTopic(ctx, "127.0.0.1:9092", "events"); err == nil {
		t.Fatal("ensureKafkaTopic() succeeded with canceled context")
	}
}

func TestCloseConfiguredClients(t *testing.T) {
	poolConfig, err := pgxpool.ParseConfig("postgres://user:pass@127.0.0.1:5432/test")
	if err != nil {
		t.Fatalf("parse PostgreSQL config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		pool.Close()
		t.Fatalf("create MongoDB client: %v", err)
	}
	clients := &Clients{
		Postgres: pool,
		Mongo:    client,
		Redis:    redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"}),
		Kafka:    &kafka.Writer{Addr: kafka.TCP("127.0.0.1:9092"), Topic: "events"},
	}
	if err := clients.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
