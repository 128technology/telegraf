package t128_tank

import (
	"testing"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/assert"
)

func TestT128Tank(t *testing.T) {
	plugin := &T128Tank{
		Topic: "events",
	}
	var acc testutil.Accumulator
	err := plugin.Start(&acc)
	assert.NoError(t, err)
	plugin.Stop()

}

func TestParseTankLine(t *testing.T) {
	inputMessage := "seq=150:measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775"
	expectedMessage := &IndexedMessage{
		Message: []byte("measurement,core=2,node=test-1,port=corp-dmz-p value=0i 1586886775"),
		Index:   index{value: 150},
	}
	result, err := ParseTankLine([]byte(inputMessage))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}
	if result.Index.value != expectedMessage.Index.value || string(result.Message) != string(expectedMessage.Message) {
		t.Errorf("ParseTankLine(%s) returned unexpected result: %+v, expected: %+v", inputMessage, result, expectedMessage)
	}
}
