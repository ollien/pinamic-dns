package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/digitalocean/godo"

	"golang.org/x/oauth2"
)

const configPath = "./config.json"

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
	configWriter, err := os.Open(config.filepath)

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

func getIP() (string, error) {
	res, err := http.Get("http://checkip.amazonaws.com/")

	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	resData, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return "", err
	}

	ip := strings.Trim(string(resData), "\n")

	return ip, nil
}

//CreateOrUpdateRecord adds a record to Digit
func CreateOrUpdateRecord(config *Config, domainService godo.DomainsService) error {
	ip, err := getIP()

	if err != nil {
		return err
	}

	dnsEditRequest := godo.DomainRecordEditRequest{
		Type: "A",
		Name: config.DNSConfig.Name,
		Data: ip,
		TTL:  config.DNSConfig.TTL,
	}
	requestContext := context.Background()
	if config.DNSConfig.ID == nil {
		record, _, err := domainService.CreateRecord(requestContext, config.DNSConfig.Domain, &dnsEditRequest)
		if err != nil {
			return err
		}
		config.DNSConfig.ID = &record.ID
		config.Write()
	} else {
		record, _, err := domainService.Record(requestContext, config.DNSConfig.Domain, *config.DNSConfig.ID)
		if err != nil {
			return err
		} else if record.Data != ip {
			_, _, err := domainService.EditRecord(requestContext, config.DNSConfig.Domain, *config.DNSConfig.ID, &dnsEditRequest)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	config, err := NewConfig(configPath)

	if err != nil {
		log.Fatal(err)
	}

	oauthClient := oauth2.NewClient(context.Background(), config)
	digitalOceanClient := godo.NewClient(oauthClient)
	err = CreateOrUpdateRecord(&config, digitalOceanClient.Domains)
	//TODO: write changes to config.DNSConfig to disk

	if err != nil {
		log.Fatal(err)
	}
}
