package metrics

import (
	"time"

	"github.com/gojek/turing/engines/router/missionctl/errors"
	"github.com/gojek/turing/engines/router/missionctl/log"
	"github.com/prometheus/client_golang/prometheus"
)

//////////////////////////// Metrics Definitions //////////////////////////////

const (
	// Namespace is the Prometheus Namespace in all metrics published by the turing app
	Namespace string = "mlp"
	// Subsystem is the Prometheus Subsystem in all metrics published by the turing app
	Subsystem string = "turing"
)

// PrometheusHistogramVec is an interface that captures the methods from the the Prometheus
// HistogramVec type that are used in the app. This is added for unit testing.
type PrometheusHistogramVec interface {
	GetMetricWith(prometheus.Labels) (prometheus.Observer, error)
}

// histogramMap maintains a mapping between the metric name and the corresponding
// histogram vector
var histogramMap map[MetricName]*prometheus.HistogramVec = map[MetricName]*prometheus.HistogramVec{
	ExperimentEngineRequestMs: prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      string(ExperimentEngineRequestMs),
		Help:      "Histogram for the runtime (in milliseconds) of Litmus requests.",
		Buckets:   prometheus.LinearBuckets(2, 2, 20),
	},
		[]string{"status"},
	),
	RouteRequestDurationMs: prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      string(RouteRequestDurationMs),
		Help:      "Histogram for the runtime (in milliseconds) of Fiber route requests.",
		Buckets:   prometheus.LinearBuckets(5, 5, 25),
	},
		[]string{"status", "route"},
	),
	TuringComponentRequestDurationMs: prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      string(TuringComponentRequestDurationMs),
		Help:      "Histogram for time spent (in milliseconds) at each Turing component.",
		Buckets:   append([]float64{0.5, 1, 2, 3, 4}, prometheus.LinearBuckets(5, 5, 25)...),
	},
		[]string{"status", "component"},
	),
}

// getHistogramVec is a getter for the prometheus.HistogramVec defined for the input key.
// It returns a value satisfying the PrometheusHistogramVec interface
func getHistogramVec(key MetricName) (PrometheusHistogramVec, error) {
	histVec, ok := histogramMap[key]
	if !ok {
		return nil, errors.Newf(errors.NotFound, "Could not find the metric for %s", key)
	}
	return histVec, nil
}

//////////////////////////// PrometheusClient /////////////////////////////////

// PrometheusClient satisfies the Collector interface
type PrometheusClient struct {
}

// InitMetrics initializes the collectors for all metrics defined for the app
// and registers them with the DefaultRegisterer.
func (PrometheusClient) InitMetrics() {
	// Register histograms
	for _, obs := range histogramMap {
		prometheus.MustRegister(obs)
	}
}

// MeasureDurationMsSince takes in the Metric name, the start time and a map of labels and values
// to be associated to the metric. If errors occur in accessing the metric or associating the
// labels, they will simply be logged.
func (PrometheusClient) MeasureDurationMsSince(
	key MetricName,
	starttime time.Time,
	labels map[string]string,
) {
	// Get the histogram vec defined for the input key
	histVec, err := getHistogramVec(key)
	if err != nil {
		log.Glob().Errorf(err.Error())
		return
	}
	// Create a histogram with the labels
	s, err := histVec.GetMetricWith(labels)
	if err != nil {
		log.Glob().Errorf("Error occurred when creating histogram for %s: %v", key, err)
		return
	}
	// Record the value in milliseconds
	s.Observe(float64(time.Since(starttime) / time.Millisecond))
}

// MeasureDurationMs takes in the Metric name and a map of labels and functions to obtain
// the label values - this allows for MeasureDurationMs to be deferred and do a delayed
// evaluation of the labels. It returns a function which, when executed, will log the
// duration in ms since MeasureDurationMs was called. If errors occur in accessing the metric or
// associating the labels, they will simply be logged.
func (p PrometheusClient) MeasureDurationMs(
	key MetricName,
	labelValueGetters map[string]func() string,
) func() {
	// Capture start time
	starttime := time.Now()
	// Return function to measure and log the duration since start time
	return func() {
		// Evaluate the labels
		labels := map[string]string{}
		for key, f := range labelValueGetters {
			labels[key] = f()
		}
		// Log measurement
		p.MeasureDurationMsSince(key, starttime, labels)
	}
}
