# 128T LTE Input Plugin

The LTE collector input when run will scan the current node configuration for any SSR supported and configured LTE devices. This collector can be used for pushing data such as signal-strength, carrier information etc to the monitoring stack.

## Configuration

```toml
[[inputs.t128_lte]]
## Required. A name for the collector which will be used as the measurement name of the produced data.
# collector_name = "lte-state"
```
