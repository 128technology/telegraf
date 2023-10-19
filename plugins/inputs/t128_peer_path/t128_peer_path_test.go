package t128_peer_path

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

type Endpoint struct {
	URL             string
	Code            int
	ExpectedRequest string
	Response        string
}

const (
	ValidPeerPathRequestWithHostName    = `{"query":"query {\nallPeers{\nnodes{\npaths{\nadjacentAddress\nadjacentHostname\ndeviceInterface\nenabled\nnetworkInterface\nnode\nstatus\nvlan}\nrouterName}}}"}`
	ValidPeerPathRequestWithoutHostName = `{"query":"query {\nallPeers{\nnodes{\npaths{\nadjacentAddress\ndeviceInterface\nenabled\nnetworkInterface\nnode\nstatus\nvlan}\nrouterName}}}"}`
)

var CollectorTestCases = []struct {
	Name             string
	InitError        bool
	Endpoint         Endpoint
	ExpectedMetrics  []*testutil.Metric
	ExpectedErrors   []string
	ExcludeHostname  bool
	ExpectedRequests []int
}{
	{
		Name:            "empty adjacent hostname",
		ExcludeHostname: false,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidPeerPathRequestWithHostName, `{
				"data": {
				  "allPeers": {
					"nodes": [
					  {
						"routerName": "NorthEast",
						"paths": [
						  {
							"node": "node1",
							"adjacentAddress": "10.10.10.10",
							"adjacentHostname": null,
							"status": "UP",
							"enabled": true,
							"deviceInterface": "wan5",
							"networkInterface": "wan5",
							"vlan": "0"
						  }
						]
					  }
					]
				  }
				}
			}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags: map[string]string{
					"adjacentAddress":  "10.10.10.10",
					"deviceInterface":  "wan5",
					"networkInterface": "wan5",
					"vlan":             "0",
					"peerRouter":       "NorthEast",
					"peer-path":        "NorthEast/10.10.10.10/node1/wan5/0",
				},
				Fields: map[string]interface{}{
					"status":  "UP",
					"enabled": true,
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:            "exclude hostname set to true",
		ExcludeHostname: true,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidPeerPathRequestWithoutHostName, `{
				"data": {
				  "allPeers": {
					"nodes": [
					  {
						"routerName": "NorthEast",
						"paths": [
						  {
							"node": "node1",
							"adjacentAddress": "10.10.10.10",
							"adjacentHostname": "fake-peer",
							"status": "UP",
							"enabled": true,
							"deviceInterface": "wan5",
							"networkInterface": "wan5",
							"vlan": "0"
						  }
						]
					  }
					]
				  }
				}
			}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags: map[string]string{
					"adjacentAddress":  "10.10.10.10",
					"deviceInterface":  "wan5",
					"networkInterface": "wan5",
					"vlan":             "0",
					"peerRouter":       "NorthEast",
					"peer-path":        "NorthEast/10.10.10.10/node1/wan5/0",
				},
				Fields: map[string]interface{}{
					"status":  "UP",
					"enabled": true,
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:            "include everything",
		ExcludeHostname: false,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidPeerPathRequestWithHostName, `{
				"data": {
				  "allPeers": {
					"nodes": [
					  {
						"routerName": "NorthEast",
						"paths": [
						  {
							"node": "node1",
							"adjacentAddress": "10.10.10.10",
							"adjacentHostname": "fake-peer",
							"status": "UP",
							"enabled": true,
							"deviceInterface": "wan5",
							"networkInterface": "wan5",
							"vlan": "0"
						  }
						]
					  }
					]
				  }
				}
			}`},
		ExpectedMetrics: []*testutil.Metric{
			{
				Measurement: "test-collector",
				Tags: map[string]string{
					"adjacentAddress":  "10.10.10.10",
					"adjacentHostname": "fake-peer",
					"deviceInterface":  "wan5",
					"networkInterface": "wan5",
					"vlan":             "0",
					"peerRouter":       "NorthEast",
					"peer-path":        "NorthEast/10.10.10.10/fake-peer/node1/wan5/0",
				},
				Fields: map[string]interface{}{
					"status":  "UP",
					"enabled": true,
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
	{
		Name:            "multiple paths",
		ExcludeHostname: false,
		Endpoint: Endpoint{"/api/v1/graphql/", 200, ValidPeerPathRequestWithHostName, `{
					"data": {
					  "allPeers": {
						"nodes": [
						  {
							"routerName": "NorthEast",
							"paths": [
							  {
								"node": "node1",
								"adjacentAddress": "172.16.3.2",
								"adjacentHostname": null,
								"status": "UP",
								"enabled": true,
								"deviceInterface": "wan5",
								"networkInterface": "wan5",
								"vlan": "0"
							  },
							  {
								"node": "node1",
								"adjacentAddress": "127.117.97.105",
								"adjacentHostname": "fake-peer",
								"status": "DOWN",
								"enabled": true,
								"deviceInterface": "wan5",
								"networkInterface": "wan5",
								"vlan": "0"
							  }
							]
						  }
						]
					  }
					}
				}`},
		ExpectedMetrics: []*testutil.Metric{
			&testutil.Metric{
				Measurement: "test-collector",
				Tags: map[string]string{
					"adjacentAddress":  "172.16.3.2",
					"deviceInterface":  "wan5",
					"networkInterface": "wan5",
					"vlan":             "0",
					"peerRouter":       "NorthEast",
					"peer-path":        "NorthEast/172.16.3.2/node1/wan5/0",
				},
				Fields: map[string]interface{}{
					"status":  "UP",
					"enabled": true,
				},
			},
			&testutil.Metric{
				Measurement: "test-collector",
				Tags: map[string]string{
					"adjacentHostname": "fake-peer",
					"deviceInterface":  "wan5",
					"networkInterface": "wan5",
					"vlan":             "0",
					"peerRouter":       "NorthEast",
					"peer-path":        "NorthEast/fake-peer/node1/wan5/0",
				},
				Fields: map[string]interface{}{
					"status":  "DOWN",
					"enabled": true,
				},
			},
		},
		ExpectedErrors:   []string{},
		ExpectedRequests: []int{1},
	},
}

func TestT128PeerPathCollector(t *testing.T) {
	for _, testCase := range CollectorTestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			fakeServer, requestCount := createTestServer(t, testCase.Endpoint)
			defer fakeServer.Close()

			plugin := &T128PeerPath{
				CollectorName:   "test-collector",
				BaseURL:         fakeServer.URL + "/api/v1/graphql",
				ExcludeHostname: testCase.ExcludeHostname,
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
				require.Equal(t, expectedRequests, *requestCount)

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

func createTestServer(t *testing.T, endpoint Endpoint) (*httptest.Server, *int) {
	requestCount := 0
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		requestCount += 1

		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NotEqual(t, r.Header.Get("deadline"), "")
		require.Equal(t, "POST", r.Method)

		if endpoint.URL != r.URL.Path {
			fmt.Printf("There isn't an endpoint for: %v\n", r.URL.Path)
			w.WriteHeader(404)
			return
		}

		if endpoint.ExpectedRequest != "" {
			contents, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(500)
				return
			}

			require.JSONEq(t, endpoint.ExpectedRequest, string(contents), "Unexpected request body for endpoint %s", endpoint.URL)
		}

		w.WriteHeader(endpoint.Code)
		w.Write([]byte(endpoint.Response))
	}))

	return fakeServer, &requestCount
}
