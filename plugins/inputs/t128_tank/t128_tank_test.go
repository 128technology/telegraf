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

			var acc testutil.Accumulator
			var receivedMessages []IndexedMessage
			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			indexFileName := strings.ReplaceAll(testcase.Name, " ", "_")
			plugin.IndexFile = path.Join(t.TempDir(), fmt.Sprintf("%s.index", indexFileName))
			if testcase.IndexFileContent != "" {
				err := os.WriteFile(plugin.IndexFile, []byte(testcase.IndexFileContent), 0755)
				assert.NoError(t, err)
			}
			reader := NewReader(plugin.ServerAddress, plugin.PortNumber, plugin.Topic, plugin.IndexFile, testcase.DefaultIndex, testutil.Logger{}, &acc).withTankReadCommandContext(testcase.TankReadCommandContext)
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

			var acc testutil.Accumulator
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			indexFileName := strings.ReplaceAll(testcase.Name, " ", "_")
			plugin.IndexFile = path.Join(t.TempDir(), fmt.Sprintf("%s.index", indexFileName))
			if testcase.IndexFileContent != "" {
				err := os.WriteFile(plugin.IndexFile, []byte(testcase.IndexFileContent), 0755)
				assert.NoError(t, err)
			}
			reader := NewReader(plugin.ServerAddress, plugin.PortNumber, plugin.Topic, plugin.IndexFile, testcase.DefaultIndex, testutil.Logger{}, &acc).withTankReadCommandContext(testcase.TankReadCommandContext)
			var receivedErrorMessage string
			go func() {
				err := reader.readFromTank(ctx, StartIndex)
				if err != nil {
					receivedErrorMessage = err.Error()
				}
			}()

			time.Sleep(1 * time.Second)

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
			Name:         "input-message-with-newline",
			InputMessage: "seq=8:type=LINK_UP, Timestamp=2022-03-25T00:00:00Z,Raw=hello\nworld",
			ExpectedMessage: &IndexedMessage{
				Message: []byte("type=LINK_UP, Timestamp=2022-03-25T00:00:00Z,Raw=hello\nworld"),
				Index:   index{value: 8},
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
