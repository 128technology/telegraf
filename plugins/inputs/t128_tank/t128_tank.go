package t128_tank

import (
	"context"
	"fmt"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

const (
	T128TankAddr = "127.0.0.1"
	T128TankPort = 11011
)

var sampleConfig = `
[[inputs.t128_tank]]
## A name for the collector which will be used as the measurement name of the produced data.
# collector_name = ""

## Required. A path for index file.
# index_file = ""

## Required. Type of topic.
# topic = "events"

## Required. Port Number to get tank data from.
# port_number = 11011

## Required. Port Address to get tank data from.
# port_address = "127.0.0.1"
`

type T128Tank struct {
	CollectorName string `toml:"collector_name"`
	IndexFile     string `toml:"index_file"`
	Topic         string `toml:"topic"`
	PortNumber    int    `toml:"port_number"`
	PortAddress   string `toml:"port_address"`
	stop          chan struct{}
}

func (*T128Tank) SampleConfig() string {
	return sampleConfig
}

func (*T128Tank) Description() string {
	return "Run TANK as a long-running input plugin"
}

func (plugin *T128Tank) Init() error {
	err := plugin.checkConfig()
	if err != nil {
		return err
	}

	return nil
}

func (plugin *T128Tank) Gather(_ telegraf.Accumulator) error {
	return nil
}

func (plugin *T128Tank) Start(acc telegraf.Accumulator) error {

	var reader = NewReader(plugin.PortAddress, plugin.PortNumber, plugin.Topic, plugin.IndexFile, StartIndex)
	go func() {
		reader.Run(context.Background(), plugin.stop)
	}()

	return nil
}

func (plugin *T128Tank) Stop() {
	close(plugin.stop)
}

func (plugin *T128Tank) checkConfig() error {
	if plugin.IndexFile == "" {
		return fmt.Errorf("index_file is a required configuration field")
	}

	if plugin.Topic == "" {
		return fmt.Errorf("topic is a required configuration field")
	}

	return nil
}

func init() {
	inputs.Add("t128_tank", func() telegraf.Input {
		return &T128Tank{
			PortNumber:  T128TankPort,
			PortAddress: T128TankAddr,
			stop:        make(chan struct{}),
		}
	})
}
