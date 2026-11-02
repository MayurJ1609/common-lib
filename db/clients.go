package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MayurJ1609/common-lib/config"
	"github.com/MayurJ1609/common-lib/db/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Clients struct {
	Postgres *pgxpool.Pool
	Mongo    *mongo.Client
	Redis    *redis.Client
	Kafka    *kafka.Writer

	mongoDatabase string
	kafkaBrokers  []string
}

func Connect(ctx context.Context, cfg config.DependencyConfig) (*Clients, error) {
	if len(cfg.KafkaBrokers) == 0 {
		return nil, fmt.Errorf("at least one Kafka broker is required")
	}
	if cfg.MongoDatabase == "" {
		return nil, fmt.Errorf("MongoDB database is required")
	}
	if cfg.KafkaTopic == "" {
		return nil, fmt.Errorf("Kafka topic is required")
	}

	clients := &Clients{mongoDatabase: cfg.MongoDatabase, kafkaBrokers: cfg.KafkaBrokers}

	pool, err := OpenPostgres(ctx, PostgresConfig{DSN: cfg.PostgresDSN})
	if err != nil {
		return nil, clients.fail(err)
	}
	clients.Postgres = pool
	if err := migrations.Run(ctx, pool); err != nil {
		return nil, clients.fail(fmt.Errorf("run PostgreSQL migrations: %w", err))
	}

	mongoOptions := options.Client().ApplyURI(cfg.MongoURI)
	if cfg.MongoUsername != "" {
		mongoOptions.SetAuth(options.Credential{
			AuthSource: cfg.MongoAuthSource,
			Username:   cfg.MongoUsername,
			Password:   cfg.MongoPassword,
		})
	}
	mongoClient, err := mongo.Connect(ctx, mongoOptions)
	if err != nil {
		return nil, clients.fail(fmt.Errorf("create MongoDB client: %w", err))
	}
	clients.Mongo = mongoClient
	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		return nil, clients.fail(fmt.Errorf("connect to MongoDB at %s: %w", cfg.MongoURI, err))
	}
	for _, collection := range cfg.MongoCollections {
		if err := ensureMongoCollection(ctx, mongoClient.Database(cfg.MongoDatabase), collection); err != nil {
			return nil, clients.fail(fmt.Errorf("create MongoDB collection %q: %w", collection, err))
		}
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	clients.Redis = redisClient
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, clients.fail(fmt.Errorf("connect to Redis at %s: %w", cfg.RedisAddress, err))
	}

	if err := ensureKafkaTopic(ctx, cfg.KafkaBrokers[0], cfg.KafkaTopic); err != nil {
		return nil, clients.fail(fmt.Errorf("prepare Kafka topic %q: %w", cfg.KafkaTopic, err))
	}
	clients.Kafka = &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Topic:        cfg.KafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
	}
	return clients, nil
}

func ensureMongoCollection(ctx context.Context, database *mongo.Database, name string) error {
	if name == "" {
		return fmt.Errorf("MongoDB collection name is required")
	}
	err := database.CreateCollection(ctx, name)
	if err == nil {
		return nil
	}
	var commandErr mongo.CommandError
	if errors.As(err, &commandErr) && commandErr.Code == 48 {
		return nil
	}
	return err
}

func ensureKafkaTopic(ctx context.Context, brokerAddress, topic string) error {
	client := &kafka.Client{Addr: kafka.TCP(brokerAddress), Timeout: 5 * time.Second}
	response, err := client.CreateTopics(ctx, &kafka.CreateTopicsRequest{
		Topics: []kafka.TopicConfig{{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}},
	})
	if err != nil {
		return fmt.Errorf("request topic creation through broker %s: %w", brokerAddress, err)
	}
	if err := response.Errors[topic]; err != nil && !errors.Is(err, kafka.TopicAlreadyExists) {
		return fmt.Errorf("broker %s rejected topic creation: %w", brokerAddress, err)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastMetadataErr error
	for {
		metadata, err := client.Metadata(ctx, &kafka.MetadataRequest{Topics: []string{topic}})
		if err == nil && kafkaTopicReady(metadata, topic) {
			return nil
		}
		if err != nil {
			lastMetadataErr = err
		}
		select {
		case <-ctx.Done():
			return errors.Join(
				fmt.Errorf("wait for topic %q to become available: %w", topic, ctx.Err()),
				wrapError("last Kafka metadata request", lastMetadataErr),
			)
		case <-ticker.C:
		}
	}
}

func wrapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func kafkaTopicReady(metadata *kafka.MetadataResponse, topicName string) bool {
	for _, topic := range metadata.Topics {
		if topic.Name != topicName || topic.Error != nil || len(topic.Partitions) == 0 {
			continue
		}
		ready := true
		for _, partition := range topic.Partitions {
			if partition.Error != nil {
				ready = false
				break
			}
		}
		if ready {
			return true
		}
	}
	return false
}

func (c *Clients) MongoDatabase() *mongo.Database {
	return c.Mongo.Database(c.mongoDatabase)
}

func (c *Clients) Publish(ctx context.Context, key, value []byte) error {
	return c.Kafka.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}

func (c *Clients) GetCache(ctx context.Context, key string) (string, bool, error) {
	value, err := c.Redis.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (c *Clients) SetCache(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.Redis.Set(ctx, key, value, ttl).Err()
}

func (c *Clients) Ping(ctx context.Context) error {
	var errs []error
	if err := c.Postgres.Ping(ctx); err != nil {
		errs = append(errs, fmt.Errorf("PostgreSQL: %w", err))
	}
	if err := c.Mongo.Ping(ctx, readpref.Primary()); err != nil {
		errs = append(errs, fmt.Errorf("MongoDB: %w", err))
	}
	if err := c.Redis.Ping(ctx).Err(); err != nil {
		errs = append(errs, fmt.Errorf("Redis: %w", err))
	}
	dialer := &kafka.Dialer{Timeout: 5 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", c.kafkaBrokers[0])
	if err != nil {
		errs = append(errs, fmt.Errorf("Kafka: %w", err))
	} else if err := connection.Close(); err != nil {
		errs = append(errs, fmt.Errorf("Kafka: close verification connection: %w", err))
	}
	return errors.Join(errs...)
}

func (c *Clients) Close(ctx context.Context) error {
	var errs []error
	if c.Kafka != nil {
		if err := c.Kafka.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close Kafka writer: %w", err))
		}
	}
	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close Redis: %w", err))
		}
	}
	if c.Mongo != nil {
		if err := c.Mongo.Disconnect(ctx); err != nil {
			errs = append(errs, fmt.Errorf("disconnect MongoDB: %w", err))
		}
	}
	if c.Postgres != nil {
		c.Postgres.Close()
	}
	return errors.Join(errs...)
}

func (c *Clients) fail(cause error) error {
	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(cause, c.Close(closeCtx))
}
