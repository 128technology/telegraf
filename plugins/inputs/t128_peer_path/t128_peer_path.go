package t128_peer_path

import (
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/t128_graphql"
)

const (
	//DefaultRequestTimeout is the request timeout if none is configured
	DefaultRequestTimeout = 5 * time.Second

	//timeoutDeadlineDiff is the difference between plugin.Timeout and the request deadline header
	timeoutDeadlineDiff = 1 * time.Second

	peerPathQuery = `query {
		allPeers{
			nodes{
				paths{
					adjacentAddress
					deviceInterface
					enabled
					networkInterface
					node
					status
					vlan
				}
				routerName
			}
		}
	}`

	peerPathHostnameQuery = `query {
		allPeers{
			nodes{
				paths{
					adjacentAddress
					adjacentHostname
					deviceInterface
					enabled
					networkInterface
					node
					status
					vlan
				}
				routerName
			}
		}
	}`
)

// T128PeerPath is an input for metrics of a 128T router instance
type T128PeerPath struct {
	CollectorName    string          `toml:"collector_name"`
	BaseURL          string          `toml:"base_url"`
	UnixSocket       string          `toml:"unix_socket"`
	Timeout          config.Duration `toml:"timeout"`
	RetryIfNotFound  bool            `toml:"retry_if_not_found"`
	ExcludeHostname  bool            `toml:"exclude_hostname"`
	gqlCollector     *t128_graphql.T128GraphQL
	endpointNotFound bool
}

var peerPathEntryPoint = "allPeers/nodes/paths"

var peerPathFields = map[string]string{"status": "status", "enabled": "enabled"}

var peerPathTags = map[string]string{
	"node":             "node",
	"adjacentAddress":  "adjacentAddress",
	"deviceInterface":  "deviceInterface",
	"networkInterface": "networkInterface",
	"vlan":             "vlan",
	"routerName":       "allPeers/nodes/routerName",
}

var peerPathHostnameTags = map[string]string{
	"node":             "node",
	"adjacentAddress":  "adjacentAddress",
	"adjacentHostname": "adjacentHostname",
	"deviceInterface":  "deviceInterface",
	"networkInterface": "networkInterface",
	"vlan":             "vlan",
	"routerName":       "allPeers/nodes/routerName",
}

var sampleConfig = `
[[inputs.t128_peer_path]]
## Required. A name for the collector which will be used as the measurement name of the produced data.
# collector_name = "peer-path-state"

## Required. The base url for data collection.
## graphQL ports vary across 128T versions.
# base_url = "http://localhost:31517/api/v1/graphql/"

## A socket to use for retrieving data - unused by default
# unix_socket = "/var/run/128technology/web-server.sock"

## Amount of time allowed before the client cancels the HTTP request
# timeout = "5s"

## Required. The versioned field to include hostname or not
# exclude_hostname = false
`

// SampleConfig returns the default configuration of the Input
func (*T128PeerPath) SampleConfig() string {
	return sampleConfig
}

// Description returns a one-sentence description on the Input
func (*T128PeerPath) Description() string {
	return "Make a 128T Peer Path query and return the data"
}

// Init sets up the input to be ready for action
func (plugin *T128PeerPath) Init() error {

	var query string
	var tags map[string]string
	if plugin.ExcludeHostname {
		query = peerPathQuery
		tags = peerPathTags
	} else {
		query = peerPathHostnameQuery
		tags = peerPathHostnameTags
	}
	plugin.gqlCollector = &t128_graphql.T128GraphQL{
		CollectorName:   plugin.CollectorName,
		BaseURL:         plugin.BaseURL,
		UnixSocket:      plugin.UnixSocket,
		EntryPoint:      peerPathEntryPoint,
		Fields:          peerPathFields,
		Tags:            tags,
		Timeout:         plugin.Timeout,
		RetryIfNotFound: plugin.RetryIfNotFound,
		Query:           query,
	}
	err := plugin.gqlCollector.Init()
	if err != nil {
		return err
	}
	return nil
}

// Gather takes in an accumulator and adds the metrics that the Input gathers
func (plugin *T128PeerPath) Gather(acc telegraf.Accumulator) error {
	if !plugin.RetryIfNotFound && plugin.endpointNotFound {
		return nil
	}
	processedResponses, err := plugin.gqlCollector.MakeRequest()
	if err != nil {
		for _, err := range err {
			acc.AddError(err)
		}
	}
	for _, processedResponse := range processedResponses {
		for k, v := range processedResponse.Tags {
			if k == "adjacentAddress" && v == "127.117.97.105" {
				delete(processedResponse.Tags, "adjacentAddress")
				continue
			}
			if k == "adjacentHostname" && v == "" {
				delete(processedResponse.Tags, "adjacentHostname")
				continue
			}
		}
		acc.AddFields(
			plugin.CollectorName,
			processedResponse.Fields,
			processedResponse.Tags,
		)
	}

	return nil
}

func init() {
	inputs.Add("t128_peer_path", func() telegraf.Input {
		return &T128PeerPath{
			Timeout: config.Duration(DefaultRequestTimeout),
		}
	})
}
