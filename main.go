package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/digitalocean/godo"

	"golang.org/x/oauth2"
)

const configPath = "./config.json"

//Config holds the configuration for the application
//Implements oauth2.TokenSource
type Config struct {
	AccessToken string `json:"access_token"`
}

//NewConfig reads the file located at path and returns a new Config
func NewConfig(path string) (Config, error) {
	configReader, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	configDecoder := json.NewDecoder(configReader)
	config := Config{}
	err = configDecoder.Decode(&config)
	if err != nil {
		return Config{}, err
	}

	return config, err
}

//Token returns a new oauth2.token object.
//Required for config to implement oauth2.TokenSource
func (config Config) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: config.AccessToken,
	}, nil
}

func main() {
	config, err := NewConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	oauthClient := oauth2.NewClient(context.Background(), config)
	digitalOceanClient := godo.NewClient(oauthClient)
}
