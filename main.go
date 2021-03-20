package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

var IP_PROVIDER = "https://v4.ident.me/"

func getOwnIPv4() (string, error) {
	resp, err := http.Get(IP_PROVIDER)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String(), nil
}

func getDomainIPv4() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.godaddy.com/v1/domains/%s/records/A/%s", DOMAIN, SUBDOMAIN), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", GODADDY_KEY, GODADDY_SECRET))
	c := new(http.Client)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	in := make([]struct {
		Data string `json:"data"`
	}, 1)
	json.NewDecoder(resp.Body).Decode(&in)
	return in[0].Data, nil
}

func putNewIP(ip string) error {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode([]struct {
		Name string `json:"name"`
		Data string `json:"data"`
		TTL  int64  `json:"ttl"`
	}{{
		SUBDOMAIN,
		ip,
		600,
	}})
	if err != nil {
		return err
	}

	log.Debugf("req %s", &buf)

	req, err := http.NewRequest("PUT",
		fmt.Sprintf("https://api.godaddy.com/v1/domains/%s/records/A", DOMAIN),
		&buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", GODADDY_KEY, GODADDY_SECRET))
	c := new(http.Client)
	resp, err := c.Do(req)
	if err != nil {
		log.Errorf("res err %s", err)
		return err
	}
	if resp.StatusCode == 200 {
		log.Debug("update ok")
		return nil
	} else {
		return fmt.Errorf("failed with HTTP status code %d", resp.StatusCode)
	}
}

func run() {
	log.Debug("get own ip -")

	ownIP, err := getOwnIPv4()
	if err != nil {
		log.Errorf("get own ip err, %s", err)
	}

	log.Debug("get own ip: %s", ownIP)

	log.Debug("get domain ip -")

	domainIP, err := getDomainIPv4()
	if err != nil {
		log.Errorf("get domain ip err, %s", err)
	}

	log.Debug("get domain ip: %s", domainIP)

	// if domainIP != ownIP {
	if err := putNewIP(ownIP); err != nil {
		log.Fatal(err)
	}
	// } else {
	// 	log.Infof("same ip, ignore")
	// }
}

// globals
var GODADDY_KEY = ""
var GODADDY_SECRET = ""
var DOMAIN = ""
var SUBDOMAIN = ""

func main() {

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	log.SetLevel(log.DebugLevel)

	// required flags
	keyPtr := flag.String("key", "", "Godaddy API key")
	secretPtr := flag.String("secret", "", "Godaddy API secret")
	domainPtr := flag.String("domain", "", "Your top level domain (e.g., example.com) registered with Godaddy and on the same account as your API key")
	// optional flags
	subdomainPtr := flag.String("subdomain", "@", "The data value (aka host) for the A record. It can be a 'subdomain' (e.g., 'subdomain' where 'subdomain.example.com' is the qualified domain name). Note that such an A record must be set up first in your Godaddy account beforehand. Defaults to @. (Optional)")
	POLLING := flag.Int64("interval", 360, "Polling interval in seconds. Lookup Godaddy's current rate limits before setting too low. Defaults to 360. (Optional)")

	flag.Parse()
	SUBDOMAIN = *subdomainPtr
	DOMAIN = *domainPtr
	GODADDY_SECRET = *secretPtr
	GODADDY_KEY = *keyPtr

	if DOMAIN == "" {
		log.Fatalf("You need to provide your domain")
	}

	if GODADDY_SECRET == "" {
		log.Fatalf("You need to provide your API secret")
	}

	if GODADDY_KEY == "" {
		log.Fatalf("You need to provide your API key")
	}

	// run
	for {
		log.Debug("start")
		run()
		time.Sleep(time.Second * time.Duration(*POLLING))
	}
}
