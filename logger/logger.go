package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Options struct {
	Level   string
	Service string
	Output  io.Writer
}

func New(options Options) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(options.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	output := options.Output
	if output == nil {
		output = os.Stdout
	}
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})
	log := slog.New(handler)
	if options.Service != "" {
		log = log.With("service", options.Service)
	}
	return log
}
