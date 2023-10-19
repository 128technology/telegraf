# 128T TANK Input Plugin

The tank input plugin collects data from a 128T.

## Configuration

```toml
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
## If from is "start", it will start consuming from the 
## first available message in the selected topic. 
## If it is "end", it will tail the topic for newly produced messages.
# from = "end"
```
