/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package clusterprofile

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	clusterinventoryv1alpha1 "go.goms.io/fleet/apis/clusterinventory/v1alpha1"
)

const (
	// clusterProfileCleanupFinalizer is the finalizer added to a MemberCluster object if
	// a corresponding ClusterProfile object has been created.
	clusterProfileCleanupFinalizer = "kubernetes-fleet.io/cluster-profile-cleanup"

	// memberAgentHeartbeatLossThreshold is the time threshold for determining if the Fleet
	// member agent has lost its heartbeat connection to the Fleet hub cluster.
	memberAgentHeartbeatLossThreshold = 5 * time.Minute
)

// Reconciler reconciles a MemberCluster object and creates the corresponding ClusterProfile
// object in the designated namespace.
type Reconciler struct {
	HubClient               client.Client
	ClusterProfileNamespace string
}

// Reconcile processes the MemberCluster object and creates the corresponding ClusterProfile object
// in the designated namespace.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	mcRef := klog.KRef(req.Namespace, req.Name)
	startTime := time.Now()
	klog.V(2).InfoS("Reconciliation starts (cluster profile controller)", "MemberCluster", mcRef)
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		klog.V(2).InfoS("Reconciliation ends (cluster profile controller)", "MemberCluster", mcRef, "latency", latency)
	}()

	// Retrieve the MemberCluster object.
	mc := &clusterv1beta1.MemberCluster{}
	if err := r.HubClient.Get(ctx, req.NamespacedName, mc); err != nil {
		if errors.IsNotFound(err) {
			klog.InfoS("Member cluster object is not found", "MemberCluster", mcRef)

			// To address a corner case where a member cluster is deleted before its cluster
			// profile is cleaned up or a cluster profile has been created without the
			// acknowledgment of this controller,
			// check if a cluster profile exists for the member cluster and delete it.
			mc = &clusterv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: req.Name,
				},
			}
			if err := r.cleanupClusterProfile(ctx, mc); err != nil {
				klog.ErrorS(err, "Failed to clean up cluster profile when member cluster is already gone", "MemberCluster", mcRef)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		klog.ErrorS(err, "Failed to get member cluster", "MemberCluster", mcRef)
		return ctrl.Result{}, err
	}

	// Check if the member cluster object has been marked for deletion.
	if mc.DeletionTimestamp != nil {
		klog.V(2).InfoS("Member cluster object is being deleted; remove the corresponding cluster profile", "MemberCluster", mcRef)

		// Delete the corresponding ClusterProfile object.
		if err := r.cleanupClusterProfile(ctx, mc); err != nil {
			klog.ErrorS(err, "Failed to clean up cluster profile when member cluster is marked for deletion", "MemberCluster", mcRef)
			return ctrl.Result{}, err
		}

		// Remove the cleanup finalizer from the MemberCluster object.
		controllerutil.RemoveFinalizer(mc, clusterProfileCleanupFinalizer)
		if err := r.HubClient.Update(ctx, mc); err != nil {
			klog.ErrorS(err, "Failed to remove cleanup finalizer", "MemberCluster", mcRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if the MemberCluster object has the cleanup finalizer; if not, add it.
	if !controllerutil.ContainsFinalizer(mc, clusterProfileCleanupFinalizer) {
		mc.Finalizers = append(mc.Finalizers, clusterProfileCleanupFinalizer)
		if err := r.HubClient.Update(ctx, mc); err != nil {
			klog.ErrorS(err, "Failed to add cleanup finalizer", "MemberCluster", mcRef)
			return ctrl.Result{}, err
		}
	}

	// Retrieve the corresponding ClusterProfile object. If the object does not exist, create it.
	cp := &clusterinventoryv1alpha1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ClusterProfileNamespace,
			Name:      mc.Name,
		},
	}
	// Note that if the object already exists and its spec matches with the desired space, no
	// update op will be performed.
	createOrUpdateRes, err := controllerutil.CreateOrUpdate(ctx, r.HubClient, cp, func() error {
		if cp.CreationTimestamp.IsZero() {
			// Only set the ClusterManager field if the object is being created; this field
			// is immutable by definition.
			cp.Spec = clusterinventoryv1alpha1.ClusterProfileSpec{
				ClusterManager: clusterinventoryv1alpha1.ClusterManager{
					Name: clusterinventoryv1alpha1.ClusterManagerName,
				},
			}
		}

		// Return an error if the cluster profile is under the management of a different platform.
		if cp.Spec.ClusterManager.Name != clusterinventoryv1alpha1.ClusterManagerName {
			return fmt.Errorf("cluster profile is under the management of a different platform: %s", cp.Spec.ClusterManager.Name)
		}

		// Set the labels.
		if cp.Labels == nil {
			cp.Labels = make(map[string]string)
		}
		cp.Labels[clusterinventoryv1alpha1.LabelClusterManagerKey] = clusterinventoryv1alpha1.ClusterManagerName

		// Set the display name.
		cp.Spec.DisplayName = mc.Name
		return nil
	})
	if err != nil {
		klog.ErrorS(err, "Failed to create or update cluster profile", "MemberCluster", mcRef, "ClusterProfile", klog.KObj(cp), "operation", createOrUpdateRes)
		return ctrl.Result{}, err
	}
	klog.V(2).InfoS("Cluster profile object is created or updated", "MemberCluster", mcRef, "ClusterProfile", klog.KObj(cp), "operation", createOrUpdateRes)

	// Update the cluster profile status.
	//
	// For simplicity reasons, for now only the health check condition is populated, using
	// Fleet member agent's API server health check result.
	var mcHealthCond *metav1.Condition
	var memberAgentLastHeartbeat *metav1.Time

	memberAgentStatus := mc.GetAgentStatus(clusterv1beta1.MemberAgent)
	if memberAgentStatus != nil {
		mcHealthCond = meta.FindStatusCondition(memberAgentStatus.Conditions, string(clusterv1beta1.AgentHealthy))
		memberAgentLastHeartbeat = &memberAgentStatus.LastReceivedHeartbeat
	}
	switch {
	case memberAgentStatus == nil:
		// The member agent hasn't reported its status yet.
		// Set the unknown health condition in the cluster profile status.
		meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
			Type:               clusterinventoryv1alpha1.ClusterConditionControlPlaneHealthy,
			Status:             metav1.ConditionUnknown,
			Reason:             "MemberAgentReportedNoStatus",
			ObservedGeneration: cp.Generation,
			Message:            "The Fleet member agent has not reported its status yet",
		})
	case mcHealthCond == nil:
		// The member agent has reported its status, but the health condition is missing.
		// Set the unknown health condition in the cluster profile status.
		meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
			Type:               clusterinventoryv1alpha1.ClusterConditionControlPlaneHealthy,
			Status:             metav1.ConditionUnknown,
			Reason:             "MemberAgentReportedNoHealthInfo",
			ObservedGeneration: cp.Generation,
			Message:            "The Fleet member agent has reported its status, but the health condition is missing",
		})
	case memberAgentLastHeartbeat == nil || time.Since(memberAgentLastHeartbeat.Time) > memberAgentHeartbeatLossThreshold:
		// The member agent has lost its heartbeat connection to the Fleet hub cluster.
		// Set the unknown health condition in the cluster profile status.
		meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
			Type:               clusterinventoryv1alpha1.ClusterConditionControlPlaneHealthy,
			Status:             metav1.ConditionUnknown,
			Reason:             "MemberAgentHeartbeatLost",
			ObservedGeneration: cp.Generation,
			Message:            "The Fleet member agent has lost its heartbeat connection to the Fleet hub cluster",
		})
	//case mcHealthCond.Status == metav1.ConditionUnknown || mcHealthCond.ObservedGeneration != mc.Generation:
	// Note (chenyu1): Skip the generation check as Fleet member agent currently does not handle
	// the health condition appropriately.
	case mcHealthCond.Status == metav1.ConditionUnknown:
		// The health condition has not been updated.
		// Set the unknown health condition in the cluster profile status.
		meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
			Type:               clusterinventoryv1alpha1.ClusterConditionControlPlaneHealthy,
			Status:             metav1.ConditionUnknown,
			Reason:             "MemberAgentHealthCheckResultUnknown",
			ObservedGeneration: cp.Generation,
			Message:            "The Fleet member agent health check result is out of date or unknown",
		})
	case mcHealthCond.Status == metav1.ConditionFalse:
		// The member agent reports that the API server is unhealthy.
		// Set the false health condition in the cluster profile status.
		meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
			Type:               clusterinventoryv1alpha1.ClusterConditionControlPlaneHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "MemberClusterAPIServerUnhealthy",
			ObservedGeneration: cp.Generation,
			Message:            "The Fleet member agent reports that the API server is unhealthy",
		})
	default:
		// The member agent reports that the API server is healthy.
		// Set the true health condition in the cluster profile status.
		meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
			Type:               clusterinventoryv1alpha1.ClusterConditionControlPlaneHealthy,
			Status:             metav1.ConditionTrue,
			Reason:             "MemberClusterAPIServerHealthy",
			ObservedGeneration: cp.Generation,
			Message:            "The Fleet member agent reports that the API server is healthy",
		})
	}
	if err := r.HubClient.Status().Update(ctx, cp); err != nil {
		klog.ErrorS(err, "Failed to update cluster profile status", "MemberCluster", mcRef, "ClusterProfile", klog.KObj(cp))
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the controller manager.
func (r *Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1beta1.MemberCluster{}).
		// TO-DO (chenyu1): watch also cluster profile objects.
		Complete(r)
}

// cleanupClusterProfile deletes the ClusterProfile object associated with a given MemberCluster object.
func (r *Reconciler) cleanupClusterProfile(ctx context.Context, mc *clusterv1beta1.MemberCluster) error {
	cp := &clusterinventoryv1alpha1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.ClusterProfileNamespace,
			Name:      mc.Name,
		},
	}
	if err := r.HubClient.Delete(ctx, cp); err != nil && !errors.IsNotFound(err) {
		klog.ErrorS(err, "Failed to delete cluster profile", "MemberCluster", klog.KObj(mc), "ClusterProfile", klog.KObj(cp))
		return err
	}
	return nil
}
