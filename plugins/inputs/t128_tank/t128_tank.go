package t128_tank

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/parsers"
)

const (
	T128TankAddr = "127.0.0.1"
	T128TankPort = 11011
)

var sampleConfig = `
[[inputs.t128_tank]]
## A (unique) file to use for index tracking. 
## This tracking allows each event to be produced once.
# index_file = ""

## Required. The TANK topic to consume.
# topic = "events"

## Port Number to get tank data from.
# port_number = 11011

## A field name to display index number
# sequence_number_field = ""

## Server Address to get tank data from.
# server_address = "127.0.0.1"

## From specifies the first message we are interested in.
## If from is "start", it will start consuming from the 
## first available message in the selected topic. 
## If it is "end", it will tail the topic for newly produced messages.
# from = "end"
`

type T128Tank struct {
	IndexFile            string `toml:"index_file"`
	Topic                string `toml:"topic"`
	PortNumber           int    `toml:"port_number"`
	ServerAddress        string `toml:"server_address"`
	SequenceNumberField  string `toml:"sequence_number_field"`
	From                 string `toml:"from"`
	Log                  telegraf.Logger
	ctx                  context.Context
	mainWG               sync.WaitGroup
	cancel               context.CancelFunc
	parser               parsers.Parser
	defaultStartingIndex index
}

func (*T128Tank) SampleConfig() string {
	return sampleConfig
}

func (*T128Tank) Description() string {
	return "Run TANK as a long-running input plugin"
}

func (plugin *T128Tank) SetParser(parser parsers.Parser) {
	plugin.parser = parser
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
	plugin.ctx, plugin.cancel = context.WithCancel(context.Background())
	reader := NewReader(plugin.ServerAddress, plugin.PortNumber, plugin.Topic, plugin.IndexFile, plugin.defaultStartingIndex, plugin.Log, acc)
	plugin.mainWG.Add(1)
	go func() {
		defer plugin.mainWG.Done()
		for {
			select {
			case <-plugin.ctx.Done():
				return
			case messages := <-reader.sendChan:
				for _, message := range messages {
					metrics, err := plugin.parser.Parse(message.Message)
					if err != nil {
						acc.AddError(err)
					}
					for _, metric := range metrics {
						if plugin.SequenceNumberField != "" {
							metric.AddField(plugin.SequenceNumberField, strconv.FormatUint(message.Index.value, 10))
						}
						acc.AddMetric(metric)
					}
				}
			}
		}
	}()

	plugin.mainWG.Add(1)
	go func() {
		defer plugin.mainWG.Done()
		reader.Run(plugin.ctx)
	}()

	return nil
}

func (plugin *T128Tank) Stop() {
	if plugin.cancel != nil {
		plugin.cancel()
	}
	plugin.mainWG.Wait()
}

func (plugin *T128Tank) checkConfig() error {
	if plugin.Topic == "" {
		return fmt.Errorf("topic is a required configuration field")
	}

	if plugin.From != "" {
		err := validateFrom(plugin.From)
		if err != nil {
			return fmt.Errorf("%s", err)
		}
	}

	if strings.ToLower(plugin.From) == "end" {
		plugin.defaultStartingIndex = EndIndex
	} else if strings.ToLower(plugin.From) == "start" {
		plugin.defaultStartingIndex = StartIndex
	} else {
		if plugin.IndexFile == "" {
			plugin.defaultStartingIndex = EndIndex
		} else {
			plugin.defaultStartingIndex = StartIndex
		}
	}
	return nil
}

func validateFrom(from string) error {
	validFromValues := map[string]bool{
		"start": true,
		"end":   true,
	}
	if _, ok := validFromValues[strings.ToLower(from)]; !ok {
		return errors.New("Invalid from value. Accepted values are 'start' or 'end'.")
	}
	return nil
}

func init() {
	inputs.Add("t128_tank", func() telegraf.Input {
		return &T128Tank{
			PortNumber:    T128TankPort,
			ServerAddress: T128TankAddr,
		}
	})
}
