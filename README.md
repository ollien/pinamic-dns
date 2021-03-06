# Pinamic DNS
Dynamic DNS for your Raspberry Pi (though it doesn't have to be!). Running the binary updates a DNS record on DigitalOcean.

## Installation
Run the binary (whether in a shell or a cron job) in the same directory as a `config.json`. The `config.json` must contain the following values.

```json
{
	"access_token": "Your DigitalOcean API Access Code",
	"dns_config": {
		"domain": "The domain for which your subdomain will reside",
		"name": "The subdomain you want to point to your IP address",
		"ttl": "The ttl for the domain record"
	}
}
```

## Command Flags

|Flag         |Decription                                                           |
|-------------|---------------------------------------------------------------------|
|--config, -c |Set a path to a `config.json`, if not `./config.json`                |
|--logfile, -l|Redirect output to a logfile                                         |
