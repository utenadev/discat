package main

import (
	"log/slog"
	"os"
)

func main() {
	config, err := loadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := NewLogger(config.VerboseMode)

	if !isStdin() {
		os.Exit(1)
	}

	// Setup graceful shutdown
	ctx := setupGracefulShutdown(logger)

	sender := NewDiscordSender(config, logger)

	if err := sender.ProcessInputWithContext(ctx); err != nil {
		if config.VerboseMode {
			logger.Error("Processing error", "error", err)
		}
		os.Exit(1)
	}

	// Print metrics
	if config.VerboseMode {
		sender.PrintMetrics(logger)
	}
}