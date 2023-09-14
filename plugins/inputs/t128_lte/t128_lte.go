package t128_lte

import (
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

const (
	ZK_LTE_STATE_PATH       = "/state/node/{node_name}/interface/{device_id}/network-plugin-state"
	ZK_LTE_DEVICE_INFO_PATH = "network-plugin-info"
)

var SERVICE_STATUS_MAP = map[string]string{
	"not-registered-searching": "Unable to Register",
	"no-service":               "No Service Available",
	"authentication-failed":    "Authentication Failure",
	"plmn-not-allowed":         "Forbidden PLMN",
}

// T128Lte is an input for metrics of a 128T router instance
type T128Lte struct {
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
	return nil
}

func (plugin *T128Lte) Gather(acc telegraf.Accumulator) error {

	return nil
}

func init() {
	inputs.Add("t128_lte", func() telegraf.Input {
		return &T128Lte{}
	})
}
