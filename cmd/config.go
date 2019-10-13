package main

import (
	"encoding/json"
	"errors"
	"os"

	"golang.org/x/oauth2"
)

const defaultConfigPath = "./config.json"

// Config holds the configuration for the application
// Implements oauth2.TokenSource
type Config struct {
	AccessToken string    `json:"access_token"`
	DNSConfig   DNSConfig `json:"dns_config"`
}

// DNSConfig represents the config of the DNS records that will be updated.
type DNSConfig struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
	TTL    int    `json:"ttl"`
}

// NewConfig reads the file located at filepath and returns a new Config
func NewConfig(filepath string) (Config, error) {
	configReader, err := os.Open(filepath)
	if err != nil {
		return Config{}, err
	}

	defer configReader.Close()

	configDecoder := json.NewDecoder(configReader)
	var config Config
	err = configDecoder.Decode(&config)
	if err != nil {
		return Config{}, err
	}

	return config, config.validate()
}

// validate returns an error if the config is invalid.
func (config Config) validate() error {
	if config.AccessToken == "" {
		return errors.New("access token must be specified in config")
	} else if config.DNSConfig.Domain == "" {
		return errors.New("domain must be specified in config")
	} else if config.DNSConfig.Name == "" {
		return errors.New("name must be specified in config")
	} else if config.DNSConfig.TTL == 0 {
		return errors.New("ttl must be specified in config")
	}

	return nil
}

// Token returns a new oauth2.token object.
// Required for config to implement oauth2.TokenSource
func (config Config) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: config.AccessToken,
	}, nil
}
