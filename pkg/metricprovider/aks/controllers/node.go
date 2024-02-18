/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package controllers

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.goms.io/fleet/pkg/metricprovider/aks/trackers"
)

type NodeReconciler struct {
	NT     *trackers.NodeTracker
	Client client.Client
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nodeRef := klog.KRef(req.Namespace, req.Name)
	startTime := time.Now()
	klog.V(2).InfoS("Reconciliation starts", "node", nodeRef)
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		klog.V(2).InfoS("Reconciliation ends", "node", nodeRef, "latency", latency)
	}()

	// Retrieve the node object.
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		// Failed to get the node object.
		if errors.IsNotFound(err) {
			// If the cause of the failure is that the node object is not found, it could be that
			// the node has been deleted before this controller from the AKS metric provider
			// gets a chance to process the deletion. The node may or may not have been tracked
			// by the metric provider, and to be on the safer side, the controller attempts
			// to untrack the node either way.
			//
			// Note that this controller will not add any finalizer to node objects, so as to
			// avoid blocking normal Kuberneters operations under unexpected circumstances.
			klog.V(2).InfoS("Node is not found; untrack it from the metric provider", "node", nodeRef)
			r.NT.Remove(node.Name)
			return ctrl.Result{}, nil
		}
		// For other errors, retry the reconciliation.
		klog.V(2).ErrorS(err, "Failed to get the node object", "node", nodeRef)
		return ctrl.Result{}, err
	}

	// Track the node. If it has been tracked, update its total and allocatable capacity
	// information with the tracker.
	//
	// Note that normally the capacity information remains immutable before object
	// creation; the tracker update only serves as a sanity check.
	//
	// Also note that the tracker will attempt to track the node even if it has been
	// marked for deletion, as cordoned, or as unschedulable. This behavior is consistent with
	// the original Fleet setup.
	r.NT.AddOrUpdate(node)

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Reconcile any node changes (create, update, delete).
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
