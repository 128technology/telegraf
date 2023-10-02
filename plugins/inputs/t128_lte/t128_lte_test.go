package t128_lte

import (
	"fmt"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

var CollectorTestCases = []struct {
	Name             string
	ConfigFile       string
	StateFile        string
	InfoFile         string
	LocalInitFile    string
	GlobalInitFile   string
	LTEPathDir       string
	InitError        bool
	ExpectedMetrics  []*testutil.Metric
	ExpectedErrors   []string
	ExpectedRequests []int
}{
	{
		Name:           "simple lte metric success",
		ConfigFile:     "./testdata/success/config.json",
		LocalInitFile:  "./testdata/success/local.init",
		GlobalInitFile: "./testdata/success/global.init",
		LTEPathDir:     "./testdata/success/",
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "128T-lte-collector",
				Tags: map[string]string{
					"device-interface": "LTE",
				},
				Fields: map[string]interface{}{
					"signal-strength":   "good",
					"carrier":           "AT&T",
					"connection-status": "connected",
					"active-band-class": "eutran-12",
					"apn":               "broadband",
					"service-mode":      "lte",
					"signal-value":      "-74",
					"rsrq-signal-value": "-15",
					"rsrp-signal-value": "-108",
					"service-status":    "normal",
					"imsi":              "310410221068243",
					"imei":              "359073062437649",
					"iccid":             "89014104272210682436",
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:           "simple lte metric with one signal",
		ConfigFile:     "./testdata/success/config-with-one-signal.json",
		LocalInitFile:  "./testdata/success/local.init",
		GlobalInitFile: "./testdata/success/global.init",
		LTEPathDir:     "./testdata/success/",
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "128T-lte-collector",
				Tags: map[string]string{
					"device-interface": "LTE",
				},
				Fields: map[string]interface{}{
					"signal-strength":   "good",
					"carrier":           "AT&T",
					"connection-status": "connected",
					"active-band-class": "eutran-12",
					"apn":               "broadband",
					"service-mode":      "lte",
					"signal-value":      "-74",
					"service-status":    "normal",
					"imsi":              "310410221068243",
					"imei":              "359073062437649",
					"iccid":             "89014104272210682436",
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:           "simple lte metric with failure message",
		ConfigFile:     "./testdata/failure/config.json",
		LocalInitFile:  "./testdata/failure/local.init",
		GlobalInitFile: "./testdata/failure/global.init",
		LTEPathDir:     "./testdata/failure/",
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "128T-lte-collector",
				Tags: map[string]string{
					"device-interface": "LTE",
				},
				Fields: map[string]interface{}{
					"signal-strength":   "excellent",
					"carrier":           "AT&T",
					"connection-status": "connected",
					"active-band-class": "eutran-12",
					"apn":               "broadband",
					"service-mode":      "lte",
					"signal-value":      "-51",
					"rsrq-signal-value": "-14",
					"snr-signal-value":  "13.0",
					"rsrp-signal-value": "-76",
					"service-status":    "Authentication Failure",
					"imsi":              "310410221068243",
					"imei":              "359073062437649",
					"iccid":             "89014104272210682436",
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:           "simple lte metric with unknown failure message",
		ConfigFile:     "./testdata/failure/config-unknown-status.json",
		LocalInitFile:  "./testdata/failure/local.init",
		GlobalInitFile: "./testdata/failure/global.init",
		LTEPathDir:     "./testdata/failure/",
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "128T-lte-collector",
				Tags: map[string]string{
					"device-interface": "LTE",
				},
				Fields: map[string]interface{}{
					"signal-strength":   "excellent",
					"carrier":           "AT&T",
					"connection-status": "connected",
					"active-band-class": "eutran-12",
					"apn":               "broadband",
					"service-mode":      "lte",
					"signal-value":      "-51",
					"rsrq-signal-value": "-14",
					"snr-signal-value":  "13.0",
					"rsrp-signal-value": "-76",
					"service-status":    "unknown-status",
					"imsi":              "310410221068243",
					"imei":              "359073062437649",
					"iccid":             "89014104272210682436",
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:           "lte metric with multiple interface",
		ConfigFile:     "./testdata/multipleInterface/config.json",
		LocalInitFile:  "./testdata/multipleInterface/local.init",
		GlobalInitFile: "./testdata/multipleInterface/global.init",
		LTEPathDir:     "./testdata/multipleInterface/",
		ExpectedMetrics: []*testutil.Metric{
			&testutil.Metric{
				Measurement: "128T-lte-collector",
				Tags: map[string]string{
					"device-interface": "LTE",
				},
				Fields: map[string]interface{}{
					"signal-strength":   "good",
					"carrier":           "Verizon",
					"connection-status": "connected",
					"active-band-class": "eutran-2",
					"apn":               "vzwinternet",
					"service-mode":      "lte",
					"signal-value":      "-72",
					"rsrq-signal-value": "-20",
					"rsrp-signal-value": "-113",
					"service-status":    "normal",
					"imsi":              "311480634897792",
					"imei":              "359073066267323",
					"iccid":             "89148000006605307956",
				},
			},
			&testutil.Metric{
				Measurement: "128T-lte-collector",
				Tags: map[string]string{
					"device-interface": "lte-int-2",
				},
				Fields: map[string]interface{}{
					"signal-strength":   "excellent",
					"carrier":           "AT&T",
					"connection-status": "connected",
					"active-band-class": "eutran-12",
					"apn":               "broadband",
					"service-mode":      "lte",
					"signal-value":      "-74",
					"rsrq-signal-value": "-15",
					"rsrp-signal-value": "-108",
					"service-status":    "normal",
					"imsi":              "310410221068243",
					"imei":              "359073062437649",
					"iccid":             "89014104272210682436",
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:           "simple HA lte metric success",
		ConfigFile:     "./testdata/ha_testcase/config.json",
		LocalInitFile:  "./testdata/ha_testcase/local.init",
		GlobalInitFile: "./testdata/ha_testcase/global.init",
		LTEPathDir:     "./testdata/ha_testcase/",
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "128T-lte-collector",
				Tags: map[string]string{
					"device-interface": "LTE",
				},
				Fields: map[string]interface{}{
					"signal-strength":   "good",
					"carrier":           "AT&T",
					"connection-status": "connected",
					"active-band-class": "eutran-12",
					"apn":               "broadband",
					"service-mode":      "lte",
					"signal-value":      "-74",
					"rsrq-signal-value": "-15",
					"rsrp-signal-value": "-108",
					"service-status":    "normal",
					"imsi":              "310410221068243",
					"imei":              "359073062437649",
					"iccid":             "89014104272210682436",
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
}

func TestT128LteCollector(t *testing.T) {
	for _, testCase := range CollectorTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			fmt.Println(testCase.Name)
			plugin := &T128Lte{
				CollectorName:  "128T-lte-collector",
				t128ConfigFile: testCase.ConfigFile,
				ltePathDir:     testCase.LTEPathDir,
				globalInitPath: testCase.GlobalInitFile,
				localInitPath:  testCase.LocalInitFile,
			}

			var acc testutil.Accumulator

			if testCase.InitError {
				require.Error(t, plugin.Init())
				return
			} else {
				require.NoError(t, plugin.Init())
			}

			for _, expectedRequests := range testCase.ExpectedRequests {
				plugin.Gather(&acc)
				fmt.Println(expectedRequests)
				// Timestamps aren't important, but need to match
				for _, m := range acc.Metrics {
					m.Time = time.Time{}
				}

				// Avoid specifying this unused type for each field
				for _, m := range testCase.ExpectedMetrics {
					m.Type = telegraf.Untyped
				}
			}

			var errorStrings []string = nil
			for _, err := range acc.Errors {
				errorStrings = append(errorStrings, err.Error())
			}

			require.ElementsMatch(t, testCase.ExpectedErrors, errorStrings)
			require.ElementsMatch(t, testCase.ExpectedMetrics, acc.Metrics)
		})
	}
}
