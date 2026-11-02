package db

import (
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
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
