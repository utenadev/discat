package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/utenadev/discat"
)

func main() {
	config, err := loadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger := discat.NewLogger(config.VerboseMode)

	if !isStdin() {
		os.Exit(1)
	}

	ctx := setupGracefulShutdown(logger)

	sender := discat.NewDiscordSender(config, logger)

	if err := sender.ProcessInputWithContext(ctx, os.Stdin); err != nil {
		if config.VerboseMode {
			logger.Error("Processing error", "error", err)
		}
		os.Exit(1)
	}

	if config.VerboseMode {
		sender.PrintMetrics(logger)
	}
}

func loadConfig() (*discat.Config, error) {
	var webhookURL, configFile string
	config := &discat.Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}

	flag.StringVar(&webhookURL, "u", "", "Discord Webhook URL")
	flag.BoolVar(&config.OneLine, "1", false, "Send message line-by-line")
	flag.BoolVar(&config.VerboseMode, "v", false, "Verbose mode")
	flag.StringVar(&configFile, "c", "", "Config file path")
	flag.Parse()

	if webhookURL != "" {
		config.WebhookURL = webhookURL
	} else if webhookEnv := os.Getenv("DISCORD_WEBHOOK_URL"); webhookEnv != "" {
		config.WebhookURL = webhookEnv
	}

	if configFile != "" {
		fileConfig, err := discat.LoadConfigFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}

		if config.WebhookURL == "" {
			config.WebhookURL = fileConfig.WebhookURL
		}
		if fileConfig.Timeout > 0 {
			config.Timeout = time.Duration(fileConfig.Timeout) * time.Second
		}
		if fileConfig.MaxRetries > 0 {
			config.MaxRetries = fileConfig.MaxRetries
		}
	}

	if config.WebhookURL == "" {
		logger := discat.NewLogger(config.VerboseMode)
		logger.Warn("Discord Webhook URL not set!")
	}

	return config, nil
}

func isStdin() bool {
	f, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return f.Mode()&os.ModeNamedPipe != 0
}

func setupGracefulShutdown(logger *discat.Logger) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal")
		cancel()
	}()
	return ctx
}