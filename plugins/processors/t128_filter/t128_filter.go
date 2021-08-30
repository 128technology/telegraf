package t128_filter

import (
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"
)

const sampleConfig = `
[[processors.t128_filter]]
  ## The conditions that must be met to pass a metric through. This is similar
  ## behavior to a tagpass, but the multiple tags are ANDed
  [processors.t128_filter.conditions]
     #tag1 = ["value1", "value2"]
	 #tag2 = ["value3"]
`

type conditionSet map[string][]string

type T128Filter struct {
	Conditions []conditionSet `toml:"conditions"`

	Log telegraf.Logger `toml:"-"`
}

func (r *T128Filter) SampleConfig() string {
	return sampleConfig
}

func (r *T128Filter) Description() string {
	return "Filter metrics from being emitted."
}

func (r *T128Filter) Apply(in ...telegraf.Metric) []telegraf.Metric {
	filteredPoints := make([]telegraf.Metric, 0)

	for _, point := range in {
		if doAllConditionSetsMatch(r.Conditions, point) {
			filteredPoints = append(filteredPoints, point)
		}
	}

	return filteredPoints
}

func doAllConditionSetsMatch(conditionSets []conditionSet, point telegraf.Metric) bool {
	conditionSetMatches := make([]bool, len(conditionSets))
	for i, conditionSet := range conditionSets {
		conditionSetMatches[i] = doesConditionSetMatch(conditionSet, point)
	}
	return and(conditionSetMatches)
}

func doesConditionSetMatch(conditionSet conditionSet, point telegraf.Metric) bool {
	keyMatches := make([]bool, len(conditionSet))
	i := 0
	for key, acceptableValues := range conditionSet {
		keyMatches[i] = doesKeyMatch(key, acceptableValues, point)
		i++
	}

	return and(keyMatches)
}

func doesKeyMatch(key string, acceptableValues []string, point telegraf.Metric) bool {
	actualValue, wasSet := point.GetTag(key)

	return wasSet && or(applyBooleanFunc(acceptableValues, func(acceptableValue string) bool {
		return acceptableValue == actualValue
	}))
}

func applyBooleanFunc(items []string, application func(string) bool) []bool {
	bools := make([]bool, len(items))
	for i, item := range items {
		bools[i] = application(item)
	}
	return bools
}

func and(bools []bool) bool {
	for _, b := range bools {
		if !b {
			return false
		}
	}
	return true
}

func or(bools []bool) bool {
	for _, b := range bools {
		if b {
			return true
		}
	}
	return false
}

func (r *T128Filter) Init() error {
	return nil
}

func newFilter() *T128Filter {
	return &T128Filter{
		Conditions: make([]conditionSet, 0),
	}
}

func init() {
	processors.Add("t128_filter", func() telegraf.Processor {
		return newFilter()
	})
}
