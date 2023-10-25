package t128_tank

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
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
# index-file = ""

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
	IndexFile            string          `toml:"index-file"`
	Topic                string          `toml:"topic"`
	PortNumber           int             `toml:"port_number"`
	ServerAddress        string          `toml:"server_address"`
	SequenceNumberField  string          `toml:"sequence_number_field"`
	From                 string          `toml:"from"`
	Precision            config.Duration `toml:"data_precision"`
	Log                  telegraf.Logger
	ctx                  context.Context
	mainWG               sync.WaitGroup
	cancel               context.CancelFunc
	parser               parsers.Parser
	defaultStartingIndex index
	adjustTime           func(telegraf.Metric)
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

	if plugin.Precision != config.Duration(time.Nanosecond) {
		plugin.adjustTime = func(m telegraf.Metric) {
			adjustedNano := m.Time().UnixNano() * int64(plugin.Precision)
			m.SetTime(time.Unix(0, adjustedNano))
		}
	}
	return nil
}

func (plugin *T128Tank) Gather(_ telegraf.Accumulator) error {
	return nil
}

func (plugin *T128Tank) Start(acc telegraf.Accumulator) error {
	if plugin.adjustTime == nil {
		unreasonableTimestamp := time.Unix(0, 0).Add(24 * time.Hour)
		plugin.adjustTime = func(m telegraf.Metric) {
			mTime := m.Time()
			if mTime.Before(unreasonableTimestamp) {
				adjustedTime := unreasonableTimestamp.Unix() * int64(plugin.Precision)
				m.SetTime(time.Unix(adjustedTime, 0))
			}
		}
	}
	plugin.ctx, plugin.cancel = context.WithCancel(context.Background())
	reader := NewReader(plugin.ServerAddress, plugin.PortNumber, plugin.Topic, plugin.IndexFile, plugin.defaultStartingIndex, plugin.Log)
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
						plugin.adjustTime(metric)
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
			return err
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
	fromLower := strings.ToLower(from)
	if fromLower != "start" && fromLower != "end" {
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
