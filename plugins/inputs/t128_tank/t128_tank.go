package t128_tank

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
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

## Server Address to get tank data from.
# server_address = "127.0.0.1"
`

var (
	typePattern       = `,type=([^\s]+)`
	recordTypePattern = `recordType=([^\s]+)`
)

type T128Tank struct {
	IndexFile     string `toml:"index_file"`
	Topic         string `toml:"topic"`
	PortNumber    int    `toml:"port_number"`
	ServerAddress string `toml:"server_address"`
	Log           telegraf.Logger
	ctx           context.Context
	mainWG        sync.WaitGroup
	cancel        context.CancelFunc
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
	plugin.ctx, plugin.cancel = context.WithCancel(context.Background())
	reader := NewReader(plugin.ServerAddress, plugin.PortNumber, plugin.Topic, plugin.IndexFile, StartIndex, plugin.Log, acc)
	plugin.mainWG.Add(1)
	go func() {
		defer plugin.mainWG.Done()
		for {
			select {
			case <-plugin.ctx.Done():
				return
			case messages := <-reader.sendChan:
				for _, message := range messages {

					tags := map[string]string{
						"index": strconv.FormatUint(message.Index.value, 10),
					}
					if strings.ToLower(plugin.Topic) == "events" {
						messageType := extractMessageType(string(message.Message), typePattern)
						tags["type"] = messageType
					} else if strings.ToLower(plugin.Topic) == "session_records" {
						messageType := extractMessageType(string(message.Message), recordTypePattern)
						tags["recordType"] = messageType
					}
					acc.AddFields("t128_tank", map[string]interface{}{
						"message": string(message.Message),
					}, tags, time.Now())
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

func extractMessageType(message string, pattern string) string {

	regex := regexp.MustCompile(pattern)
	match := regex.FindStringSubmatch(message)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func (plugin *T128Tank) checkConfig() error {
	if plugin.Topic == "" {
		return fmt.Errorf("topic is a required configuration field")
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
