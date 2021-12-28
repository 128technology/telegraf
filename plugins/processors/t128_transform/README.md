# T128 Transform Processor Plugin

The `t128_transform` transforms metrics based on the difference between two observed points.

### Configuration:

```toml
[[processors.t128_transform]]
  ## For 'rate' and 'diff', if more than this amount of time passes between
  ## data points, the previous value will be considered old and the value will
  ## be recalculated as if it hadn't been seen before. A zero expiration means
  ## never expire.
  ##
  ## When using the 'state-change' transform, an update metric will be sent
  ## upon expiration even if the value has not changed.
  # expiration = "0s"

  ## The operation that should be performed between two observed points.
  ## It can be 'diff', 'rate', or 'state-change'.
  # transform = "rate"

  ## For the fields who's key/value pairs don't match, should the original
  ## field be removed?
  # remove-original = true

  ## Specify a field to be populated with the last produced value. If the
  ## field name is an empty string or there is no prior value, the field will
  ## be excluded.
  # previous_field = ""

  ## Specify a path to persist state across telegraf instance restarts.
  ## Only applicable for "state-change" transforms.
  ## A default of "" indicates that state will not be persisted.
  # persist_to = ""

[processors.t128_transform.fields]
  ## Replace fields with their computed values, renaming them if indicated
  # "/rate/metric" = "/total/metric"
  # "/inline/replace" = "/inline/replace"

[processors.t128_transform.previous_fields]
  ## Populate these fields with the previous transformed value. If there is no
  ## prior value, the field will be excluded.
  # "/rate/metric/previous" = "/rate/metric"
```

### Example Diff:

```toml
[[processors.t128_transform]]
  transform = "diff"
  remove-original = true
[processors.t128_transform.fields]
  diff = "total"
```

```diff
- measurement total=10i 1612214805000000000
- measurement total=15i 1612214810000000000
+ measurement diff=5i 1612214810000000000
```

### Example Rate:

```toml
[[processors.t128_transform]]
  transform = "rate"
  remove-original = true
[processors.t128_transform.fields]
  rate = "total"
```

```diff
- measurement total=10i 1612214805000000000
- measurement total=15i 1612214810000000000
+ measurement rate=1i 1612214810000000000
```
