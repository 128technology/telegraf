# 128T Peer Path Input Plugin

The peer path input plugin collects data from a 128T instance via graphQL.

## Configuration

```toml
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

## Versioned fields
# exclude_hostname = false
```
