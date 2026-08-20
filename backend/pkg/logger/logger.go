package logger

import (
	"log/slog"
	"os"
)

var L *slog.Logger

func Init(mode string) {
	var handler slog.Handler
	if mode == "release" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	L = slog.New(handler)
	slog.SetDefault(L)
}
