package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
)

const MaxDiscordMessageLength = 2000

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type MessageSender interface {
	SendMessage(content string) error
}

type DiscordSender struct {
	client      HTTPClient
	config      *Config
	logger      *Logger
	rateLimiter *RateLimiter
	metrics     *Metrics
	wg          *sync.WaitGroup
}

func NewDiscordSender(config *Config, logger *Logger) *DiscordSender {
	return &DiscordSender{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		config:      config,
		logger:      logger,
		rateLimiter: NewRateLimiter(),
		metrics:     NewMetrics(),
		wg:          &sync.WaitGroup{},
	}
}

func (ds *DiscordSender) ProcessInputWithContext(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	var lines strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ds.logger.Info("Processing interrupted")
			ds.wg.Wait()
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		fmt.Println(line)

		if ds.config.OneLine {
			if ds.config.WebhookURL != "" {
				ds.wg.Add(1)
				go ds.sendMessageWithRetry(line, ds.config.MaxRetries)
			}
		} else {
			lines.WriteString(line)
			lines.WriteString("\n")
		}
	}

	if !ds.config.OneLine && ds.config.WebhookURL != "" {
		content := lines.String()
		messages := ds.splitMessage(content)
		for _, msg := range messages {
			ds.wg.Add(1)
			go ds.sendMessageWithRetry(msg, ds.config.MaxRetries)
		}
	}

	ds.wg.Wait()
	ds.rateLimiter.Close()
	return scanner.Err()
}

func (ds *DiscordSender) splitMessage(content string) []string {
	if len(content) <= MaxDiscordMessageLength {
		return []string{content}
	}

	var messages []string
	lines := strings.Split(content, "\n")
	var current strings.Builder

	for _, line := range lines {
		// If the line is too long for a single message, split it.
		for len(line) > MaxDiscordMessageLength {
			// If there's something in the buffer, flush it first.
			if current.Len() > 0 {
				messages = append(messages, current.String())
				current.Reset()
			}
			messages = append(messages, line[:MaxDiscordMessageLength])
			line = line[MaxDiscordMessageLength:]
		}

		if current.Len()+len(line)+1 > MaxDiscordMessageLength {
			if current.Len() > 0 {
				messages = append(messages, current.String())
				current.Reset()
			}
		}

		current.WriteString(line)
		current.WriteString("\n")
	}

	if current.Len() > 0 {
		messages = append(messages, current.String())
	}

	return messages
}

func (ds *DiscordSender) sendMessageWithRetry(content string, maxRetries int) {
	defer ds.wg.Done()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ds.doSendMessage(content); err == nil {
			ds.metrics.IncrementSent(len(content))
			return
		}

		ds.logger.Debug("Send attempt failed", "attempt", attempt+1, "maxRetries", maxRetries+1)

		if attempt < maxRetries {
			backoff := time.Duration(attempt+1) * time.Second
			time.Sleep(backoff)
		}
	}

	ds.metrics.IncrementError()
	ds.logger.Error("Failed to send message after retries", "maxRetries", maxRetries+1)
}

func (ds *DiscordSender) doSendMessage(content string) error {
	if content == "" {
		return nil
	}

	ds.rateLimiter.Wait()

	payload := struct {
		Content string `json:"content"`
	}{
		Content: stripansi.Strip(content),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		ds.logger.Error("JSON marshal error", "error", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", ds.config.WebhookURL, strings.NewReader(string(data)))
	if err != nil {
		ds.logger.Error("Request creation error", "error", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := ds.client.Do(req)
	if err != nil {
		ds.logger.Error("HTTP request error", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		ds.logger.Error("Discord API error", "status", resp.StatusCode)
		return fmt.Errorf("discord API error: %d", resp.StatusCode)
	}

	ds.logger.Debug("Message sent successfully", "status", resp.StatusCode)
	return nil
}

func (ds *DiscordSender) PrintMetrics(logger *Logger) {
	sent, errors, bytes := ds.metrics.GetStats()
	logger.Info("Metrics",
		"messages_sent", sent,
		"errors", errors,
		"bytes_sent", bytes,
	)
}
