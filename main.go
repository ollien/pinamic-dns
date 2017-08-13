package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/digitalocean/godo"
	"github.com/ogier/pflag"

	"golang.org/x/oauth2"
)

const defaultConfigPath = "./config.json"

//DNSStatusCode represents the result of what CreateOrUpdateRecord did.
type DNSStatusCode int

//DNSResult represents the result of running CreateOrUpdateRecord, including information of its run.
type DNSResult struct {
	IP         string
	StatusCode DNSStatusCode
}

//Possible results for DNSResult
const (
	StatusIPSet DNSStatusCode = iota
	StatusIPUpdated
	StatusIPAlreadySet
)

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
func CreateOrUpdateRecord(config *Config, domainService godo.DomainsService) (DNSResult, error) {
	ip, err := getIP()
	if err != nil {
		return DNSResult{}, err
	}

	editRequest, err := makeEditRequest(*config, ip)
	if err != nil {
		return DNSResult{}, err
	}

	requestContext := context.Background()
	if config.DNSConfig.ID == nil {
		_, _, err := CreateRecord(requestContext, config, &editRequest, domainService)
		if err != nil {
			return DNSResult{}, err
		}

		return DNSResult{
			IP:         ip,
			StatusCode: StatusIPSet,
		}, nil
	}

	record, res, err := domainService.Record(requestContext, config.DNSConfig.Domain, *config.DNSConfig.ID)
	if err != nil {
		//If there is a 404, this means that the id within config.json does not exist on the DigitalOcean servers. More often than not, this will mean that the record has been deleted. As such, we can recreate the record without issue.
		if res.StatusCode == 404 {
			_, _, err := CreateRecord(requestContext, config, &editRequest, domainService)
			if err != nil {
				return DNSResult{}, err
			}

			return DNSResult{
				IP:         ip,
				StatusCode: StatusIPSet,
			}, nil
		}

		return DNSResult{}, err
	}

	if record.Data != ip {
		_, _, err := UpdateRecord(requestContext, config, &editRequest, domainService)
		if err != nil {
			return DNSResult{}, err
		}

		return DNSResult{
			IP:         ip,
			StatusCode: StatusIPUpdated,
		}, nil
	} else if res.StatusCode == 200 {
		//If record.Data does not equal the IP, and there was a 200 result, we can assume that the IP is already set and there are no further problems.
		return DNSResult{
			IP:         ip,
			StatusCode: StatusIPAlreadySet,
		}, nil
	}

	return DNSResult{}, fmt.Errorf("There was an unknown error in setting the '%s' record to '%s'. Query returned status code %d",
		config.DNSConfig.Name,
		ip,
		res.StatusCode)
}

func main() {
	configPath := pflag.StringP("config", "c", defaultConfigPath, "Set a path to a config.json")
	silent := pflag.Bool("silent", false, "Disable all output to stdout. Errors will still be reported to stderr.")
	logFilePath := pflag.StringP("logfile", "l", "", "Redirect output to a log file.")
	pflag.Parse()

	var logFile *os.File
	//In order to avoid shadowing errors when assinging something to logFile, we must declare err now.
	var err error
	if len(*logFilePath) > 0 {
		logFile, err = os.OpenFile(*logFilePath, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}

		defer logFile.Close()
		//Set the log file for any errors that may occur before we log results.
	}

	config, err := NewConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	oauthClient := oauth2.NewClient(context.Background(), config)
	digitalOceanClient := godo.NewClient(oauthClient)
	result, err := CreateOrUpdateRecord(&config, digitalOceanClient.Domains)
	if err != nil {
		log.Fatal(err)
	}

	var writer io.Writer
	fullySilent := false

	if *silent && logFile != nil {
		writer = logFile
	} else if !*silent && logFile != nil {
		writer = io.MultiWriter(os.Stdout, logFile)
	} else if !*silent && logFile == nil {
		writer = os.Stdout
	} else {
		fullySilent = true
	}

	log.SetOutput(writer)

	if !fullySilent {
		switch result.StatusCode {
		case StatusIPSet:
			log.Printf("Succuessfuly set the '%s' record to point to '%s'",
				aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
				aurora.Cyan(aurora.Bold(result.IP)))
		case StatusIPUpdated:
			log.Printf("Succuessfuly updated the '%s' record to point to '%s'",
				aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
				aurora.Cyan(aurora.Bold(result.IP)))
		case StatusIPAlreadySet:
			log.Printf("The '%s' record already points to '%s'",
				aurora.Cyan(aurora.Bold(config.DNSConfig.Name)),
				aurora.Cyan(aurora.Bold(result.IP)))
		}
	}
}
