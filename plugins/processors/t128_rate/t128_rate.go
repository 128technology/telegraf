package rename

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/plugins/processors"
)

const sampleConfig = `
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
	# "/inline/replace" = "/inline/replace"
`

type T128Rate struct {
	Fields         map[string]string `toml:"fields"`
	Expiration     internal.Duration `toml:"expiration"`
	RemoveOriginal bool              `toml:"remove-original"`

	Log telegraf.Logger `toml:"-"`

	targetFields map[string]target
	cache        map[uint64]map[string]observedValue
}

type target struct {
	key           string
	matchesSource bool
}

type observedValue struct {
	value   float64
	expires time.Time
}

func (r *T128Rate) SampleConfig() string {
	return sampleConfig
}

func (r *T128Rate) Description() string {
	return "Compute the rate of fields that pass through this filter."
}

func (r *T128Rate) Apply(in ...telegraf.Metric) []telegraf.Metric {
	for _, point := range in {
		seriesHash := point.HashID()

		removeFields := make([]string, 0)

		for _, field := range point.FieldList() {
			target, ok := r.targetFields[field.Key]
			if !ok {
				continue
			}

			currentValue, converted := convert(field.Value)
			if !converted {
				r.Log.Warnf("Failed to convert field '%s' to float for a rate calculation. The rate computation will be skipped.")
				continue
			}

			cacheFields, metricIsCached := r.cache[seriesHash]
			if !metricIsCached {
				r.cache[seriesHash] = make(map[string]observedValue, 0)
			}

			rateAdded := false
			if observed, ok := cacheFields[field.Key]; ok {
				if point.Time().Before(observed.expires) {
					point.AddField(target.key, currentValue-observed.value)
					rateAdded = true
				}
			}

			if (target.matchesSource && !rateAdded) || (!target.matchesSource && r.RemoveOriginal) {
				removeFields = append(removeFields, field.Key)
			}

			r.cache[seriesHash][field.Key] = observedValue{
				value:   currentValue,
				expires: point.Time().Add(r.Expiration.Duration),
			}
		}

		for _, fieldKey := range removeFields {
			point.RemoveField(fieldKey)
		}
	}

	return in
}

func (r *T128Rate) Init() error {
	if len(r.Fields) == 0 {
		return fmt.Errorf("at least one value must be specified in the 'fields' list")
	}

	for dest, src := range r.Fields {
		if target, ok := r.targetFields[src]; ok {
			// For simple testing
			conflicting := []string{dest, target.key}
			sort.Strings(conflicting)

			return fmt.Errorf("both '%s' and '%s' are configured to be calculated from '%s'", conflicting[0], conflicting[1], src)
		}

		r.targetFields[src] = target{
			key:           dest,
			matchesSource: src == dest,
		}
	}

	if r.Expiration.Duration == 0 {
		// No expiration means never expire, so set to maximum duration
		r.Expiration.Duration = math.MaxInt64
	} else {
		// If the time difference matches, don't expire. Adjusting here makes
		// later math easier.
		r.Expiration.Duration++
	}

	return nil
}

func newRate() *T128Rate {
	return &T128Rate{
		targetFields: make(map[string]target),
		cache:        make(map[uint64]map[string]observedValue),
	}
}

func init() {
	processors.Add("t128_rate", func() telegraf.Processor {
		return newRate()
	})
}

func convert(in interface{}) (float64, bool) {
	switch v := in.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}
