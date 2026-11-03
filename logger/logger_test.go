package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewConfiguresLevelAndService(t *testing.T) {
	tests := []struct {
		name       string
		level      string
		logMessage string
		want       bool
	}{
		{name: "default info", logMessage: "info message", want: true},
		{name: "debug", level: "DEBUG", logMessage: "debug message", want: true},
		{name: "warning alias", level: "warning", logMessage: "warning message", want: true},
		{name: "error", level: "error", logMessage: "error message", want: true},
		{name: "filtered debug", level: "warn", logMessage: "debug message", want: false},
		{name: "unknown defaults to info", level: "unknown", logMessage: "info message", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			log := New(Options{Level: tt.level, Service: "orders", Output: &output})
			switch {
			case strings.HasPrefix(tt.logMessage, "debug"):
				log.Debug(tt.logMessage)
			case strings.HasPrefix(tt.logMessage, "warning"):
				log.Warn(tt.logMessage)
			case strings.HasPrefix(tt.logMessage, "error"):
				log.Error(tt.logMessage)
			default:
				log.Info(tt.logMessage)
			}

			containsMessage := strings.Contains(output.String(), tt.logMessage)
			if containsMessage != tt.want {
				t.Fatalf("output %q contains message = %t, want %t", output.String(), containsMessage, tt.want)
			}
			if tt.want && !strings.Contains(output.String(), `"service":"orders"`) {
				t.Fatalf("output %q does not include service attribute", output.String())
			}
		})
	}
}
