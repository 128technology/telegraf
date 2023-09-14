package t128_lte

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

const (
	T128_CONFIG_FILE = "/var/run/128technology/config.json"
	LTE_PATH_DIR     = "/var/run/128technology/lte"
	GLOBAL_INIT_PATH = "/etc/128technology/global.init"
	LOCAL_INIT_PATH  = "/etc/128technology/local.init"
)

var SERVICE_STATUS_MAP = map[string]string{
	"not-registered-searching": "Unable to Register",
	"no-service":               "No Service Available",
	"authentication-failed":    "Authentication Failure",
	"plmn-not-allowed":         "Forbidden PLMN",
}

var globalData map[string]interface{}
var localData map[string]interface{}

// T128Lte is an input for metrics of a 128T router instance
type T128Lte struct {
	RouterName string `toml:"router_name"`
	NodeName   string `toml:"node_name"`
}

var sampleConfig = ``

// SampleConfig returns the default configuration of the Input
func (*T128Lte) SampleConfig() string {
	return sampleConfig
}

// Description returns a one-sentence description on the Input
func (*T128Lte) Description() string {
	return "Make a 128T LTE query and return the data"
}

func (plugin *T128Lte) Init() error {
	err := plugin.checkConfig()
	if err != nil {
		return err
	}

	return nil
}

func (plugin *T128Lte) Gather(acc telegraf.Accumulator) error {

	return nil
}

func (plugin *T128Lte) getRouterName() (interface{}, error) {
	content, err := os.ReadFile(GLOBAL_INIT_PATH)
	if err != nil {
		return nil, fmt.Errorf("Cannot read the %s : %w", content, err)
	}
	err = json.Unmarshal(content, &globalData)
	if err != nil {
		return nil, fmt.Errorf("Cannot unmarshal %s : %w", content, err)
	}
	return globalData["routerName"], nil
}

func (plugin *T128Lte) getNodeName() (interface{}, error) {
	content, err := os.ReadFile(LOCAL_INIT_PATH)
	if err != nil {
		return nil, fmt.Errorf("Cannot read the %s : %w", content, err)
	}
	err = json.Unmarshal(content, &globalData)
	if err != nil {
		return nil, fmt.Errorf("Cannot unmarshal %s : %w", content, err)
	}
	return globalData["routerName"], nil
}

func (plugin *T128Lte) getLteInterfaces() error {

	return nil
}

func (plugin *T128Lte) getLteStateInfo(deviceName string) error {
	stateFile := filepath.Join(LTE_PATH_DIR, fmt.Sprint(deviceName, ".state"))
	fileInfo, err := plugin.checkFileExists(stateFile)
	if err != nil {
		return err
	}
	//Read the JSON File
	return nil
}

func (plugin *T128Lte) getLteInfo(deviceName string) error {
	infoFile := filepath.Join(LTE_PATH_DIR, fmt.Sprint(deviceName, ".info"))
	fileInfo, err := plugin.checkFileExists(infoFile)
	if err != nil {
		return err
	}
	//Read the JSON File
	return nil
}

func (plugin *T128Lte) checkFileExists(filePath string) (fs.FileInfo, error) {
	checkFile, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("LTE %s doesn't exists : %w", filePath, err)
	}
	return checkFile, nil
}

func (plugin *T128Lte) checkConfig() error {
	//Add the field to check if config path is present or not?
	return nil
}

func init() {
	inputs.Add("t128_lte", func() telegraf.Input {
		return &T128Lte{}
	})
}
