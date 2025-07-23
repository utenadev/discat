package main

import (
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
	verbose bool
}

func NewLogger(verbose bool) *Logger {
	level := slog.LevelError
	if verbose {
		level = slog.LevelDebug
	}

	return &Logger{
		Logger:  slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})),
		verbose: verbose,
	}
}
