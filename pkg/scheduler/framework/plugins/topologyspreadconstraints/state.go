/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package topologyspreadconstraints

import (
	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

type domainName string

type bindingCounterByDomain struct {
	// counter tracks the number of bindings in each domain.
	counter map[domainName]int

	// smallest is the smallest count in the counter.
	smallest int
	// secondSmallest is the second smallest count in the counter.
	//
	// Note that the second smallest count might be the same as the smallest count.
	secondSmallest int
	// largest is the largest count in the counter.
	//
	// Note that the largest count might be the same as the second smallest (and consequently
	// the smallest) count.
	largest int
}

type clusterName string
type violationReasons []string
type doNotScheduleViolations map[clusterName]violationReasons
type topologySpreadScores map[clusterName]int

type topologySpreadConstraintsPluginState struct {
	// doNotScheduleConstraints is a list of topology spread constraints with a DoNotSchedule
	// requirement.
	doNotScheduleConstraints []*fleetv1beta1.TopologySpreadConstraint

	// scheduleAnywayConstraints is a list of topology spread constraints with a ScheduleAnyway
	// requirement.
	scheduleAnywayConstraints []*fleetv1beta1.TopologySpreadConstraint

	// violations
	violations doNotScheduleViolations

	// scores
	scores topologySpreadScores
}
