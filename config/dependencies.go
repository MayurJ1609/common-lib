package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type DependencyConfig struct {
	PostgresDSN      string
	MongoURI         string
	MongoDatabase    string
	MongoUsername    string
	MongoPassword    string
	MongoAuthSource  string
	MongoCollections []string
	RedisAddress     string
	RedisPassword    string
	RedisDB          int
	KafkaBrokers     []string
	KafkaTopic       string
}

func LoadDependencyConfig() (DependencyConfig, error) {
	fileValues, err := loadDependencyEnvFile(".env.example")
	if err != nil {
		return DependencyConfig{}, err
	}

	redisDB, err := strconv.Atoi(dependencyConfigValue("REDIS_DB", "0", fileValues))
	if err != nil {
		return DependencyConfig{}, fmt.Errorf("parse REDIS_DB: %w", err)
	}
	if redisDB < 0 {
		return DependencyConfig{}, fmt.Errorf("REDIS_DB must be non-negative")
	}
	mongoUsername := dependencyConfigValue("MONGODB_USERNAME", "", fileValues)
	mongoPassword := dependencyConfigValue("MONGODB_PASSWORD", "", fileValues)
	if (mongoUsername == "") != (mongoPassword == "") {
		return DependencyConfig{}, fmt.Errorf("MONGODB_USERNAME and MONGODB_PASSWORD must both be set or both be empty")
	}

	brokers := splitDependencyCSV(dependencyConfigValue("KAFKA_BROKERS", "127.0.0.1:9092", fileValues))
	if len(brokers) == 0 {
		return DependencyConfig{}, fmt.Errorf("KAFKA_BROKERS must contain at least one broker")
	}
	collections := splitDependencyCSV(dependencyConfigValue("MONGODB_COLLECTIONS", "items", fileValues))

	return DependencyConfig{
		PostgresDSN:      dependencyConfigValue("POSTGRES_DSN", "postgres://boilerplate:boilerplate@localhost:5432/boilerplate?sslmode=disable", fileValues),
		MongoURI:         dependencyConfigValue("MONGODB_URI", "mongodb://127.0.0.1:27017", fileValues),
		MongoDatabase:    dependencyConfigValue("MONGODB_DATABASE", "boilerplate", fileValues),
		MongoUsername:    mongoUsername,
		MongoPassword:    mongoPassword,
		MongoAuthSource:  dependencyConfigValue("MONGODB_AUTH_SOURCE", "admin", fileValues),
		MongoCollections: collections,
		RedisAddress:     dependencyConfigValue("REDIS_ADDR", "127.0.0.1:6379", fileValues),
		RedisPassword:    dependencyConfigValue("REDIS_PASSWORD", "", fileValues),
		RedisDB:          redisDB,
		KafkaBrokers:     brokers,
		KafkaTopic:       dependencyConfigValue("KAFKA_TOPIC", "boilerplate.events", fileValues),
	}, nil
}

func dependencyConfigValue(key, fallback string, fileValues map[string]string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value := fileValues[key]; value != "" {
		return value
	}
	return fallback
}

func loadDependencyEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("open config file %q: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("parse config file %q at line %d: expected KEY=VALUE", path, lineNumber)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}
	return values, nil
}

func splitDependencyCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}
