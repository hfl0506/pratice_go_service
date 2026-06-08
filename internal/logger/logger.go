package logger

import (
	"log/slog"
	"os"
)

func Init() *slog.Logger {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)

	return logger
}
