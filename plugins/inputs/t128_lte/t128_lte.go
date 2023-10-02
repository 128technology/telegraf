package t128_lte

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

var serviceStatusMap = map[string]string{
	"not-registered-searching": "Unable to Register",
	"no-service":               "No Service Available",
	"authentication-failed":    "Authentication Failure",
	"plmn-not-allowed":         "Forbidden PLMN",
}

var sampleConfig = `
[[inputs.t128_lte]]
## Required. A name for the collector which will be used as the measurement name of the produced data.
# collector_name = "lte-state"
`

type globalInitConfig struct {
	Init struct {
		RouterName string `json:"routerName"`
	} `json:"init"`
}

type localInitConfig struct {
	Init struct {
		Id string `json:"id"`
	} `json:"init"`
}

type t128Config struct {
	Config struct {
		Authority struct {
			Router []struct {
				Name string       `json:"name"`
				Node []nodeConfig `json:"node"`
			} `json:"router"`
		} `json:"authority"`
	} `json:"config"`
}

type nodeConfig struct {
	Name            string `json:"name"`
	DeviceInterface []lteDeviceInterface
}

type lteDeviceInterface struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	TargetInterface string `json:"targetInterface"`
}

type lteDeviceState struct {
	LTE struct {
		Active3GPPProfile struct {
			APN string `json:"APN"`
		} `json:"Active 3GPP Profile"`
		SignalStrength     string `json:"Signal Strength"`
		RSRQSignalStrength string `json:"RSRQ Signal Strength"`
		RSSISignalStrength string `json:"RSSI Signal Strength"`
		RadioInterface     string `json:"Radio Interface"`
		SNRSignalStrength  string `json:"SNR Signal Strength"`
		RSRPSignalStrength string `json:"RSRP Signal Strength"`
		RegistrationStatus string `json:"Registration Status"`
		ActiveBandClass    string `json:"Active Band Class"`
		Carrier            string `json:"Carrier"`
		ConnectionStatus   string `json:"Connection Status"`
		Failure            struct {
			Reason string `json:"reason"`
		} `json:"failure"`
	} `json:"LTE"`
}

type lteDeviceInfo struct {
	DeviceInfo struct {
		IMEI  string `json:"imei"`
		ICCID string `json:"iccid"`
		IMSI  string `json:"imsi"`
	} `json:"Device Info"`
}

// T128Lte is an input for metrics of a 128T router instance
type T128Lte struct {
	CollectorName  string `toml:"collector_name"`
	routerName     string
	nodeName       string
	t128ConfigFile string
	ltePathDir     string
	globalInitPath string
	localInitPath  string
}

// SampleConfig returns the default configuration of the Input
func (*T128Lte) SampleConfig() string {
	return sampleConfig
}

// Description returns a one-sentence description on the Input
func (*T128Lte) Description() string {
	return "Make a 128T LTE query and return the data"
}

func (plugin *T128Lte) Init() error {
	var err error
	plugin.routerName, err = plugin.loadRouterName()
	if err != nil {
		return err
	}
	plugin.nodeName, err = plugin.loadNodeName()
	if err != nil {
		return err
	}
	return nil
}

func (plugin *T128Lte) Gather(acc telegraf.Accumulator) error {
	foundAnyLTEInterface := false
	for _, lteInterface := range plugin.getLteInterfaces() {
		foundAnyLTEInterface = true
		lteMetric, err := plugin.getLteStateInfo(lteInterface, plugin.ltePathDir)
		if !hasRequiredData(lteMetric) {
			acc.AddError(fmt.Errorf("Interface %s does not include required data to be included in the collector", lteInterface.Name))
			continue
		}
		lteInfo, err := plugin.getLteInfo(lteInterface, plugin.ltePathDir)
		if err != nil {
			acc.AddError(fmt.Errorf("Unable to find LTE info %w", err))
			continue
		}
		tags := map[string]string{
			"device-interface": lteInterface.Name,
		}
		fields := map[string]interface{}{
			"signal-strength":   lteMetric.LTE.SignalStrength,
			"carrier":           lteMetric.LTE.Carrier,
			"connection-status": lteMetric.LTE.ConnectionStatus,
			"active-band-class": lteMetric.LTE.ActiveBandClass,
			"apn":               lteMetric.LTE.Active3GPPProfile.APN,
			"service-mode":      lteMetric.LTE.RadioInterface,
			"service-status":    getServiceStatus(lteMetric),
			"imsi":              lteInfo.DeviceInfo.IMSI,
			"imei":              lteInfo.DeviceInfo.IMEI,
			"iccid":             lteInfo.DeviceInfo.ICCID,
		}
		signalValues := getSignalValues(lteMetric)
		for k, v := range signalValues {
			fields[k] = v
		}
		//one last check to make sure the fields don't exist if values are empty
		for fieldName, value := range fields {
			if value == "" {
				delete(fields, fieldName)
			}
		}
		acc.AddFields(
			plugin.CollectorName,
			fields,
			tags,
		)

	}
	if !foundAnyLTEInterface {
		acc.AddError(fmt.Errorf("No LTE devices found in config"))
	}
	return nil

}

func (plugin *T128Lte) loadRouterName() (string, error) {
	var globalData globalInitConfig
	content, err := os.ReadFile(plugin.globalInitPath)
	if err != nil {
		return "", fmt.Errorf("Cannot read the %s : %w", content, err)
	}
	err = json.Unmarshal(content, &globalData)
	if err != nil {
		return "", fmt.Errorf("Cannot unmarshal %s : %w", content, err)
	}
	return globalData.Init.RouterName, nil
}

func (plugin *T128Lte) loadNodeName() (string, error) {
	var localData localInitConfig
	content, err := os.ReadFile(plugin.localInitPath)
	if err != nil {
		return "", fmt.Errorf("Cannot read the %s : %w", content, err)
	}
	err = json.Unmarshal(content, &localData)
	if err != nil {
		return "", fmt.Errorf("Cannot unmarshal %s : %w", content, err)
	}
	return localData.Init.Id, nil
}

func (plugin *T128Lte) getLteInterfaces() []lteDeviceInterface {
	content, err := readT128Config(plugin.t128ConfigFile)
	if err != nil {
		return []lteDeviceInterface{}
	}
	node, err := matchNode(plugin.nodeName, content)
	if err != nil {
		fmt.Errorf("Unable to find node name %s : %w", plugin.nodeName, err)
		return []lteDeviceInterface{}
	}
	deviceInterfaces, err := plugin.getDeviceInterfaces(node)
	if err != nil {
		return []lteDeviceInterface{}
	}
	return deviceInterfaces
}

func readT128Config(t128ConfigFile string) ([]byte, error) {
	content, err := os.ReadFile(t128ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("Cannot read the %s : %w", content, err)
	}
	return content, nil
}

func matchNode(nodeName string, content []byte) (nodeConfig, error) {
	var nodeData t128Config
	err := json.Unmarshal(content, &nodeData)
	if err != nil {
		return nodeConfig{}, fmt.Errorf("Cannot unmarshal %s : %w", content, err)
	}
	for _, router := range nodeData.Config.Authority.Router {
		for _, node := range router.Node {
			if node.Name == nodeName {
				return node, nil
			}
		}
	}
	return nodeConfig{}, fmt.Errorf("No matching node names are available in the config %s", content)
}

func (plugin *T128Lte) getDeviceInterfaces(nodeData nodeConfig) ([]lteDeviceInterface, error) {
	var deviceInterfaces []lteDeviceInterface

	for _, deviceType := range nodeData.DeviceInterface {
		if deviceType.Type == "lte" {
			deviceInterfaces = append(deviceInterfaces, lteDeviceInterface{deviceType.Name, deviceType.Type, deviceType.TargetInterface})
		}
	}
	if deviceInterfaces != nil {
		return deviceInterfaces, nil
	}
	return []lteDeviceInterface{}, fmt.Errorf("Cannot find the type")
}

func (plugin *T128Lte) getLteStateInfo(deviceInterfaceData lteDeviceInterface, pathDir string) (lteDeviceState, error) {
	var lteState lteDeviceState
	targetName := deviceInterfaceData.TargetInterface
	if targetName == "" {
		return lteDeviceState{}, fmt.Errorf("Cannot find target intferace")
	}
	stateFile := filepath.Join(pathDir, fmt.Sprint(targetName, ".state"))
	_, exist := statFile(stateFile)
	if !exist {
		return lteDeviceState{}, fmt.Errorf("Unable to load data from %s", stateFile)
	}
	lteStateInfoContent, _ := ioutil.ReadFile(stateFile)
	err := json.Unmarshal(lteStateInfoContent, &lteState)
	if err != nil {
		return lteDeviceState{}, fmt.Errorf("Cannot unmarshal LTE State Info %s : %w", stateFile, err)
	}
	return lteState, nil
}

func (plugin *T128Lte) getLteInfo(deviceInterfaceData lteDeviceInterface, pathDir string) (lteDeviceInfo, error) {
	var lteInfo lteDeviceInfo
	targetName := deviceInterfaceData.TargetInterface
	if targetName == "" {
		return lteDeviceInfo{}, fmt.Errorf("Cannot find target intferace")
	}
	infoFile := filepath.Join(pathDir, fmt.Sprint(targetName, ".info"))
	_, exist := statFile(infoFile)
	if !exist {
		return lteDeviceInfo{}, fmt.Errorf("Unable to load data from %s", infoFile)
	}
	lteInfoContent, _ := ioutil.ReadFile(infoFile)
	err := json.Unmarshal(lteInfoContent, &lteInfo)
	if err != nil {
		return lteDeviceInfo{}, fmt.Errorf("Cannot unmarshal LTE State Info %s : %w", lteInfoContent, err)
	}
	return lteInfo, nil
}

func statFile(filePath string) (info fs.FileInfo, exists bool) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, false
	}
	return info, true
}

func getServiceStatus(lteMetric lteDeviceState) string {
	if lteMetric.LTE.RegistrationStatus == "registered" && lteMetric.LTE.ConnectionStatus == "connected" {
		return "normal"
	}
	if lteMetric.LTE.Failure.Reason != "" {
		registration_status := serviceStatusMap[lteMetric.LTE.Failure.Reason]
		if registration_status == "" {
			return lteMetric.LTE.Failure.Reason
		}
		return serviceStatusMap[lteMetric.LTE.Failure.Reason]
	}
	return lteMetric.LTE.RegistrationStatus
}

func getSignalValues(lteMetric lteDeviceState) map[string]string {
	signalStrengths := make(map[string]string)
	signalStrengths["rsrp-signal-value"] = getSignalName(lteMetric.LTE.RSRPSignalStrength)
	signalStrengths["rsrq-signal-value"] = getSignalName(lteMetric.LTE.RSRQSignalStrength)
	signalStrengths["signal-value"] = getSignalName(lteMetric.LTE.RSSISignalStrength)
	signalStrengths["snr-signal-value"] = getSignalName(lteMetric.LTE.SNRSignalStrength)
	return signalStrengths
}

func getSignalName(value string) string {
	return strings.Split(value, " ")[0]
}

func hasRequiredData(lteMetric lteDeviceState) bool {
	if lteMetric.LTE.SignalStrength == "" && lteMetric.LTE.RSSISignalStrength == "" {
		return false
	}
	return true
}

func init() {
	inputs.Add("t128_lte", func() telegraf.Input {
		return &T128Lte{
			CollectorName:  "lte-state",
			t128ConfigFile: "/var/run/128technology/config.json",
			ltePathDir:     "/var/run/128technology/lte",
			globalInitPath: "/etc/128technology/global.init",
			localInitPath:  "/etc/128technology/local.init",
		}
	})
}
