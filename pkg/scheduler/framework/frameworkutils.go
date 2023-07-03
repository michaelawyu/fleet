/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package framework

import (
	"fmt"
	"reflect"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
	"go.goms.io/fleet/pkg/scheduler/framework/uniquename"
	"go.goms.io/fleet/pkg/utils/controller"
)

// classifyBindings categorizes bindings into the following groups:
//   - active bindings, i.e., bindings that are associated with a normally operating cluster and
//     have been cleared for processing by the dispatcher; and
//   - creating bindings, i.e., bindings that have been associated with a normally operating cluster,
//     but have not yet been cleared for processing by the dispatcher; and
//   - dangling bindings, i.e., bindings that are associated with a cluster that is no longer in
//     a normally operating state (the cluster has left the fleet, or is in the state of leaving),
//     yet has not been marked as deleting by the scheduler; and
//   - obsolete bindings, i.e., bindings that are no longer associated with the latest scheduling
//     policy; and
//   - deleted bindings, i.e, bindings that are marked for deletion in the API server, but have not
//     yet been marked as deleting by the scheduler.
func classifyBindings(policy *fleetv1beta1.ClusterPolicySnapshot, bindings []fleetv1beta1.ClusterResourceBinding, clusters []fleetv1beta1.MemberCluster) (active, creating, obsolete, dangling []*fleetv1beta1.ClusterResourceBinding) {
	// Pre-allocate arrays.
	active = make([]*fleetv1beta1.ClusterResourceBinding, 0, len(bindings))
	creating = make([]*fleetv1beta1.ClusterResourceBinding, 0, len(bindings))
	obsolete = make([]*fleetv1beta1.ClusterResourceBinding, 0, len(bindings))
	dangling = make([]*fleetv1beta1.ClusterResourceBinding, 0, len(bindings))

	// Build a map for clusters for quick loopup.
	clusterMap := make(map[string]fleetv1beta1.MemberCluster)
	for _, cluster := range clusters {
		clusterMap[cluster.Name] = cluster
	}

	for idx := range bindings {
		binding := bindings[idx]
		targetCluster, isTargetClusterPresent := clusterMap[binding.Spec.TargetCluster]

		switch {
		case binding.DeletionTimestamp != nil:
			// Ignore any binding that has been deleted.
		case binding.Spec.State == fleetv1beta1.BindingStateDeleting:
			// Ignore any binding that is of the deleting state.
		case !isTargetClusterPresent || targetCluster.Spec.State == fleetv1beta1.ClusterStateLeave:
			// Check if the binding is now dangling, i.e., it is associated with a cluster that is no longer
			// in normal operations, but is still of an active or creating state.
			//
			// Note that this check is solely for the purpose of detecting a situation where bindings are stranded
			// on a leaving/left cluster; it does not perform any binding association eligibility check for the cluster.
			dangling = append(dangling, &binding)
		case binding.Spec.PolicySnapshotName != policy.Name:
			// The binding is in the creating or active state, but is no longer associated with the latest scheduling policy snapshot.
			obsolete = append(obsolete, &binding)
		case binding.Spec.State == fleetv1beta1.BindingStateCreating:
			// Check if the binding is of the creating state.
			creating = append(creating, &binding)
		case binding.Spec.State == fleetv1beta1.BindingStateActive:
			// Check if the binding is of the active state.
			active = append(active, &binding)
		}
	}

	return active, creating, obsolete, dangling
}

// shouldDownscale checks if the scheduler needs to perform some downscaling, and (if so) how many active or creating bindings
// it should remove.
func shouldDownscale(policy *fleetv1beta1.ClusterPolicySnapshot, desired, present, obsolete int) (act bool, count int) {
	if policy.Spec.Policy.PlacementType == fleetv1beta1.PickNPlacementType && desired <= present {
		// Downscale only applies to CRPs of the Pick N placement type; and it only applies when the number of
		// clusters requested by the user is less than the number of currently active + creating bindings combined;
		// or there are the right number of active + creating bindings, yet some obsolete bindings still linger
		// in the system.
		if count := present - desired + obsolete; count > 0 {
			return true, present - desired
		}
	}
	return false, 0
}

// fullySchedulingCondition returns a condition for fully scheduled policy snapshot.
func fullyScheduledCondition(policy *fleetv1beta1.ClusterPolicySnapshot) metav1.Condition {
	return metav1.Condition{
		Type:               string(fleetv1beta1.PolicySnapshotScheduled),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             fullyScheduledReason,
		Message:            fullyScheduledMessage,
	}
}

// shouldSchedule checks if the scheduler needs to perform some scheduling.
//
// A scheduling cycle is only needed if
// * the policy is of the PickAll type; or
// * the policy is of the PickN type, and currently there are not enough number of bindings.
func shouldSchedule(policy *fleetv1beta1.ClusterPolicySnapshot, desiredCount, existingCount int) bool {
	if policy.Spec.Policy.PlacementType == fleetv1beta1.PickAllPlacementType {
		return true
	}

	return desiredCount > existingCount
}

// equalDecisions returns if two arrays of ClusterDecisions are equal; it returns true if
// every decision in one array is also present in the other array regardless of their indexes,
// and vice versa.
func equalDecisions(current, desired []fleetv1beta1.ClusterDecision) bool {
	desiredDecisionByCluster := make(map[string]fleetv1beta1.ClusterDecision, len(desired))
	for _, decision := range desired {
		desiredDecisionByCluster[decision.ClusterName] = decision
	}

	for _, decision := range current {
		// Note that it will return false if no matching decision can be found.
		if !reflect.DeepEqual(decision, desiredDecisionByCluster[decision.ClusterName]) {
			return false
		}
	}

	return len(current) == len(desired)
}

// calcNumOfClustersToSelect calculates the number of clusters to select in a scheduling run; it
// essentially returns the minimum among the desired number of clusters, the batch size limit,
// and the number of scored clusters.
func calcNumOfClustersToSelect(desired, limit, scored int) int {
	num := desired
	if limit < num {
		num = limit
	}
	if scored < num {
		num = scored
	}
	return num
}

// Pick clusters with the top N highest scores from a sorted list of clusters.
//
// Note that this function assumes that the list of clusters have been sorted by their scores,
// and the N count is no greater than the length of the list.
func pickTopNScoredClusters(scoredClusters ScoredClusters, N int) ScoredClusters {
	// Sort the clusters by their scores in reverse order.
	//
	// Note that when two clusters have the same score, they are sorted by their names in
	// lexicographical order instead; this is to achieve deterministic behavior when picking
	// clusters.
	sort.Sort(sort.Reverse(scoredClusters))

	// No need to pick if there is no scored cluster or the number to pick is zero.
	if len(scoredClusters) == 0 || N == 0 {
		return make(ScoredClusters, 0)
	}

	// No need to pick if the number of scored clusters is less than or equal to N.
	if len(scoredClusters) <= N {
		return scoredClusters
	}

	return scoredClusters[:N]
}

// crossReferencePickedCustersAndBindings cross references picked clusters in the current scheduling
// run and existing bindings to find out:
//
//   - bindings that should be created, i.e., create a binding for every cluster that is newly picked
//     and does not have a binding associated with;
//   - bindings that should be updated, i.e., associate a binding, whose target cluster is picked again
//     in the current run, with the latest scheduling policy snapshot (if applicable);
//   - bindings that should be deleted, i.e., mark a binding as deleting if its target cluster is no
//     longer picked in the current run.
//
// Note that this function will return bindings with all fields fulfilled/refreshed, as applicable.
func crossReferencePickedCustersAndObsoleteBindings(crpName string, policy *fleetv1beta1.ClusterPolicySnapshot, picked ScoredClusters, obsolete []*fleetv1beta1.ClusterResourceBinding) (toCreate, toUpdate, toDelete []*fleetv1beta1.ClusterResourceBinding, err error) {
	errorFormat := "failed to cross reference picked clusters and existing bindings: %w"

	// Pre-allocate with a reasonable capacity.
	toCreate = make([]*fleetv1beta1.ClusterResourceBinding, 0, len(picked))
	toUpdate = make([]*fleetv1beta1.ClusterResourceBinding, 0, 20)
	toDelete = make([]*fleetv1beta1.ClusterResourceBinding, 0, 20)

	// Build a map of picked scored clusters for quick lookup.
	pickedMap := make(map[string]*ScoredCluster)
	for _, scored := range picked {
		pickedMap[scored.Cluster.Name] = scored
	}

	// Build a map of all clusters that have been cross-referenced.
	checked := make(map[string]bool)

	for _, binding := range obsolete {
		scored, ok := pickedMap[binding.Spec.TargetCluster]
		checked[binding.Spec.TargetCluster] = true

		if ok {
			// The binding's target cluster is picked again in the current run; yet the binding
			// is originally created/updated in accordance with an out-of-date scheduling policy.

			// Update the binding so that it is associated with the latest score.
			affinityScore := int32(scored.Score.AffinityScore)
			topologySpreadScore := int32(scored.Score.TopologySpreadScore)
			binding.Spec.ClusterDecision = fleetv1beta1.ClusterDecision{
				ClusterName: scored.Cluster.Name,
				Selected:    true,
				ClusterScore: &fleetv1beta1.ClusterScore{
					AffinityScore:       &affinityScore,
					TopologySpreadScore: &topologySpreadScore,
				},
				Reason: pickedByPolicyReason,
			}

			// Update the binding so that it is associated with the lastest scheduling policy.
			binding.Spec.PolicySnapshotName = policy.Name

			// Add the binding to the toUpdate list.
			toUpdate = append(toUpdate, binding)
		} else {
			toDelete = append(toDelete, binding)
		}
	}

	for _, scored := range picked {
		if _, ok := checked[scored.Cluster.Name]; !ok {
			// The cluster is newly picked in the current run; it does not have an associated binding in presence.
			name, err := uniquename.ClusterResourceBindingUniqueName(crpName, scored.Cluster.Name)
			if err != nil {
				// Cannot get a unique name for the binding; normally this should never happen.
				return nil, nil, nil, controller.NewUnexpectedBehaviorError(fmt.Errorf(errorFormat, err))
			}
			affinityScore := int32(scored.Score.AffinityScore)
			topologySpreadScore := int32(scored.Score.TopologySpreadScore)

			toCreate = append(toCreate, &fleetv1beta1.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						fleetv1beta1.CRPTrackingLabel: crpName,
					},
				},
				Spec: fleetv1beta1.ResourceBindingSpec{
					State: fleetv1beta1.BindingStateCreating,
					// Leave the associated resource snapshot name empty; it is up to another controller
					// to fulfill this field.
					PolicySnapshotName: policy.Name,
					TargetCluster:      scored.Cluster.Name,
					ClusterDecision: fleetv1beta1.ClusterDecision{
						ClusterName: scored.Cluster.Name,
						Selected:    true,
						ClusterScore: &fleetv1beta1.ClusterScore{
							AffinityScore:       &affinityScore,
							TopologySpreadScore: &topologySpreadScore,
						},
						Reason: pickedByPolicyReason,
					},
				},
			})
		}
	}

	return toCreate, toUpdate, toDelete, nil
}

// newSchedulingDecisionsFrom returns a list of scheduling decisions, based on the newly manipulated list of
// bindings and (if applicable) a list of filtered clusters.
func newSchedulingDecisionsFrom(maxClusterDecisionCount int, filtered []*filteredClusterWithStatus, existing ...[]*fleetv1beta1.ClusterResourceBinding) []fleetv1beta1.ClusterDecision {
	// Pre-allocate with a reasonable capacity.
	newDecisions := make([]fleetv1beta1.ClusterDecision, 0, maxClusterDecisionCount)

	// Build new scheduling decisions.
	for _, bindingSet := range existing {
		for _, binding := range bindingSet {
			newDecisions = append(newDecisions, binding.Spec.ClusterDecision)
		}
	}

	// Move some decisions from unbound clusters, if there are still enough room.
	if diff := maxClusterDecisionCount - len(newDecisions); diff > 0 {
		for i := 0; i < diff && i < len(filtered); i++ {
			clusterWithStatus := filtered[i]
			newDecisions = append(newDecisions, fleetv1beta1.ClusterDecision{
				ClusterName: clusterWithStatus.cluster.Name,
				Selected:    false,
				Reason:      clusterWithStatus.status.String(),
			})
		}
	}

	return newDecisions
}

// shouldRequeue determines if the scheduler should start another scheduling cycle on the same
// policy snapshot.
//
// For each scheduling run, four different possibilities exist:
//
//   - the desired batch size is equal to the batch size limit, i.e., no plugin has imposed a limit
//     on the batch size; and the actual number of bindings created/updated is equal to the desired
//     batch size
//     -> in this case, no immediate requeue is necessary as all the work has been completed.
//   - the desired batch size is equal to the batch size limit, i.e., no plugin has imposed a limit
//     on the batch size; but the actual number of bindings created/updated is less than the desired
//     batch size
//     -> in this case, no immediate requeue is necessary as retries will not correct the situation;
//     the scheduler should wait for the next signal from scheduling triggers, e.g., new cluster
//     joined, or scheduling policy is updated.
//   - the desired batch size is less than the batch size limit, i.e., a plugin has imposed a limit
//     on the batch size; and the actual number of bindings created/updated is equal to batch size
//     limit
//     -> in this case, immediate requeue is needed as there might be more fitting clusters to bind
//     resources to.
//   - the desired batch size is less than the batch size limit, i.e., a plugin has imposed a limit
//     on the batch size; but the actual number of bindings created/updated is less than the batch
//     size limit
//     -> in this case, no immediate requeue is necessary as retries will not correct the situation;
//     the scheduler should wait for the next signal from scheduling triggers, e.g., new cluster
//     joined, or scheduling policy is updated.
func shouldRequeue(desiredBatchSize, batchSizeLimit, bindingCount int) bool {
	if desiredBatchSize > batchSizeLimit && bindingCount == batchSizeLimit {
		return true
	}
	return false
}
