package t128_filter

import (
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"
)

const sampleConfig = `
[[processors.t128_filter]]
  ## The conditions that must be met to pass a metric through. This is similar
  ## behavior to a tagpass, but the multiple tags are ANDed
  [[processors.t128_filter.condition]]

  [processors.t128_filter.condition.tags]
     #tag1 = ["value1", "value2"]
	 #tag2 = ["value3"]
`

type tags map[string][]string

type Condition struct {
	Tags tags `toml:"tags"`
}

type T128Filter struct {
	Conditions []Condition `toml:"condition"`

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
		if doAllConditionsMatch(r.Conditions, point) {
			filteredPoints = append(filteredPoints, point)
		}
	}

	return filteredPoints
}

func doAllConditionsMatch(conditions []Condition, point telegraf.Metric) bool {
	conditionMatches := make([]bool, len(conditions))
	for i, condition := range conditions {
		conditionMatches[i] = doTagsMatch(condition.Tags, point)
	}
	return and(conditionMatches)
}

func doTagsMatch(tags tags, point telegraf.Metric) bool {
	keyMatches := make([]bool, len(tags))
	i := 0
	for key, acceptableValues := range tags {
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
		Conditions: make([]Condition, 0),
	}
}

func init() {
	processors.Add("t128_filter", func() telegraf.Processor {
		return newFilter()
	})
}
