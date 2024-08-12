/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package kueue

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	placementv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

const (
	// dryRunCRPRefLabel is a label key that a user could add to job objects to indicate
	// that in a MultiKueue setup the placement of the job should refer to the scheduling
	// decisions produced by the CRP.
	dryRunCRPRefLabel = "kubernetes-fleet.io/dry-run-crp-ref"

	// clusterManagerSchedulingHintAnnotation is the annotation key used to hint the scheduling decisions
	// made by Fleet. The annotation value is a list of comma-separated cluster names,
	// with the first cluster being the most preferred one.
	//
	// Note (chenyu1): this is obviously a hack, but for demo purposes it should suffice.
	clusterManagerSchedulingHintAnnotation = "kueue.x-k8s.io/cluster-manager-scheduling-hint"
)

const (
	clusterDecisionCountLimit = 5
)

// Reconciler reconciles Job objects.
type Reconciler struct {
	HubClient client.Client
}

// Reconcile reconciles a Job object.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jobRef := klog.KRef(req.Namespace, req.Name)
	startTime := time.Now()
	klog.V(2).InfoS("Reconciliation starts (job controller for multikueue setup)", "Job", jobRef)
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		klog.V(2).InfoS("Reconciliation ends (job controller for multikueue setup)", "Job", jobRef, "Latency", latency)
	}()

	// Retrieve the Job object.
	job := &batchv1.Job{}
	if err := r.HubClient.Get(ctx, req.NamespacedName, job); err != nil {
		klog.ErrorS(err, "Failed to get Job object", "Job", jobRef)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the job has the dryRunCRPRefLabel label.
	crpName, found := job.Labels[dryRunCRPRefLabel]
	if !found {
		klog.V(2).InfoS("Job does not have the dryRunCRPRefLabel label", "Job", jobRef)
		return ctrl.Result{}, nil
	}

	// Find the latest ClusterSchedulingPolicySnapshot associated with the CRP object.
	cspsList := &placementv1beta1.ClusterSchedulingPolicySnapshotList{}
	matchLabelOpt := &client.MatchingLabels{
		placementv1beta1.CRPTrackingLabel:      crpName,
		placementv1beta1.IsLatestSnapshotLabel: strconv.FormatBool(true),
	}
	if err := r.HubClient.List(ctx, cspsList, matchLabelOpt); err != nil {
		klog.ErrorS(err, "Failed to list ClusterSchedulingPolicySnapshot objects", "Job", jobRef)
		return ctrl.Result{}, err
	}
	if len(cspsList.Items) != 1 {
		klog.V(2).InfoS("No ClusterSchedulingPolicySnapshot objects found", "Job", jobRef)
		return ctrl.Result{RequeueAfter: time.Second * 2}, nil
	}

	snapshot := &cspsList.Items[0]
	klog.V(2).InfoS("Found the latest ClusterSchedulingPolicySnapshot object",
		"Job", jobRef,
		"ClusterSchedulingPolicySnapshot", klog.KObj(snapshot),
		"ClusterResourcePlacement", klog.KRef("", crpName))

	// Extract the scheduling decisions from the policy snapshot.
	allDecisions := snapshot.Status.ClusterDecisions
	slices.SortFunc(allDecisions, func(i, j placementv1beta1.ClusterDecision) int {
		// If one decision is positive (selected) and the other is negative (not selected),
		// the positive decision should come first.
		if i.Selected != j.Selected {
			if i.Selected {
				return 1
			}
			return -1
		}

		// If both decisions are positive, the decision with the higher score should come first.
		if i.Selected {
			// As a sanity check, assume nil score = zero score.
			asi := 0
			if i.ClusterScore.AffinityScore != nil {
				asi = int(*i.ClusterScore.AffinityScore)
			}
			tsi := 0
			if i.ClusterScore.TopologySpreadScore != nil {
				tsi = int(*i.ClusterScore.TopologySpreadScore)
			}
			asj := 0
			if j.ClusterScore.AffinityScore != nil {
				asj = int(*j.ClusterScore.AffinityScore)
			}
			tsj := 0
			if j.ClusterScore.TopologySpreadScore != nil {
				tsj = int(*j.ClusterScore.TopologySpreadScore)
			}

			switch {
			case asi > asj:
				return 1
			case asi == asj && tsi > tsj:
				return 1
			case asi == asj && tsi == tsj:
				return 0
			default:
				return -1
			}
		}

		// If both decisions are negative, the order does not matter.
		return 0
	})

	limit := clusterDecisionCountLimit
	if len(allDecisions) < limit {
		limit = len(allDecisions)
	}
	preferredNames := []string{}
	for i := 0; i < limit; i++ {
		decision := allDecisions[i]
		if !decision.Selected {
			break
		}
		preferredNames = append(preferredNames, decision.ClusterName)
		klog.V(4).InfoS("Found a scheduling decision", "Job", jobRef, "ClusterName", decision.ClusterName)
	}

	// Update the job object with the scheduling hint annotation.
	if len(preferredNames) == 0 {
		klog.V(2).InfoS("No scheduling hints found; wait for fulfillment (if any)", "Job", jobRef)
		return ctrl.Result{RequeueAfter: time.Second * 2}, nil
	}

	annotations := job.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[clusterManagerSchedulingHintAnnotation] = strings.Join(preferredNames, ",")
	job.SetAnnotations(annotations)
	if err := r.HubClient.Update(ctx, job); err != nil {
		klog.ErrorS(err, "Failed to update Job object with scheduling hint annotation", "Job", jobRef)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Complete(r)
}
