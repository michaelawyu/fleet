/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package workapplier

import (
	"context"
	"fmt"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"go.goms.io/fleet/pkg/utils"
	"go.goms.io/fleet/pkg/utils/controller"
)

func (r *Reconciler) trackInMemberClusterObjAvailability(ctx context.Context, bundles []*manifestProcessingBundle, workRef *klog.ObjectRef) {
	// Track the availability of all the applied objects in the member cluster in parallel.
	//
	// This is concurrency-safe as the bundles slice has been pre-allocated.

	// Prepare a child context.
	// Cancel the child context anyway to avoid leaks.
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	doWork := func(pieces int) {
		bundle := bundles[pieces]
		if !isManifestObjectApplied(bundle.applyResTyp) {
			// The manifest object in the bundle has not been applied yet. No availability check
			// is needed.
			bundle.availabilityResTyp = ManifestProcessingAvailabilityResultTypeSkipped
			return
		}

		availabilityResTyp, err := trackInMemberClusterObjAvailabilityByGVR(bundle.gvr, bundle.inMemberClusterObj)
		if err != nil {
			// An unexpected error has occurred during the availability check.
			bundle.availabilityErr = err
			bundle.availabilityResTyp = ManifestProcessingAvailabilityResultTypeFailed
			klog.ErrorS(err,
				"Failed to track the availability of the applied object in the member cluster",
				"work", *workRef, "GVR", *bundle.gvr, "inMemberClusterObj", klog.KObj(bundle.inMemberClusterObj))
			return
		}
		bundle.availabilityResTyp = availabilityResTyp
	}

	// Run the availability check in parallel.
	r.parallelizer.ParallelizeUntil(childCtx, len(bundles), doWork, "trackInMemberClusterObjAvailability")
}

func trackInMemberClusterObjAvailabilityByGVR(
	gvr *schema.GroupVersionResource,
	inMemberClusterObj *unstructured.Unstructured,
) (ManifestProcessingAvailabilityResultType, error) {
	switch *gvr {
	case utils.DeploymentGVR:
		return trackDeploymentAvailability(inMemberClusterObj)
	case utils.StatefulSettGVR:
		return trackStatefulSetAvailability(inMemberClusterObj)
	case utils.DaemonSettGVR:
		return trackDaemonSetAvailability(inMemberClusterObj)
	case utils.ServiceGVR:
		return trackServiceAvailability(inMemberClusterObj)
	default:
		if isDataResource(*gvr) {
			klog.V(2).InfoS("The object from the member cluster is a data object, consider it to be immediately available",
				"gvr", *gvr, "inMemberClusterObj", klog.KObj(inMemberClusterObj))
			return ManifestProcessingAvailabilityResultTypeAvailable, nil
		}
		klog.V(2).InfoS("Cannot determine the availability of the object from the member cluster; untrack its availability",
			"gvr", *gvr, "resource", klog.KObj(inMemberClusterObj))
		return ManifestProcessingAvailabilityResultTypeNotTrackable, nil
	}
}

func trackDeploymentAvailability(inMemberClusterObj *unstructured.Unstructured) (ManifestProcessingAvailabilityResultType, error) {
	var deploy appv1.Deployment
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(inMemberClusterObj.Object, &deploy); err != nil {
		// Normally this branch should never run.
		return ManifestProcessingAvailabilityResultTypeFailed, controller.NewUnexpectedBehaviorError(fmt.Errorf("failed to convert the unstructured object to a deployment: %w", err))
	}

	// Check if the deployment is available.
	requiredReplicas := int32(1)
	if deploy.Spec.Replicas != nil {
		requiredReplicas = *deploy.Spec.Replicas
	}
	if deploy.Status.ObservedGeneration == deploy.Generation &&
		requiredReplicas == deploy.Status.AvailableReplicas &&
		requiredReplicas == deploy.Status.UpdatedReplicas {
		klog.V(2).InfoS("Deployment is available", "deployment", klog.KObj(inMemberClusterObj))
		return ManifestProcessingAvailabilityResultTypeAvailable, nil
	}
	klog.V(2).InfoS("Deployment is not ready yet, will check later to see if it becomes available", "deployment", klog.KObj(inMemberClusterObj))
	return ManifestProcessingAvailabilityResultTypeNotYetAvailable, nil
}

func trackStatefulSetAvailability(inMemberClusterObj *unstructured.Unstructured) (ManifestProcessingAvailabilityResultType, error) {
	var statefulSet appv1.StatefulSet
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(inMemberClusterObj.Object, &statefulSet); err != nil {
		// Normally this branch should never run.
		return ManifestProcessingAvailabilityResultTypeFailed, controller.NewUnexpectedBehaviorError(fmt.Errorf("failed to convert the unstructured object to a stateful set: %w", err))
	}

	// Check if the stateful set is available.
	//
	// A statefulSet is available if all if its replicas are available and the current replica count
	// is equal to the updated replica count, which implies that all replicas are up to date.
	requiredReplicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		requiredReplicas = *statefulSet.Spec.Replicas
	}
	if statefulSet.Status.ObservedGeneration == statefulSet.Generation &&
		statefulSet.Status.AvailableReplicas == requiredReplicas &&
		statefulSet.Status.CurrentReplicas == statefulSet.Status.UpdatedReplicas &&
		statefulSet.Status.CurrentRevision == statefulSet.Status.UpdateRevision {
		klog.V(2).InfoS("StatefulSet is available", "statefulSet", klog.KObj(inMemberClusterObj))
		return ManifestProcessingAvailabilityResultTypeAvailable, nil
	}
	klog.V(2).InfoS("Stateful set is not ready yet, will check later to see if it becomes available", "statefulSet", klog.KObj(inMemberClusterObj))
	return ManifestProcessingAvailabilityResultTypeNotYetAvailable, nil
}

func trackDaemonSetAvailability(inMemberClusterObj *unstructured.Unstructured) (ManifestProcessingAvailabilityResultType, error) {
	var daemonSet appv1.DaemonSet
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(inMemberClusterObj.Object, &daemonSet); err != nil {
		// Normally this branch should never run.
		return ManifestProcessingAvailabilityResultTypeFailed, controller.NewUnexpectedBehaviorError(fmt.Errorf("failed to convert the unstructured object to a daemon set: %w", err))
	}

	// Check if the daemonSet is available.
	//
	// A daemonSet is available if all if its desired replicas (the count of which is equal to
	// the number of applicable nodes in the cluster) are available and the current replica count
	// is equal to the updated replica count, which implies that all replicas are up to date.
	if daemonSet.Status.ObservedGeneration == daemonSet.Generation &&
		daemonSet.Status.NumberAvailable == daemonSet.Status.DesiredNumberScheduled &&
		daemonSet.Status.CurrentNumberScheduled == daemonSet.Status.UpdatedNumberScheduled {
		klog.V(2).InfoS("DaemonSet is available", "daemonSet", klog.KObj(inMemberClusterObj))
		return ManifestProcessingAvailabilityResultTypeAvailable, nil
	}
	klog.V(2).InfoS("Daemon set is not ready yet, will check later to see if it becomes available", "daemonSet", klog.KObj(inMemberClusterObj))
	return ManifestProcessingAvailabilityResultTypeNotYetAvailable, nil
}

func trackServiceAvailability(inMemberClusterObj *unstructured.Unstructured) (ManifestProcessingAvailabilityResultType, error) {
	var svc corev1.Service
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(inMemberClusterObj.Object, &svc); err != nil {
		return ManifestProcessingAvailabilityResultTypeFailed, controller.NewUnexpectedBehaviorError(fmt.Errorf("failed to convert the unstructured object to a service: %w", err))
	}
	switch svc.Spec.Type {
	case "":
		fallthrough // The default service type is ClusterIP.
	case corev1.ServiceTypeClusterIP:
		fallthrough
	case corev1.ServiceTypeNodePort:
		// Fleet considers a ClusterIP or NodePort service to be available if it has at least one
		// IP assigned.
		if len(svc.Spec.ClusterIPs) > 0 && len(svc.Spec.ClusterIPs[0]) > 0 {
			klog.V(2).InfoS("Service is available", "service", klog.KObj(inMemberClusterObj), "serviceType", svc.Spec.Type)
			return ManifestProcessingAvailabilityResultTypeAvailable, nil
		}
		klog.V(2).InfoS("Service is not ready yet, will check later to see if it becomes available", "service", klog.KObj(inMemberClusterObj), "serviceType", svc.Spec.Type)
		return ManifestProcessingAvailabilityResultTypeNotYetAvailable, nil
	case corev1.ServiceTypeLoadBalancer:
		// Fleet considers a loadBalancer service to be available if it has at least one load
		// balancer IP or hostname assigned.
		if len(svc.Status.LoadBalancer.Ingress) > 0 &&
			(len(svc.Status.LoadBalancer.Ingress[0].IP) > 0 || len(svc.Status.LoadBalancer.Ingress[0].Hostname) > 0) {
			klog.V(2).InfoS("Service is available", "service", klog.KObj(inMemberClusterObj), "serviceType", svc.Spec.Type)
			return ManifestProcessingAvailabilityResultTypeAvailable, nil
		}
		klog.V(2).InfoS("Service is not ready yet, will check later to see if it becomes available", "service", klog.KObj(inMemberClusterObj), "serviceType", svc.Spec.Type)
		return ManifestProcessingAvailabilityResultTypeNotYetAvailable, nil
	}

	// we don't know how to track the availability of when the service type is externalName
	klog.V(2).InfoS("Cannot determine the availability of external name services; untrack its availability", "service", klog.KObj(inMemberClusterObj))
	return ManifestProcessingAvailabilityResultTypeNotTrackable, nil
}

// isDataResource checks if the resource is a data resource which means it is available immediately after creation.
func isDataResource(gvr schema.GroupVersionResource) bool {
	switch gvr {
	case utils.NamespaceGVR:
		return true
	case utils.SecretGVR:
		return true
	case utils.ConfigMapGVR:
		return true
	case utils.RoleGVR:
		return true
	case utils.ClusterRoleGVR:
		return true
	case utils.RoleBindingGVR:
		return true
	case utils.ClusterRoleBindingGVR:
		return true
	}
	return false
}