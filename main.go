package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	Buildstamp string
	Githash    string
)

var IP_PROVIDER = "http://v4.ident.me/"

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
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.godaddy.com/v1/domains/%s/records/A/%s", DOMAIN, SUBDOMAIN), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", GODADDY_KEY, GODADDY_SECRET))
	c := http.Client{Timeout: 5 * time.Second}

	resp, err := c.Do(req)

	if nil != resp {
		defer resp.Body.Close()
	}

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
	c := http.Client{Timeout: 5 * time.Second}

	resp, err := c.Do(req)

	if nil != resp {
		defer resp.Body.Close()
	}

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

	//设置output,默认为stderr,可以为任何io.Writer，比如文件*os.File
	file, err := os.OpenFile("./dnslog", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	writers := []io.Writer{
		file,
		os.Stdout}
	//同时写文件和屏幕
	fileAndStdoutWriter := io.MultiWriter(writers...)
	if err == nil {
		log.SetOutput(fileAndStdoutWriter)
	} else {
		log.Info("failed to log to file.")
	}

	// required flags
	keyPtr := flag.String("key", "", "Godaddy API key")
	secretPtr := flag.String("secret", "", "Godaddy API secret")
	domainPtr := flag.String("domain", "", "Your top level domain (e.g., example.com) registered with Godaddy and on the same account as your API key")
	// optional flags
	subdomainPtr := flag.String("subdomain", "@", "The data value (aka host) for the A record. It can be a 'subdomain' (e.g., 'subdomain' where 'subdomain.example.com' is the qualified domain name). Note that such an A record must be set up first in your Godaddy account beforehand. Defaults to @. (Optional)")
	POLLING := flag.Int64("interval", 360, "Polling interval in seconds. Lookup Godaddy's current rate limits before setting too low. Defaults to 360. (Optional)")

	var runAsDaemon bool
	var flagversion bool
	flag.BoolVar(&flagversion, "v", false, "version")
	flag.BoolVar(&runAsDaemon, "d", false, "start as daemon")

	flag.Parse()

	if flagversion {
		fmt.Printf("Git Commit Hash: %s\n", Githash)
		fmt.Printf("Build Time : %s\n", Buildstamp)
		return
	}

	if runAsDaemon {

		if os.Getppid() != 1 {

			log.Info("------start server fork------")

			startDaemon()

			log.Info("--------------- server fork finish ---------------")
			return
		}
	}

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
		log.Debug("--start--")
		run()
		log.Debug("---end---")
		time.Sleep(time.Second * time.Duration(*POLLING))
	}
}

// nohup sudo /home/kun/git/godaddns/godaddns -key 9Q1BC4viSQc_c4H5TFupw4QMPME6sHUfU
// -secret c4LP2RNWN4UzwVdQL5SX4 -domain=fangfangtu.com > /dev/null 2>&1 &

func startDaemon() {
	log.Infof("runAsDaemon, current pid = %d", os.Getppid())

	var newarg []string = make([]string, 0)

	//可能有些多余，如果主进程推出较慢可能会有问题

	skip := false
	for _, v := range os.Args {
		if skip {
			skip = false

		} else if v == "-d" {

		} else if v == "-c" {
			skip = true

		} else {
			newarg = append(newarg, v)
		}
	}

	ex, err := os.Executable()
	if err != nil {
		log.Error("get exe path err, ", err.Error())
		return
	}

	// 将其他命令传入生成出的进程
	cmd := exec.Command(ex, newarg[1:]...)

	// cmd.Stdin = os.Stdin // 给新进程设置文件描述符，可以重定向到文件中
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	err = cmd.Start() // 开始执行新进程，不等待新进程退出

	if err != nil {
		log.Error("start cmd err, ", err.Error())
	}
}
