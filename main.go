package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/acarl005/stripansi"
	"gopkg.in/yaml.v2"
)

const MaxDiscordMessageLength = 2000

type Config struct {
	WebhookURL  string
	OneLine     bool
	VerboseMode bool
	Timeout     time.Duration
	MaxRetries  int
}

type ConfigFile struct {
	WebhookURL string `yaml:"webhook_url"`
	Timeout    int    `yaml:"timeout"`
	MaxRetries int    `yaml:"max_retries"`
}

type Logger struct {
	*slog.Logger
	verbose bool
}

type RateLimiter struct {
	tokens chan struct{}
	ticker *time.Ticker
}

type Metrics struct {
	MessagesSent  int64
	MessagesError int64
	BytesSent     int64
	mu            sync.RWMutex
}

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

func loadConfig() (*Config, error) {
	var webhookURL, configFile string
	config := &Config{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}

	flag.StringVar(&webhookURL, "u", "", "Discord Webhook URL")
	flag.BoolVar(&config.OneLine, "1", false, "Send message line-by-line")
	flag.BoolVar(&config.VerboseMode, "v", false, "Verbose mode")
	flag.StringVar(&configFile, "c", "", "Config file path")
	flag.Parse()

	// Precedence: flag > environment variable > config file
	if webhookURL != "" {
		config.WebhookURL = webhookURL
	} else if webhookEnv := os.Getenv("DISCORD_WEBHOOK_URL"); webhookEnv != "" {
		config.WebhookURL = webhookEnv
	}

	if configFile != "" {
		fileConfig, err := loadConfigFile(configFile)
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
		logger := NewLogger(config.VerboseMode)
		logger.Warn("Discord Webhook URL not set!")
	}

	return config, nil
}

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

func loadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ConfigFile
	err = yaml.Unmarshal(data, &config)
	return &config, err
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

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		tokens: make(chan struct{}, 5),
		ticker: time.NewTicker(time.Second),
	}

	// Add initial tokens
	for i := 0; i < 5; i++ {
		rl.tokens <- struct{}{}
	}

	go func() {
		for range rl.ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
			}
		}
	}()

	return rl
}

func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

func (rl *RateLimiter) Close() {
	rl.ticker.Stop()
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncrementSent(bytes int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesSent++
	m.BytesSent += int64(bytes)
}

func (m *Metrics) IncrementError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesError++
}

func (m *Metrics) GetStats() (int64, int64, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.MessagesSent, m.MessagesError, m.BytesSent
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

func isStdin() bool {
	f, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return f.Mode()&os.ModeNamedPipe != 0
}
