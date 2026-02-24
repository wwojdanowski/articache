package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func Init(level, format string) error {
	parsedLevel, err := parseLevel(level)
	if err != nil {
		return err
	}

	handlerOpts := &slog.HandlerOptions{Level: parsedLevel}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	default:
		return fmt.Errorf("invalid log format %q (expected json or text)", format)
	}

	logger := slog.New(handler).With("service", "articache")
	slog.SetDefault(logger)
	return nil
}

func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q (expected debug, info, warn, error)", level)
	}
}

