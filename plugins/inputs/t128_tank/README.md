# 128T TANK Input Plugin

The tank input plugin collects data from a 128T.

## Configuration

```toml
[[inputs.t128_tank]]
## A name for the collector which will be used as the measurement name of the produced data.
# collector_name = "event_collector"

## Required. A path for index file.
# index_file = ""

## Required. Type of topic.
# topic = "events"

## Required. Port Number to get tank data from.
# port_number = 11011

## Required. Port Address to get tank data from.
# port_address = "127.0.0.1"
```
