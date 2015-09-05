package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
	"strings"
	"os/exec"
)

type CommandOptions struct {
	verbose bool
	key     string
}

var globalOpts CommandOptions

func logOutput(message string, a ...interface{}) {
	if !globalOpts.verbose {
		return
	}
	fmt.Fprintf(os.Stderr, message, a)
}

func getUrl(url string, headers map[string]string) (*string, *http.Response, error) {
	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	req, _ := http.NewRequest("GET", url, nil)
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, resp, err
	}
	if resp.StatusCode != 200 {
		return nil, resp, errors.New("An error getting the url " + url + " : " + resp.Status)
	}
	out, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, resp, err
	}
	s := string(out)
	return &s, resp, err
}

/////////////////////////////////////////////////////////
//  Base Cloud
/////////////////////////////////////////////////////////
type BaseCloud struct {
	name        string
	isMyCloud   bool
	supportsKey bool
}

func (c *BaseCloud) cloudDescription() string {
	return c.name
}

func (c *BaseCloud) isEffectiveCloud() bool {
	return c.isMyCloud
}

func (c *BaseCloud) supportsKeys() bool {
	return c.supportsKey
}

func (c *BaseCloud) getKey(key string) (*string, error) {
	return nil, errors.New("Cloud does not support keys")
}

/////////////////////////////////////////////////////////
//  A few clouds base their information of a simple
//  http get
/////////////////////////////////////////////////////////
type SimpleUrlBasedCloud struct {
	BaseCloud
	baseUrl  string
	testUrl  string
	metadata *string
}

func (c *SimpleUrlBasedCloud) detectEffectiveCloud() {
	metadata, _, err := getUrl(c.testUrl, map[string]string{})
	c.metadata = metadata
	c.isMyCloud = err == nil
}

func (c *SimpleUrlBasedCloud) getKey(key string) (*string, error) {
	url := c.baseUrl + key
	metadata, _, err := getUrl(url, map[string]string{})
	return metadata, err
}

/////////////////////////////////////////////////////////
// AWS
/////////////////////////////////////////////////////////
type AWSCloud struct {
	SimpleUrlBasedCloud
}

func NewAWSCloud() AWSCloud {
	c := AWSCloud{}
	c.baseUrl = "http://169.254.169.254/latest/meta-data/"
	c.testUrl = "http://169.254.169.254/latest/meta-data/instance-id"
	c.name = "AWS"
	c.supportsKey = true
	return c
}

/////////////////////////////////////////////////////////
// OpenStack
/////////////////////////////////////////////////////////
type OpenStackCloud struct {
	SimpleUrlBasedCloud
}

func NewOpenStackCloud() OpenStackCloud {
	c := OpenStackCloud{}
	c.testUrl = "http://169.254.169.254/openstack/2012-08-10/meta_data.json"
	c.supportsKey = true
	c.name = "OpenStack"
	return c
}

func (c *OpenStackCloud) getKey(key string) (*string, error) {

	dec := json.NewDecoder(strings.NewReader(*c.metadata))

	var m map[string]string
	dec.Decode(&m)
	v := m[key]
	if v == "" {
		return nil, errors.New("No such key " + key)
	}
	return &v, nil
}

/////////////////////////////////////////////////////////
// Digital Ocean
/////////////////////////////////////////////////////////
type DigitalOceanCloud struct {
	SimpleUrlBasedCloud
}

func NewDigitalOceanCloud() DigitalOceanCloud {
	c := DigitalOceanCloud{}
	c.baseUrl = "http://169.254.169.254/metadata/v1/"
	c.testUrl = "http://169.254.169.254/metadata/v1/id"
	c.name = "Digital Ocean"
	c.supportsKey = true
	return c
}

/////////////////////////////////////////////////////////
// GCE
/////////////////////////////////////////////////////////
type GCECloud struct {
	BaseCloud
}

func NewGCECloud() GCECloud {
	c := GCECloud{}
	c.supportsKey = true
	c.name = "GCE"
	return c
}

func (c *GCECloud) detectEffectiveCloud() {
	c.supportsKey = true
	url := "http://metadata.google.internal/"
	headers := map[string]string{"Metadata-Flavor": "Google"}
	_, resp, err := getUrl(url, headers)

	if err != nil {
		c.isMyCloud = false
	} else {
		c.isMyCloud = resp.Header.Get("Metadata-Flavor") == "Google"
	}
}

func (c *GCECloud) getKey(key string) (*string, error) {
	url := "http://metadata.google.internal/computeMetadata/v1/" + key
	headers := map[string]string{"Metadata-Flavor": "Google"}
	metadata, _, err := getUrl(url, headers)
	return metadata, err
}

/////////////////////////////////////////////////////////
// GCE
/////////////////////////////////////////////////////////
type AzureCloud struct {
	BaseCloud
}

func (c *AzureCloud) detectEffectiveCloud() {
	c.supportsKey = true

	c.isMyCloud = false
	if _, err := os.Stat("/var/lib/waagent/ovf-env.xml"); err == nil {
		c.isMyCloud = true
	}
}

/////////////////////////////////////////////////////////
// Joyent
/////////////////////////////////////////////////////////
type JoyentCloud struct {
	BaseCloud
}

func NewJoyentCloud() JoyentCloud {
	c := JoyentCloud{}
	c.supportsKey = true
	c.name = "Joyent"
	return c
}

func (c *JoyentCloud) detectEffectiveCloud() {
	c.supportsKey = true

	c.isMyCloud = false
	if _, err := os.Stat("/usr/sbin/mdata-get"); err == nil {
		c.isMyCloud = true
	}
}

func (c *JoyentCloud) getKey(key string) (*string, error) {
	var cmd string = "/usr/sbin/mdata-get"
	out, err := exec.Command(cmd, key).Output()
	if err != nil {
		return nil, err
	}
	s := string(out)
	return &s, nil
}

///////

func detectEffectiveCloud(wg *sync.WaitGroup, cd CloudDetector) {
	cd.detectEffectiveCloud()
	wg.Done()
}

type CloudDetector interface {
	detectEffectiveCloud()
	isEffectiveCloud() bool
	supportsKeys() bool
	cloudDescription() string
	getKey(string) (*string, error)
}

func setupClouds() []CloudDetector {
	awsCloud := NewAWSCloud()
	gceCloud := NewGCECloud()
	azureCloud := AzureCloud{BaseCloud{name: "Azure"}}
	openStackCloud := NewOpenStackCloud()
	digitalOceanCloud := NewDigitalOceanCloud()
	joyentCloud := NewJoyentCloud()
	cdList := []CloudDetector{
		&awsCloud,
		&gceCloud,
		&azureCloud,
		&openStackCloud,
		&digitalOceanCloud,
		&joyentCloud}
	return cdList
}

func setupOptions(cdList []CloudDetector) {
	usageMessage := `Usage: mycloud
--------------
This program will inspect the local system to determine what cloud it is running
in.  If no cloud can be determined it will return a non zero value and print
the UNKNOWN to stdout.  If a cloud is found it will return 0 and print one of
the following values to stdout:
`
	for _, cd := range cdList {
		usageMessage = usageMessage + "\t" + cd.cloudDescription() + "\n"
	}

	usageMessage = usageMessage + `
Optionally this can be used to fetch keys from the clouds metadata server on the
clouds that support it.  The following clouds support fetching specific metadata
keys:
`
	for _, cd := range cdList {
		if cd.supportsKeys() {
			usageMessage = usageMessage + "\t" + cd.cloudDescription() + "\n"
		}
	}

	usageMessage = usageMessage + `

[options]
`
	var key = flag.String("key", "", "A metadata key to fetch.  This is not supported on all clouds")
	var verbose = flag.Bool("verbose", false, "Log output to stderr as the program progresses")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usageMessage)
		flag.PrintDefaults()
	}

	flag.Parse()

	globalOpts = CommandOptions{key: *key, verbose: *verbose}
}

func main() {
	cdList := setupClouds()
	setupOptions(cdList)
	wg := new(sync.WaitGroup)
	wg.Add(len(cdList))
	for _, cd := range cdList {
		logOutput("Cloud candidate %s\n", cd.cloudDescription())
		go detectEffectiveCloud(wg, cd)
	}
	wg.Wait()

	var rc int = 1
	for _, cd := range cdList {
		if cd.isEffectiveCloud() {
			rc = 0
			fmt.Printf("%s\n", cd.cloudDescription())
			if globalOpts.key != "" {
				val, err := cd.getKey(globalOpts.key)
				if err != nil {
					logOutput("Failed to get the key %s.  Error: %s\n", globalOpts.key, err)
					fmt.Printf("UNKNOWN\n")
					rc = 1
				} else {
					fmt.Printf("%s\n", *val)
				}
			}
			os.Exit(rc)
		}
	}

	fmt.Printf("UNKNOWN\n")
	os.Exit(1)
}
