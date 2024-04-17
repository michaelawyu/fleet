/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package scheduler

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// The scheduler related metrics.
var (
	// SchedulingCycleDurationMilliseconds is a Fleet scheduler metric that tracks how long it
	// takes to complete a scheduling loop run.
	schedulingCycleDurationMilliseconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "scheduling_cycle_duration_milliseconds",
			Help: "The duration of a scheduling cycle run in milliseconds",
			Buckets: []float64{
				10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000,
			},
		},
		[]string{
			"is_failed",
			"needs_requeue",
		},
	)

	// SchedulerActiveWorkers is a prometheus metric which holds the number of active scheduler loop.
	schedulerActiveWorkers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scheduling_active_workers",
		Help: "Number of currently running scheduling loop",
	}, []string{})
)

func init() {
	metrics.Registry.MustRegister(schedulingCycleDurationMilliseconds, schedulerActiveWorkers)
}
