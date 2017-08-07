package main

import (
	"encoding/json"
	"os"
)

//Config holds the configuration for the application
type Config struct {
	Token string
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

func main() {

}
