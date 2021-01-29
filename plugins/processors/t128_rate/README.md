# T128 Rate Processor Plugin

The `t128_rate` processor computes the point to point difference of a field.

### Configuration:

```toml
[[processors.t128_rate]]
  ## If more than this amount of time passes between data points, the
  ## previous value will be considered old and the rate will be recalculated
  ## as if it hadn't been seen before. A zero expiration means never expire.
  # expiration = "0s"

  ## For the fields who's key/value pairs don't match, should the original
  ## field be removed?
  # remove-original = true

[processors.t128_rate.fields]
  ## Replace fields with their rates, renaming them if indicated
  # "/rate/metric" = "/total/metric"
  # "/inline/replace" = "/inline/replace"]
```

### Example processing:

```toml
[[processors.t128_rate]]
[processors.t128_rate.fields]
  rate = total
```

```diff
- measurement total=10i 1502489900000000000
- measurement total=15i 1502490000000000000
+ measurement rate=5i 1502490000000000000
```
