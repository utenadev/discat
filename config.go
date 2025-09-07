package discat

import (
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


func LoadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ConfigFile
	err = yaml.Unmarshal(data, &config)
	return &config, err
}
