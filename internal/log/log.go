package log

import (
	"log/slog"
	"os"
)

func SetupLogging() {
	// set log level from environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		level = slog.LevelInfo
		logLevel = "info"
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))
	slog.Info("setting log level", "level", logLevel)
}
