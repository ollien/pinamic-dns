package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/digitalocean/godo"

	"golang.org/x/oauth2"
)

const configPath = "./config.json"

//Config holds the configuration for the application
//Implements oauth2.TokenSource
type Config struct {
	AccessToken string    `json:"access_token"`
	DNSConfig   DNSConfig `json:"dns_config"`
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

func getIP() (string, error) {
	res, err := http.Get("http://checkip.amazonaws.com/")
	var resData []byte
	_, err = res.Body.Read(resData)
	if err != nil {
		return "", err
	}

	return string(resData), nil
}

//CreateOrUpdateRecord adds a record to Digit
func CreateOrUpdateRecord(config *DNSConfig) error {
	ip, err := getIP()

	if err != nil {
		return err
	}

	dnsOp := godo.DomainsServiceOp{}
	dnsEditRequest := godo.DomainRecordEditRequest{
		Type: "A",
		Name: config.Name,
		Data: ip,
		TTL:  config.TTL,
	}
	requestContext := context.Background()
	if config.ID == nil {
		record, _, err := dnsOp.CreateRecord(requestContext, config.Domain, &dnsEditRequest)
		if err != nil {
			return err
		}
		*config.ID = record.ID
	} else {
		record, _, err := dnsOp.Record(requestContext, config.Domain, *config.ID)
		if err != nil {
			return err
		} else if record.Data != ip {
			_, _, err := dnsOp.EditRecord(requestContext, config.Domain, *config.ID, &dnsEditRequest)
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
}
