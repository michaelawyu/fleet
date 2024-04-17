/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package framework

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	schedulingCyclePerStageDurationMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "scheduling_cycle_per_stage_duration_milliseconds",
			Help: "The duration of each stage of a scheduling cycle run in milliseconds",
			Buckets: []float64{
				10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000,
			},
		},
		[]string{
			"stage",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(schedulingCyclePerStageDurationMilliseconds)
}

func observeSchedulingCyclePerStageDurationMetrics(stage string, duration float64) {
	schedulingCyclePerStageDurationMilliseconds.WithLabelValues(stage).Observe(duration)
}
