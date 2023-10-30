package t128_tank

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/plugins/parsers/influx"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
)

type mockTankCommadContext func(ctx context.Context, name string, arg ...string) *exec.Cmd

func (mctx mockTankCommadContext) CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return mctx(ctx, name, arg...)
}

func TestT128TankReader(t *testing.T) {
	testcases := []struct {
		Name                   string
		IndexFile              string
		IndexFileContent       string
		Topic                  string
		PortNumber             int
		ServerAddress          string
		TankReadCommandContext mockTankCommadContext
		DefaultIndex           index
		ExpectedMetrics        []IndexedMessage
	}{
		{
			Name:          "index file with no value",
			Topic:         "events",
			PortNumber:    11011,
			ServerAddress: "127.0.0.2",
			DefaultIndex:  StartIndex,
			TankReadCommandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
				cmd := exec.CommandContext(ctx, "echo", "seq=1:measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775")
				return cmd
			},
			ExpectedMetrics: []IndexedMessage{
				{
					Message: []byte("measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775"),
					Index:   index{value: 1},
				},
			},
		},
		{
			Name:             "index file with value",
			IndexFileContent: "150",
			Topic:            "events",
			PortNumber:       11011,
			ServerAddress:    "127.0.0.2",
			DefaultIndex:     StartIndex,
			TankReadCommandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
				cmd := exec.CommandContext(ctx, "echo", "seq=150:measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775")
				return cmd
			},
			ExpectedMetrics: []IndexedMessage{
				{
					Message: []byte("measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775"),
					Index:   index{value: 150},
				},
			},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			plugin := &T128Tank{
				IndexFile:     testcase.IndexFile,
				Topic:         testcase.Topic,
				PortNumber:    testcase.PortNumber,
				ServerAddress: testcase.ServerAddress,
			}

			var receivedMessages []IndexedMessage
			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			indexFileName := strings.ReplaceAll(testcase.Name, " ", "_")
			plugin.IndexFile = path.Join(t.TempDir(), fmt.Sprintf("%s.index", indexFileName))
			if testcase.IndexFileContent != "" {
				err := os.WriteFile(plugin.IndexFile, []byte(testcase.IndexFileContent), 0755)
				assert.NoError(t, err)
			}
			reader := NewReader(
				plugin.ServerAddress,
				plugin.PortNumber,
				plugin.Topic,
				plugin.IndexFile,
				testcase.DefaultIndex,
				testutil.Logger{},
			).withTankReadCommandContext(testcase.TankReadCommandContext)
			wg.Add(1)
			go func() {
				defer wg.Done()
				reader.Run(ctx)
			}()

			select {
			case <-time.After(5 * time.Second):
				t.Log("no messages received")
			case <-ctx.Done():
				t.Log("context canceled")
			case messages := <-reader.sendChan:
				receivedMessages = append(receivedMessages, messages...)
			}

			cancel()

			if len(testcase.ExpectedMetrics) > 0 && receivedMessages != nil {
				assert.Equal(t, testcase.ExpectedMetrics, receivedMessages)

			}
		})
	}
}

func TestBoundaryFault(t *testing.T) {
	testcases := []struct {
		Name                   string
		IndexFile              string
		IndexFileContent       string
		Topic                  string
		PortNumber             int
		ServerAddress          string
		TankReadCommandContext mockTankCommadContext
		DefaultIndex           index
		ExpectedError          string
	}{
		{
			Name:          "boundary-fault",
			Topic:         "events",
			PortNumber:    11011,
			ServerAddress: "127.0.0.2",
			DefaultIndex:  StartIndex,
			TankReadCommandContext: func(ctx context.Context, name string, arg ...string) *exec.Cmd {
				return exec.CommandContext(ctx, "echo", "Boundary Check fault. first available sequence number is 150, high watermark is 200")
			},
			ExpectedError: "parsed boundary fault: next available index is 150",
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			plugin := &T128Tank{
				IndexFile:     testcase.IndexFile,
				Topic:         testcase.Topic,
				PortNumber:    testcase.PortNumber,
				ServerAddress: testcase.ServerAddress,
			}

			var wg sync.WaitGroup
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			indexFileName := strings.ReplaceAll(testcase.Name, " ", "_")
			plugin.IndexFile = path.Join(t.TempDir(), fmt.Sprintf("%s.index", indexFileName))
			if testcase.IndexFileContent != "" {
				err := os.WriteFile(plugin.IndexFile, []byte(testcase.IndexFileContent), 0755)
				assert.NoError(t, err)
			}
			reader := NewReader(
				plugin.ServerAddress,
				plugin.PortNumber,
				plugin.Topic,
				plugin.IndexFile,
				testcase.DefaultIndex,
				testutil.Logger{},
			).withTankReadCommandContext(testcase.TankReadCommandContext)
			var receivedErrorMessage string
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := reader.readFromTank(ctx, StartIndex)
				if err != nil {
					receivedErrorMessage = err.Error()
				}
			}()

			wg.Wait()

			assert.Equal(t, testcase.ExpectedError, receivedErrorMessage)
		})
	}
}

func TestParseTankLine(t *testing.T) {
	testcases := []struct {
		Name            string
		InputMessage    string
		ExpectedMessage *IndexedMessage
	}{
		{
			Name:         "input-message",
			InputMessage: "seq=861:events,collector_id=auditd,node=westB,subtype=authentication,type=admin event_detail='node=t137-dut3.openstacklocal type=USER_AUTH msg=audit(1697466322.733:10608): pid=16967 uid=0 auid=4294967295 ses=4294967295 msg='op=PAM:authentication grantors=pam_faillock,pam_unix acct=\"centos\" exe=\"/usr/sbin/sshd\" hostname=172.18.15.253 addr=172.18.15.253 terminal=ssh res=success'',permitted=t,user='centos' 1697466322733010608",
			ExpectedMessage: &IndexedMessage{
				Message: []byte("events,collector_id=auditd,node=westB,subtype=authentication,type=admin event_detail='node=t137-dut3.openstacklocal type=USER_AUTH msg=audit(1697466322.733:10608): pid=16967 uid=0 auid=4294967295 ses=4294967295 msg='op=PAM:authentication grantors=pam_faillock,pam_unix acct=\"centos\" exe=\"/usr/sbin/sshd\" hostname=172.18.15.253 addr=172.18.15.253 terminal=ssh res=success'',permitted=t,user='centos' 1697466322733010608"),
				Index:   index{value: 861},
			},
		},
		{
			Name:         "simple",
			InputMessage: "seq=150:measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775",
			ExpectedMessage: &IndexedMessage{
				Message: []byte("measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775"),
				Index:   index{value: 150},
			},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			result, err := ParseTankLine([]byte(testcase.InputMessage))
			assert.NoError(t, err)
			assert.Equal(t, result, testcase.ExpectedMessage)
		})
	}
}

func TestIndexParsing(t *testing.T) {
	index, err := newIndex("0")
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), index.value)

	index, err = newIndex("end")
	assert.NoError(t, err)
	assert.Equal(t, uint64(endIndexUint), index.value)

	index, err = newIndex("3\n")
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), index.value)

	_, err = newIndex("foo")
	assert.Error(t, err)
}

func newMetric(name string, tags map[string]string, fields map[string]interface{}) telegraf.Metric {
	if tags == nil {
		tags = map[string]string{}
	}
	if fields == nil {
		fields = map[string]interface{}{}
	}
	m := metric.New(name, tags, fields, time.Date(1960, time.October, 25, 12, 0, 0, 0, time.UTC))
	return m
}

func TestPrecisionTimestamp(t *testing.T) {
	nanosecond := config.Duration(1 * time.Nanosecond)
	second := config.Duration(1 * time.Second)

	reasonableTimestamp, err := time.Parse(time.RFC3339Nano, "2023-01-01T10:15:23.578Z")
	if !assert.NoError(t, err) {
		return
	}
	reasonableTimestamp = reasonableTimestamp.UTC()

	reasonableSeconds := reasonableTimestamp.Unix()
	incorrectlyInterpreted := time.Unix(reasonableSeconds/(10e9), reasonableSeconds%(10e9)).UTC()

	testCases := []struct {
		Name                string
		ConfiguredPrecision *config.Duration
		ActualTimestamp     int64
		ExpectedTimestamp   time.Time
	}{
		{
			Name:                "Precision in Nanosecond",
			ConfiguredPrecision: &nanosecond,
			ActualTimestamp:     reasonableTimestamp.UnixNano(),
			ExpectedTimestamp:   reasonableTimestamp,
		},
		{
			Name:                "Precision in Nanosecond with unreasonable timestamp",
			ConfiguredPrecision: &nanosecond,
			ActualTimestamp:     incorrectlyInterpreted.UnixNano(),
			ExpectedTimestamp:   incorrectlyInterpreted,
		},
		{
			Name:                "Precision is Seconds",
			ConfiguredPrecision: &second,
			ActualTimestamp:     reasonableTimestamp.Unix(),
			ExpectedTimestamp:   reasonableTimestamp.Truncate(1 * time.Second),
		},
		{
			Name:                "Precision is Seconds with unreasonable timestamp",
			ConfiguredPrecision: &second,
			ActualTimestamp:     incorrectlyInterpreted.Unix(),
			ExpectedTimestamp:   incorrectlyInterpreted.Truncate(1 * time.Second),
		},
		{
			Name:                "Incorrectly Interpreted",
			ConfiguredPrecision: &nanosecond,
			ActualTimestamp:     reasonableTimestamp.Unix(),
			ExpectedTimestamp:   incorrectlyInterpreted,
		},
		{
			Name:              "Precision is empty with unreasonable timestamp",
			ActualTimestamp:   reasonableTimestamp.Unix(),
			ExpectedTimestamp: reasonableTimestamp.Truncate(1 * time.Second),
		},
		{
			Name:              "Precision is empty with reasonable timestamp",
			ActualTimestamp:   reasonableTimestamp.UnixNano(),
			ExpectedTimestamp: reasonableTimestamp,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			fmt.Println(testCase.Name)
			var acc testutil.Accumulator

			plugin := &T128Tank{
				Topic:     "test",
				Log:       testutil.Logger{},
				Precision: testCase.ConfiguredPrecision,
			}

			if !assert.NoError(t, plugin.Init()) {
				return
			}
			plugin.Start(&acc)

			metricHandler := influx.NewMetricHandler()
			metricParser := influx.NewParser(metricHandler)
			metricParser.ParseLine(fmt.Sprintf("test_metric value=10i %v", testCase.ActualTimestamp))
			metric, err := metricHandler.Metric()
			if !assert.NoError(t, err) {
				return
			}

			if plugin.adjustTime != nil {
				plugin.adjustTime(metric)
			}

			assert.Equal(t, testCase.ExpectedTimestamp, metric.Time().UTC())
		})
	}
}
