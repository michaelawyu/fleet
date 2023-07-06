/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package topologyspreadconstraints

import (
	"fmt"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
	"go.goms.io/fleet/pkg/scheduler/framework"
)

// countByDomain counts the number of scheduled or bound bindings in each domain per a given
// topology key.
func countByDomain(clusters []fleetv1beta1.MemberCluster, state framework.CycleStatePluginReadWriter, topologyKey string) *bindingCounterByDomain {
	// Calculate the number of bindings in each domain.
	//
	// Note that all domains will have their corresponding counts, even if the counts are zero.
	counter := make(map[domainName]int)
	for _, cluster := range clusters {
		val, ok := cluster.Labels[topologyKey]
		if !ok {
			// The cluster under inspection does not have the topology key and thus is
			// not part of the spread.
			continue
		}

		count, ok := counter[domainName(val)]
		if !ok {
			// Initialize the count for the domain (even if there is no scheduled or bound
			// binding in the domain).
			counter[domainName(val)] = 0
		}

		if state.IsClusterScheduledOrBound(cluster.Name) {
			// The cluster under inspection owns a scheduled or bound binding.
			counter[domainName(val)] = count + 1
		}
	}

	// Prepare the special counts.
	//
	// Here the code loops through all counts to find the smallest, second smallest, and
	// largest values instead of performing a sort to save some overhead.

	// Initialize the special counts with a placeholder value.
	var smallest, secondSmallest, largest int = -1, -1, -1

	for _, c := range counter {
		switch {
		case smallest == -1:
			// The special counts are initialized with a placeholder value, which signals that
			// this is the first run of the the loop; set the special counts to actual values.
			smallest = c
			secondSmallest = c
			largest = c
		case c <= smallest:
			// A new smallest count appears.
			secondSmallest = smallest
			smallest = c
		case c <= secondSmallest:
			// A new second smallest count appears.
			secondSmallest = c
		case c > largest:
			// A new largest count appears.
			largest = c
		default:
			// Do nothing if the count is larger than the second smallest count but no greater
			// than the largest count.
		}
	}

	return &bindingCounterByDomain{
		counter:        counter,
		smallest:       smallest,
		secondSmallest: secondSmallest,
		largest:        largest,
	}
}

// willViolatereturns whether producing one more binding in a domain would lead
// to violations; it will also return the skew change caused by the provisional placement.
func willViolate(counter *bindingCounterByDomain, name domainName, maxSkew int) (violated bool, skewChange int, err error) {
	count, ok := counter.counter[name]
	if !ok {
		// The domain is not registered in the counter; normally this would never
		// happen as the state being evaluated is consistent and the counter tracks
		// all domains.
		return false, 0, fmt.Errorf("domain %s is not registered in the counter", name)
	}

	if count < counter.smallest || count > counter.largest {
		// Perform a sanity check here; normally this would never happen as the counter
		// tracks all domains.
		return false, 0, fmt.Errorf("the counter has invalid special counts: [%d, %d], received %d", counter.smallest, counter.largest, count)
	}

	currentSkew := counter.largest - counter.smallest
	switch {
	case counter.largest == counter.smallest:
		// Currently all domains have the same count of bindings.
		//
		// In this case, the placement will increase the skew by 1.
		return currentSkew+1 > maxSkew, 1, nil
	case count == counter.smallest && counter.smallest != counter.secondSmallest:
		// The plan is to place at the domain with the smallest count of bindings, and currently
		// there are no other domains with the same smallest count.
		//
		// In this case, the placement will decrease the skew by 1.
		return currentSkew-1 > maxSkew, -1, nil
	case count == counter.largest:
		// The plan is to place at the domain with the largest count of bindings.
		//
		// In this case, the placement will increase the skew by 1.
		return currentSkew+1 > maxSkew, 1, nil
	default:
		// In all the other cases, the skew will not be affected.
		//
		// These cases include:
		//
		// * place at the domain with the smallest count of bindings, but there are other
		//   domains with the same smallest count; and
		// * place at the domain with a count of bindings that is greater than the smallest
		//   count, but less than the largest count.
		return currentSkew > maxSkew, 0, nil
	}
}

// classifyConstraints classifies topology spread constraints in a policy based on their
// whenUnsatisfiable requirements.
func classifyConstraints(policy *fleetv1beta1.ClusterSchedulingPolicySnapshot) (doNotSchedule, scheduleAnyway []*fleetv1beta1.TopologySpreadConstraint) {
	// Pre-allocate arrays.
	doNotSchedule = make([]*fleetv1beta1.TopologySpreadConstraint, 0, len(policy.Spec.Policy.TopologySpreadConstraints))
	scheduleAnyway = make([]*fleetv1beta1.TopologySpreadConstraint, 0, len(policy.Spec.Policy.TopologySpreadConstraints))

	for _, constraint := range policy.Spec.Policy.TopologySpreadConstraints {
		if constraint.WhenUnsatisfiable == fleetv1beta1.ScheduleAnyway {
			scheduleAnyway = append(scheduleAnyway, &constraint)
			continue
		}

		// DoNotSchedule is the default value for the whenUnsatisfiable field; currenly the only
		// two supported requirements are DoNotSchedule and ScheduleAnyway.
		doNotSchedule = append(doNotSchedule, &constraint)
	}

	return doNotSchedule, scheduleAnyway
}

// evaluateAllConstraints evaluates all topology spread constraints in a policy againsts all
// clusters being inspected in the current scheduling cycle.
func evaluateAllConstraints(
	state framework.CycleStatePluginReadWriter,
	doNotSchedule, scheduleAnyway []*fleetv1beta1.TopologySpreadConstraint,
) (violations doNotScheduleViolations, scores topologySpreadScores, err error) {
	violations = make(doNotScheduleViolations)
	// Note that this function guarantees that all clusters that do not lead to violations of
	// DoNotSchedule topology spread constraints will be assigned a score, even if none of the
	// topology spread constraints concerns these clusters.
	scores = make(topologySpreadScores)

	clusters := state.ListClusters()

	for _, constraint := range doNotSchedule {
		domainCounter := countByDomain(clusters, state, constraint.TopologyKey)

		for _, cluster := range clusters {
			val, ok := cluster.Labels[constraint.TopologyKey]
			if !ok {
				// The cluster under inspection does not have the topology key and thus is not part
				// of the spread.
				//
				// Placing resources on such clusters will not lead to topology spread constraint
				// violations.
				//
				// Assign a score even if the constraint does not concern the cluster.
				scores[clusterName(cluster.Name)] += 0
				continue
			}

			// The cluster under inspection is part of the spread.

			// Verify if the placement will violate the constraint.
			violated, skewChange, err := willViolate(domainCounter, domainName(val), int(*constraint.MaxSkew))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to evaluate DoNotSchedule topology spread constraints: %w", err)
			}
			if violated {
				// A violation happens.
				reasons := violationReasons{
					fmt.Sprintf(doNotScheduleConstraintViolationReasonTemplate, constraint.TopologyKey, constraint.MaxSkew),
				}
				violations[clusterName(cluster.Name)] = reasons
				continue
			}
			scores[clusterName(cluster.Name)] += skewChange * skewChangeScoreFactor
		}
	}

	for _, constraint := range scheduleAnyway {
		domainCounter := countByDomain(clusters, state, constraint.TopologyKey)

		for _, cluster := range clusters {
			val, ok := cluster.Labels[constraint.TopologyKey]
			if !ok {
				// The cluster under inspection does not have the topology key and thus is not part
				// of the spread.
				//
				// Placing resources on such clusters will not lead to topology spread constraint
				// violations.
				//
				// Assign a score even if the constraint does not concern the cluster.
				scores[clusterName(cluster.Name)] += 0
				continue
			}

			// The cluster under inspection is part of the spread.

			// Verify if the placement will violate the constraint.
			violated, skewChange, err := willViolate(domainCounter, domainName(val), int(*constraint.MaxSkew))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to evaluate ScheduleAnyway topology spread constraints: %w", err)
			}
			if violated {
				// A violation happens; since this is a ScheduleAnyway topology spread constraint,
				// a violation penality is applied to the score.
				scores[clusterName(cluster.Name)] -= maxSkewViolationPenality
				continue
			}
			scores[clusterName(cluster.Name)] += skewChange * skewChangeScoreFactor
		}
	}

	return violations, scores, nil
}

// prepareTopologySpreadConstraintsPluginState initializes the state for the plugin to use
// in the scheduling cycle.
func prepareTopologySpreadConstraintsPluginState(state framework.CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterSchedulingPolicySnapshot) (*topologySpreadConstraintsPluginState, error) {
	// Classify the topology spread constraints.
	doNotSchedule, scheduleAnyway := classifyConstraints(policy)

	// Based on current spread, inspect the clusters.
	//
	// Specifically, check if a cluster violates any DoNotSchedule topology spread constraint,
	// and how much of a skew change it will incur for each constraint.
	violations, scores, err := evaluateAllConstraints(state, doNotSchedule, scheduleAnyway)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare topology spread constraints plugin state: %w", err)
	}

	pluginState := &topologySpreadConstraintsPluginState{
		doNotScheduleConstraints:  doNotSchedule,
		scheduleAnywayConstraints: scheduleAnyway,
		violations:                violations,
		scores:                    scores,
	}
	return pluginState, nil
}
