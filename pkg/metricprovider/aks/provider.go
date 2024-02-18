/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

// Package aks features the AKS metric provider for Fleet.
package aks

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	"go.goms.io/fleet/pkg/metricprovider"
	"go.goms.io/fleet/pkg/metricprovider/aks/controllers"
	"go.goms.io/fleet/pkg/metricprovider/aks/trackers"
)

const (
	// A list of metric names that the AKS metric provider collects.

	// NodeCountMetric is a metric that describes the number of nodes in the cluster.
	NodeCountMetric = "kubernetes.azure.com/node-count"
)

const (
	// The condition related values in use by the AKS metric provider.

	// MetricCollectionSucceededConditionType is a condition type that indicates whether a
	// metric collection attempt has succeeded.
	MetricCollectionSucceededConditionType = "MetricCollectionSucceeded"
	MetricCollectionSucceededReason        = "AllMetricsCollectedSuccessfully"
	MetricCollectionSucceededMessage       = "All metrics have been collected successfully"
)

type MetricProvider struct {
	pt *trackers.PodTracker
	nt *trackers.NodeTracker

	// The controller manager in use by the AKS metric provider; this field is mostly reserved for
	// testing purposes.
	mgr ctrl.Manager
}

// Verify that the AKS metric provider implements the MetricProvider interface at compile time.
var _ metricprovider.MetricProvider = &MetricProvider{}

func (p *MetricProvider) Start(ctx context.Context, config *rest.Config) error {
	klog.V(2).Info("Starting AKS metric provider")

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme: scheme.Scheme,
		// Disable metric serving for the AKS metric provider controller manager.
		//
		// Note that this will not stop the metrics from being collected and exported; as they
		// are registered via a top-level variable as a part of the controller runtime package,
		// which is also used by the Fleet member agent.
		MetricsBindAddress: "0",
		// Disable health probe serving for the AKS metric provider controller manager.
		HealthProbeBindAddress: "0",
		// Disable leader election for the AKS metric provider.
		//
		// Note that for optimal performance, only the running instance of the Fleet member agent
		// (if there are multiple ones) should have the AKS metric provider enabled; this can
		// be achieved by starting the AKS metric provider only when an instance of the Fleet
		// member agent wins the leader election. It should be noted that running the AKS metric
		// provider for multiple times will not incur any side effect other than some minor
		// performance costs, as at this moment the AKS metric provider observes data individually
		// in a passive manner with no need for any centralized state.
		LeaderElection: false,
	})
	p.mgr = mgr

	if err != nil {
		klog.Errorf("Failed to start AKS metric provider: %v", err)
		return err
	}

	// Set up the node and pod reconcilers.
	klog.V(2).Info("Starting the node reconciler")
	nodeReconciler := &controllers.NodeReconciler{
		NT:     p.nt,
		Client: mgr.GetClient(),
	}
	if err := nodeReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Failed to start the node reconciler in the AKS metric provider: %v", err)
		return err
	}

	klog.V(2).Info("Starting the pod reconciler")
	podReconciler := &controllers.PodReconciler{
		PT:     p.pt,
		Client: mgr.GetClient(),
	}
	if err := podReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Failed to start the pod reconciler in the AKS metric provider: %v", err)
		return err
	}

	// Start the controller manager.
	//
	// Note that the controller manager will run in a separate goroutine to avoid blocking
	// the member agent.
	go func() {
		// This call will block until the context exits.
		if err := mgr.Start(ctx); err != nil {
			klog.Errorf("Failed to start the AKS metric provider controller manager: %v", err)
		}
	}()

	// Wait for the cache to sync.
	//
	// Note that this does not guarantee that any of the object changes has actually been
	// processed; it only implies that an initial state has been populated. Though for our
	// use case it might be good enough, considering that the only side effect is that
	// some exported metrics might be skewed initially (e.g., nodes/pods not being tracked).
	//
	// An alternative is to perform a list for once during the startup, which might be
	// too expensive for a large cluster.
	mgr.GetCache().WaitForCacheSync(ctx)

	return nil
}

func (p *MetricProvider) Collect(_ context.Context) metricprovider.MetricCollectionResponse {
	// Collect the non-resource metrics.
	metrics := make(map[string]float64)
	metrics[NodeCountMetric] = float64(p.nt.NodeCount())

	// Collect the resource metrics.
	resources := clusterv1beta1.ResourceUsage{}
	resources.Capacity = p.nt.TotalCapacity()
	resources.Allocatable = p.nt.TotalAllocatable()

	requested := p.pt.TotalRequested()
	available := make(corev1.ResourceList)
	for rn := range resources.Allocatable {
		left := resources.Allocatable[rn].DeepCopy()
		// In some unlikely scenarios, it could happen that, due to unavoidable
		// inconsistencies in the data collection process, the total value of a specific
		// requested resource exceeds that of the allocatable resource, as observed by
		// the metric provider; for example, the node tracker might fail to track a node
		// in time yet the some pods have been assigned to the pod and gets tracked by
		// the pod tracker. In such cases, the metric provider will report a zero
		// value for the resource; and this occurrence should get fixed in the next (few)
		// metric collection iterations.
		if left.Cmp(requested[rn]) > 0 {
			left.Sub(requested[rn])
		} else {
			left = resource.Quantity{}
		}
		available[rn] = left
	}
	resources.Available = available

	// Report the condition.
	conditions := []metav1.Condition{
		{
			Type:    MetricCollectionSucceededConditionType,
			Status:  metav1.ConditionTrue,
			Reason:  MetricCollectionSucceededReason,
			Message: MetricCollectionSucceededMessage,
		},
	}

	// Return the collection response.
	return metricprovider.MetricCollectionResponse{
		Metrics:    metrics,
		Resources:  resources,
		Conditions: conditions,
	}
}

// New returns a new AKS metric provider.
func New() *MetricProvider {
	return &MetricProvider{
		pt: trackers.NewPodTracker(),
		nt: trackers.NewNodeTracker(),
	}
}
