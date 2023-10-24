package t128_tank

import (
	"context"
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
## If from is "beginning" or "start", it will start consuming from the 
## first available message in the selected topic. If it is "eof" or "end", 
## it will tail the topic for newly produced messages.
# from = "end"

## Tracking of sent messages allows the input to stop streaming from TANK until
## existing messages have been processed. This can prevent buffer overflow in
## telegraf which would drop messages. When batch size or in flight batches is
## non-zero, all of these tracking fields must be non-zero.
# tracking_max_in_flight_batches = 0
# tracking_max_batch_size = 0
# tracking_max_batch_delay = "1s"
`

type T128Tank struct {
	IndexFile           string `toml:"index_file"`
	Topic               string `toml:"topic"`
	PortNumber          int    `toml:"port_number"`
	ServerAddress       string `toml:"server_address"`
	SequenceNumberField string `toml:"sequence_number_field"`
	From                string `toml:"from"`

	TrackingMaxBatches    int             `toml:"tracking_max_in_flight_batches"`
	TrackingMaxBatchSize  int             `toml:"tracking_max_batch_size"`
	TrackingMaxBatchDelay config.Duration `toml:"tracking_max_batch_delay"`
	Precision             config.Duration `toml:"data_precision"`

	Log        telegraf.Logger
	ctx        context.Context
	mainWG     sync.WaitGroup
	cancel     context.CancelFunc
	parser     parsers.Parser
	reader     *Reader
	indexValue index

	adjustTime func(telegraf.Metric)
}

func (*T128Tank) SampleConfig() string {
	return sampleConfig
}

func (*T128Tank) Description() string {
	return "Run TANK as a long-running input plugin"
}

type parserWithTimePrecision interface {
	SetTimePrecision(time.Duration)
}

func (plugin *T128Tank) SetParser(parser parsers.Parser) {
	plugin.parser = parser
	if precisionParser, ok := parser.(parserWithTimePrecision); ok {
		precisionParser.SetTimePrecision(time.Duration(plugin.Precision))
	}
}

func (plugin *T128Tank) Init() error {
	err := plugin.checkConfig()
	if err != nil {
		return err
	}

	if (plugin.TrackingMaxBatches > 0) != (plugin.TrackingMaxBatchSize > 0) {
		return fmt.Errorf(
			"Fields 'tracking_max_in_flight_batches' (%v) and 'tracking_max_batch_size' (%v) must both be zero or non-zero",
			plugin.TrackingMaxBatches,
			plugin.TrackingMaxBatchSize,
		)
	}

	if plugin.TrackingMaxBatches > 0 && plugin.TrackingMaxBatchDelay <= 0 {
		return fmt.Errorf("Field 'tracking_max_batch_delay' must be positive when 'tracking_max_in_flight_batches' is non-zero")
	}

	plugin.reader = NewReader(
		plugin.ServerAddress,
		plugin.PortNumber,
		plugin.Topic,
		plugin.IndexFile,
		plugin.indexValue,
		plugin.Log,
	)

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
		plugin.adjustTime = func(telegraf.Metric) {}
	}

	if plugin.TrackingMaxBatches > 0 {
		return plugin.startWithTracking(acc)
	}

	return plugin.startWithoutTracking(acc)
}

func (plugin *T128Tank) startWithoutTracking(acc telegraf.Accumulator) error {
	plugin.ctx, plugin.cancel = context.WithCancel(context.Background())
	plugin.mainWG.Add(1)

	go func() {
		defer plugin.mainWG.Done()

		readerBK := &readerBookkeeping{reader: plugin.reader}
		readerBK.launch(plugin.ctx)

		for {
			select {
			case <-plugin.ctx.Done():
				readerBK.stop()
				return
			case messages := <-readerBK.sendChan:
				if len(messages) == 0 {
					continue
				}

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
				readerBK.sendComplete <- messages[len(messages)-1].Index.value
			}
		}
	}()

	return nil
}

func (plugin *T128Tank) startWithTracking(ac telegraf.Accumulator) error {
	plugin.ctx, plugin.cancel = context.WithCancel(context.Background())

	maxTrackedBatches := plugin.TrackingMaxBatches
	maxBatchSize := plugin.TrackingMaxBatchSize
	maxBatchDelay := time.Duration(plugin.TrackingMaxBatchDelay)

	acc := ac.WithTracking(maxTrackedBatches)
	batchTimer := time.NewTicker(maxBatchDelay)

	plugin.Log.Infof("precision is %v", plugin.Precision)

	plugin.mainWG.Add(1)
	inFlightBatches := map[telegraf.TrackingID]uint64{}
	go func() {
		defer plugin.mainWG.Done()

		accumulatingGroup := make([]telegraf.Metric, 0, maxBatchSize)
		lastAccumulatingIndex := uint64(0)

		readerBK := &readerBookkeeping{reader: plugin.reader}
		readerBK.launch(plugin.ctx)

		sendBatch := func() {
			if len(inFlightBatches) >= maxTrackedBatches {
				readerBK.stop()
				batchTimer.Reset(maxBatchDelay)
			}

			trackingID := acc.AddTrackingMetricGroup(accumulatingGroup)
			inFlightBatches[trackingID] = lastAccumulatingIndex
			accumulatingGroup = make([]telegraf.Metric, 0, maxBatchSize)
		}

		for {
			select {
			case <-plugin.ctx.Done():
				readerBK.stop()
				return
			case group := <-acc.Delivered():
				groupID := group.ID()
				if index, ok := inFlightBatches[groupID]; ok {
					plugin.Log.Infof("delivered group ID %v with index %v", groupID, index)
					delete(inFlightBatches, groupID)
					readerBK.sendComplete <- index
				}
				if len(inFlightBatches) == 0 && !readerBK.running {
					readerBK.relaunch(plugin.ctx)
				}
			case messages := <-readerBK.sendChan:
				for _, message := range messages {
					metrics, err := plugin.parser.Parse(message.Message)
					if err != nil {
						acc.AddError(err)
					}
					if len(accumulatingGroup) == 0 {
						// avoid sending a batch very soon after the first entry is added
						batchTimer.Reset(maxBatchDelay)
					}
					for _, metric := range metrics {
						plugin.adjustTime(metric)
					}
					accumulatingGroup = append(accumulatingGroup, metrics...)
					if len(accumulatingGroup) >= maxBatchSize {
						sendBatch()
					}
					lastAccumulatingIndex = message.Index.value
				}
			case <-batchTimer.C:
				if len(accumulatingGroup) > 0 {
					sendBatch()
				}
			}
		}
	}()

	return nil
}

type readerBookkeeping struct {
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	// send processed indices here
	sendComplete chan uint64
	sendChan     chan []IndexedMessage
	reader       *Reader
}

func (r *readerBookkeeping) relaunch(ctx context.Context) {
	close(r.sendComplete)
	close(r.sendChan)
	r.wg.Wait()
	r.launch(ctx)
}

func (r *readerBookkeeping) launch(ctx context.Context) {
	r.running = true
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.wg = sync.WaitGroup{}
	r.sendChan = make(chan []IndexedMessage)
	r.sendComplete = make(chan uint64)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.reader.log.Infof("Launching the tank-cli sub-command")
		r.reader.Run(r.ctx, r.sendChan, r.sendComplete)
	}()
}

func (r *readerBookkeeping) stop() {
	if r.running {
		r.reader.log.Infof("Stopping the tank-cli sub-command")
		r.cancel()
		r.running = false
	}
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

	if plugin.IndexFile == "" {
		if strings.ToLower(plugin.From) == "end" || strings.ToLower(plugin.From) == "eof" {
			plugin.indexValue = EndIndex
		} else if strings.ToLower(plugin.From) == "beginning" || strings.ToLower(plugin.From) == "start" {
			plugin.indexValue = StartIndex
		} else {
			plugin.indexValue = EndIndex
		}
	} else {
		if strings.ToLower(plugin.From) == "end" || strings.ToLower(plugin.From) == "eof" {
			plugin.indexValue = EndIndex
		} else if strings.ToLower(plugin.From) == "beginning" || strings.ToLower(plugin.From) == "start" {
			plugin.indexValue = StartIndex
		} else {
			plugin.indexValue = StartIndex
		}
	}

	return nil
}

func init() {
	inputs.Add("t128_tank", func() telegraf.Input {
		return &T128Tank{
			PortNumber:            T128TankPort,
			ServerAddress:         T128TankAddr,
			TrackingMaxBatchDelay: config.Duration(1 * time.Second),
			Precision:             config.Duration(1 * time.Nanosecond),
		}
	})
}
