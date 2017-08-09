package main

import (
	"encoding/json"
	"os"

	"golang.org/x/oauth2"
)

//Config holds the configuration for the application
//Implements oauth2.TokenSource
type Config struct {
	AccessToken string    `json:"access_token"`
	DNSConfig   DNSConfig `json:"dns_config"`
	filepath    string
}

//DNSConfig represents the config of the DNS records that will be updated.
//This basically just uses the properties from godo.DomainRecordEditRequest, however, as this is intended for A records, we omit the Priority, Port, and Weight properties. We also don't need to set the "data" property, as this is something that will be provided by this script.
//Note that the Domain and ID properties are custom and not aprt of DomainRecordEditRequest
//ID is a pointer due to the fact that we need to be able to detect if the id is zero or was just unset.
type DNSConfig struct {
	Domain string `json:"domain"`
	ID     *int   `json:"id,omitempty"`
	Name   string `json:"name"`
	TTL    int    `json:"ttl"`
}

//NewConfig reads the file located at filepath and returns a new Config
func NewConfig(filepath string) (Config, error) {
	configReader, err := os.Open(filepath)

	if err != nil {
		return Config{}, err
	}

	defer configReader.Close()

	configDecoder := json.NewDecoder(configReader)
	config := Config{
		filepath: filepath,
	}
	err = configDecoder.Decode(&config)

	if err != nil {
		return Config{}, err
	}

	return config, err
}

//Write writes the config to the file specified by config.filepath
func (config Config) Write() error {
	configWriter, err := os.OpenFile(config.filepath, os.O_WRONLY|os.O_TRUNC, 0666)

	if err != nil {
		return err
	}

	defer configWriter.Close()
	configEncoder := json.NewEncoder(configWriter)
	err = configEncoder.Encode(config)

	return err
}

//Token returns a new oauth2.token object.
//Required for config to implement oauth2.TokenSource
func (config Config) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: config.AccessToken,
	}, nil
}
