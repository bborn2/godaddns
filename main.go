package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	Buildstamp string
	Githash    string
)

var IP_PROVIDER = "http://v4.ident.me/"

type Result struct {
	ID      string `json:"id"`
	ZoneID  string `json:"zone_id"`
	Content string `json:"content"`
}

type Response struct {
	Result  []Result `json:"result"`
	Success bool     `json:"success"`
}

type DNSRecord struct {
	Content string   `json:"content"`
	Name    string   `json:"name"`
	Proxied bool     `json:"proxied"`
	Type    string   `json:"type"`
	Comment string   `json:"comment"`
	Tags    []string `json:"tags"`
	TTL     int      `json:"ttl"`
}

func getOwnIPv4() (string, error) {

	c := http.Client{Timeout: 10 * time.Second}

	resp, err := c.Get(IP_PROVIDER)

	if nil != resp {
		defer resp.Body.Close()
	}

	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String(), nil
}

func getDomainIPv4() (string, error) {

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", ZONEID), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", CF_TOKEN))
	c := http.Client{Timeout: 5 * time.Second}

	resp, err := c.Do(req)

	if nil != resp {
		defer resp.Body.Close()
	}

	if err != nil {
		return "", err
	}

	var response Response
	json.NewDecoder(resp.Body).Decode(&response)

	if response.Success {
		DNSID = response.Result[0].ID

		return response.Result[0].Content, nil
	} else {

		return "", errors.New("get dns record error")
	}
}

func putNewIP(ip string) error {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(DNSRecord{
		Content: ip,
		Name:    DOMAIN,
		Proxied: false,
		Type:    "A",
		TTL:     3600,
	})

	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", ZONEID, DNSID),
		&buf)

	if err != nil {
		log.Error("Error creating request:", err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", CF_TOKEN))
	c := http.Client{Timeout: 5 * time.Second}

	resp, err := c.Do(req)

	if nil != resp {
		defer resp.Body.Close()
	}

	if err != nil {
		log.Errorf("res err %s", err)
		return err
	}

	var response Response
	json.NewDecoder(resp.Body).Decode(&response)

	// log.Debug(response)

	if resp.StatusCode == 200 && response.Success {
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
		return
	}

	log.Debugf("get own ip: %s", ownIP)

	log.Debug("get domain ip -")

	domainIP, err := getDomainIPv4()
	if err != nil {
		log.Errorf("get domain ip err, %s", err)
		return
	}

	log.Debugf("get domain ip: %s", domainIP)

	if domainIP != ownIP {
		if err := putNewIP(ownIP); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Infof("same ip, ignore")
	}
}

// globals
var CF_TOKEN = ""
var ZONEID = ""

var DOMAIN = "@"
var DNSID = ""

func main() {

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	log.SetLevel(log.DebugLevel)

	file, err := os.OpenFile("./dnslog", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	writers := []io.Writer{
		file,
		os.Stdout}

	fileAndStdoutWriter := io.MultiWriter(writers...)
	if err == nil {
		log.SetOutput(fileAndStdoutWriter)
	} else {
		log.Info("failed to log to file.")
	}

	// required flags
	keyPtr := flag.String("key", "", "cf Token")
	// domainPtr := flag.String("domain", "", "Your top level domain (e.g., example.com)")
	zoneidPtr := flag.String("zoneid", "", "Zone id")

	var flagversion bool
	flag.BoolVar(&flagversion, "v", false, "version")

	flag.Parse()

	if flagversion {
		fmt.Printf("Git Commit Hash: %s\n", Githash)
		fmt.Printf("Build Time : %s\n", Buildstamp)
		return
	}

	CF_TOKEN = *keyPtr
	ZONEID = *zoneidPtr

	if CF_TOKEN == "" {
		log.Fatalf("You need to provide your cloudFlare TOKEN")
		return
	}

	if ZONEID == "" {
		log.Fatalf("You need to provide your cloudFlare Zone id")
		return
	}

	run()
}
