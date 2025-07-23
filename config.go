package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

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

func loadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ConfigFile
	err = yaml.Unmarshal(data, &config)
	return &config, err
}
