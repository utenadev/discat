package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func setupGracefulShutdown(logger *Logger) context.Context {
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
