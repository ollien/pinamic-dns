package main

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/ogier/pflag"
	pinamicdns "github.com/ollien/pinamic-dns"
	"github.com/ollien/xtrace"
)

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

func getIP() (net.IP, error) {
	res, err := http.Get("http://checkip.amazonaws.com/")

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	resData, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	rawIP := strings.Trim(string(resData), "\n")

	return net.ParseIP(rawIP), nil
}

func main() {
	configPath := ""
	logFilePath := ""
	pflag.StringVarP(&configPath, "config", "c", defaultConfigPath, "Set a path to a config.json")
	pflag.StringVarP(&logFilePath, "logfile", "l", "", "Redirect output to a log file.")
	pflag.Parse()

	logWriter := os.Stderr
	if logFilePath != "" {
		logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}

		defer logFile.Close()
		logWriter = logFile
	}
	logger := log.New(logWriter, "", log.LstdFlags)

	config, err := NewConfig(configPath)
	if err != nil {
		logger.Fatal(err)
	}

	var setter pinamicdns.IPSetter
	setter, err = pinamicdns.NewDigitalOceanIPSetter(config, pinamicdns.DigitalOceanRecordTTL(config.DNSConfig.TTL))
	ip, err := getIP()
	if err != nil {
		logger.Fatalf("Could not get IP to update with: %s", err)
	}

	err = setter.SetIP(config.DNSConfig.Domain, config.DNSConfig.Name, ip)
	if err != nil {
		logger.Printf("Could not update record: %s", err)
		tracer, tracerErr := xtrace.NewTracer(err)
		if tracerErr != nil {
			logger.Fatalf("Could not produce error trace: %s", err)
		}

		traceErr := tracer.Trace(logWriter)
		if traceErr != nil {
			logger.Fatalf("Could not produce error trace: %s", err)
		}
		// HACK: Write a newline so there's one after the trace
		// Should probably be done in xtrace
		logWriter.Write([]byte("\n"))

		os.Exit(1)
	}
}
