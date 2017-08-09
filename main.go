package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/digitalocean/godo"
	"github.com/logrusorgru/aurora"

	"golang.org/x/oauth2"
)

const configPath = "./config.json"

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

//CreateOrUpdateRecord adds a record to DigitalOcean if it does not already exist, and updates it otherwise.
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
