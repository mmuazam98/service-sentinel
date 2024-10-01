package config

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Services        []Service `yaml:"services"`
	AlertWebhookURL string    `yaml:"alert_webhook_url"`
	SlackWebhookURL string    `yaml:"slack_webhook_url"`
}

type Service struct {
	Name string
	URL  string
}

func LoadConfig() Config {
	data, err := os.ReadFile("config/config.yaml")
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Error parsing YAML file: %v", err)
	}

	fmt.Printf("Loaded %d services from config:\n", len(config.Services))
	for _, service := range config.Services {
		fmt.Printf("  - %s: %s\n", service.Name, service.URL)
	}

	return config
}
