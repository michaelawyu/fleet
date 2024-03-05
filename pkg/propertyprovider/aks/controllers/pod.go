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

	"go.goms.io/fleet/pkg/propertyprovider/aks/trackers"
)

type PodReconciler struct {
	PT     *trackers.PodTracker
	Client client.Client
}

func (p *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	podRef := klog.KRef(req.Namespace, req.Name)
	startTime := time.Now()
	klog.V(2).InfoS("Reconciliation starts", "pod", podRef)
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		klog.V(2).InfoS("Reconciliation ends", "pod", podRef, "latency", latency)
	}()

	// Retrieve the pod object.
	pod := &corev1.Pod{}
	if err := p.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		// Failed to get the pod object.
		if errors.IsNotFound(err) {
			// This branch essentially processes the pod deletion event (the actual deletion).
			// At this point the pod may have not been tracked by the tracker at all; if that's
			// the case, the removal (untracking) operation is a no-op.
			//
			// Note that this controller will not add any finalizer to pod objects, so as to
			// avoid blocking normal Kuberneters operations under unexpected circumstances.
			p.PT.Remove(req.NamespacedName.String())
			return ctrl.Result{}, nil
		}

		// For other errors, retry the reconciliation.
		klog.V(2).ErrorS(err, "Failed to get the pod object", "pod", podRef)
		return ctrl.Result{}, err
	}

	// Note that this controller will not untrack a pod when it is first marked for deletion;
	// instead, it performs the untracking when the pod object is actually gone from the
	// etcd store. This is intentional, as when a pod is marked for deletion, workloads might
	// not have been successfully terminated yet, and untracking the pod too early might lead to a
	// case of temporary inconsistency.

	// Track the pod if:
	//
	// * it is **NOT** of the Succeeded or Failed state; and
	// * it has been assigned to a node.
	//
	// This behavior is consistent with how the Kubernetes CLI tool reports requested capacity
	// on a specific node (`kubectl describe node` command).
	//
	// Note that the tracker will attempt to track the pod even if it has been marked for deletion.
	if len(pod.Spec.NodeName) > 0 && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
		p.PT.AddOrUpdate(pod)
	} else {
		// Untrack the pod.
		//
		// It may have been descheduled, or transited into a terminal state.
		p.PT.Remove(req.NamespacedName.String())
	}

	return ctrl.Result{}, nil
}

func (p *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Reconcile any pod changes (create, update, delete).
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(p)
}