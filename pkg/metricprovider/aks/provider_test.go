/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package aks

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	"go.goms.io/fleet/pkg/metricprovider"
	"go.goms.io/fleet/pkg/metricprovider/aks/trackers"
)

const (
	nodeName1 = "node-1"
	nodeName2 = "node-2"
	nodeName3 = "node-3"

	podName1 = "pod-1"
	podName2 = "pod-2"
	podName3 = "pod-3"

	containerName1 = "container-1"
	containerName2 = "container-2"
	containerName3 = "container-3"

	namespaceName1 = "work-1"
	namespaceName2 = "work-2"
)

func TestCollect(t *testing.T) {
	testCases := []struct {
		name                         string
		nodes                        []corev1.Node
		pods                         []corev1.Pod
		wantMetricCollectionResponse metricprovider.MetricCollectionResponse
	}{
		{
			name: "can report metrics",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName1,
					},
					Spec: corev1.NodeSpec{},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("16Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3.2"),
							corev1.ResourceMemory: resource.MustParse("15.2Gi"),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName2,
					},
					Spec: corev1.NodeSpec{},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("32Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("7"),
							corev1.ResourceMemory: resource.MustParse("30Gi"),
						},
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName1,
						Namespace: namespaceName1,
					},
					Spec: corev1.PodSpec{
						NodeName: nodeName1,
						Containers: []corev1.Container{
							{
								Name: containerName1,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
								},
							},
							{
								Name: containerName2,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1.5"),
										corev1.ResourceMemory: resource.MustParse("4Gi"),
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName2,
						Namespace: namespaceName1,
					},
					Spec: corev1.PodSpec{
						NodeName: nodeName2,
						Containers: []corev1.Container{
							{
								Name: containerName3,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("6"),
										corev1.ResourceMemory: resource.MustParse("16Gi"),
									},
								},
							},
						},
					},
				},
			},
			wantMetricCollectionResponse: metricprovider.MetricCollectionResponse{
				Metrics: map[string]float64{
					NodeCountMetric: 2,
				},
				Resources: clusterv1beta1.ResourceUsage{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("12"),
						corev1.ResourceMemory: resource.MustParse("48Gi"),
					},
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10.2"),
						corev1.ResourceMemory: resource.MustParse("45.2Gi"),
					},
					Available: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1.7"),
						corev1.ResourceMemory: resource.MustParse("23.2Gi"),
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:    MetricCollectionSucceededConditionType,
						Status:  metav1.ConditionTrue,
						Reason:  MetricCollectionSucceededReason,
						Message: MetricCollectionSucceededMessage,
					},
				},
			},
		},
		{
			name: "will report zero values if the requested resources exceed the allocatable resources",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName1,
					},
					Spec: corev1.NodeSpec{},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("4"),
							corev1.ResourceMemory: resource.MustParse("16Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("3.2"),
							corev1.ResourceMemory: resource.MustParse("15.2Gi"),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName2,
					},
					Spec: corev1.NodeSpec{},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("32Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("7"),
							corev1.ResourceMemory: resource.MustParse("30Gi"),
						},
					},
				},
			},
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName1,
						Namespace: namespaceName1,
					},
					Spec: corev1.PodSpec{
						NodeName: nodeName1,
						Containers: []corev1.Container{
							{
								Name: containerName1,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10"),
										corev1.ResourceMemory: resource.MustParse("20Gi"),
									},
								},
							},
							{
								Name: containerName2,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20"),
										corev1.ResourceMemory: resource.MustParse("20Gi"),
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName2,
						Namespace: namespaceName1,
					},
					Spec: corev1.PodSpec{
						NodeName: nodeName2,
						Containers: []corev1.Container{
							{
								Name: containerName3,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("20"),
										corev1.ResourceMemory: resource.MustParse("20Gi"),
									},
								},
							},
						},
					},
				},
			},
			wantMetricCollectionResponse: metricprovider.MetricCollectionResponse{
				Metrics: map[string]float64{
					NodeCountMetric: 2,
				},
				Resources: clusterv1beta1.ResourceUsage{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("12"),
						corev1.ResourceMemory: resource.MustParse("48Gi"),
					},
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10.2"),
						corev1.ResourceMemory: resource.MustParse("45.2Gi"),
					},
					Available: corev1.ResourceList{
						corev1.ResourceCPU:    resource.Quantity{},
						corev1.ResourceMemory: resource.Quantity{},
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:    MetricCollectionSucceededConditionType,
						Status:  metav1.ConditionTrue,
						Reason:  MetricCollectionSucceededReason,
						Message: MetricCollectionSucceededMessage,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Build the trackers manually for testing purposes.
			nt := trackers.NewNodeTracker()
			pt := trackers.NewPodTracker()
			for idx := range tc.nodes {
				nt.AddOrUpdate(&tc.nodes[idx])
			}
			for idx := range tc.pods {
				pt.AddOrUpdate(&tc.pods[idx])
			}

			p := &MetricProvider{
				nt: nt,
				pt: pt,
			}
			res := p.Collect(ctx)
			if diff := cmp.Diff(res, tc.wantMetricCollectionResponse); diff != "" {
				t.Fatalf("Collect() metric collection response diff (-got, +want):\n%s", diff)
			}
		})
	}
}
