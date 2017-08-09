package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/digitalocean/godo"
	"github.com/logrusorgru/aurora"

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

func makeEditRequest(config Config, ip string) (godo.DomainRecordEditRequest, error) {
	return godo.DomainRecordEditRequest{
		Type: "A",
		Name: config.DNSConfig.Name,
		Data: ip,
		TTL:  config.DNSConfig.TTL,
	}, nil
}

//CreateRecord creates a record with DigitalOcean.
func CreateRecord(context context.Context, config *Config, editRequest *godo.DomainRecordEditRequest, domainService godo.DomainsService) (*godo.DomainRecord, *godo.Response, error) {
	record, res, err := domainService.CreateRecord(context, config.DNSConfig.Domain, editRequest)
	if err != nil {
		return new(godo.DomainRecord), new(godo.Response), err
	}

	config.DNSConfig.ID = &record.ID
	err = config.Write()

	//Even if there is an error writing the file, it's probably best we still return the existing record and response.
	return record, res, err
}

//UpdateRecord updates an existing record with DigitalOcean
func UpdateRecord(context context.Context, config *Config, editRequest *godo.DomainRecordEditRequest, domainService godo.DomainsService) (*godo.DomainRecord, *godo.Response, error) {
	if config.DNSConfig.ID == nil {
		err := errors.New("config.DNSConfig.ID cannot be nil")
		return new(godo.DomainRecord), new(godo.Response), err
	}

	return domainService.EditRecord(context, config.DNSConfig.Domain, *config.DNSConfig.ID, editRequest)
}

//CreateOrUpdateRecord adds a record to Digit
func CreateOrUpdateRecord(config *Config, domainService godo.DomainsService) error {
	ip, err := getIP()
	if err != nil {
		return err
	}

	editRequest, err := makeEditRequest(*config, ip)
	if err != nil {
		return err
	}

	requestContext := context.Background()
	if config.DNSConfig.ID == nil {
		_, _, err := CreateRecord(requestContext, config, &editRequest, domainService)
		if err != nil {
			return err
		}

		fmt.Printf("Succuessfuly set the '%s' record to point to '%s'",
			aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
			aurora.Cyan(aurora.Bold(ip)))
	} else {
		record, res, err := domainService.Record(requestContext, config.DNSConfig.Domain, *config.DNSConfig.ID)
		if res.StatusCode == 404 {
			_, _, err := CreateRecord(requestContext, config, &editRequest, domainService)
			if err != nil {
				return err
			}

			fmt.Printf("Succuessfuly set the '%s' record to point to '%s'",
				aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
				aurora.Cyan(aurora.Bold(ip)))
		} else if err != nil {
			return err
		} else if record.Data != ip {
			_, _, err := UpdateRecord(requestContext, config, &editRequest, domainService)
			if err != nil {
				return err
			}

			fmt.Printf("Succuessfuly updated the '%s' record to point to '%s'",
				aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
				aurora.Cyan(aurora.Bold(ip)))
		} else if res.StatusCode == 200 {
			fmt.Printf("The '%s' record already points to '%s'",
				aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
				aurora.Cyan(aurora.Bold(ip)))
		} else {
			fmt.Printf(aurora.Sprintf(aurora.Red("There was an unknown error setting the '%s' record to '%s'"),
				aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
				aurora.Cyan(aurora.Bold(ip))))
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

	if err != nil {
		log.Fatal(err)
	}
}
