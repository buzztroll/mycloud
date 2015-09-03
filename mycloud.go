package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
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

///////

func detectEffectiveCloud(wg *sync.WaitGroup, cd CloudDetector) {
	cd.detectEffectiveCloud()
	wg.Done()
}

type CloudDetector interface {
	detectEffectiveCloud()
	isEffectiveCloud() bool
	cloudDescription() string
	getKey(string) (*string, error)
}

func setupClouds() []CloudDetector {
	awsCloud := NewAWSCloud()
	gceCloud := GCECloud{BaseCloud{name: "GCE", isMyCloud: false}}
	openStackCloud := NewOpenStackCloud()
	digitalOceanCloud := NewDigitalOceanCloud()
	cdList := []CloudDetector{&awsCloud,
		&gceCloud,
		&openStackCloud,
		&digitalOceanCloud}
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
clouds that support it.

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
