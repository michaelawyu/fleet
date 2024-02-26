/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package aks

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	"go.goms.io/fleet/pkg/metricprovider"
	"go.goms.io/fleet/pkg/metricprovider/aks/trackers"
)

// All the test cases in this block are serial + ordered, as manipulation of nodes/pods
// in one test case will disrupt another.
var _ = Describe("aks metric provider", func() {
	Context("add a new node", Serial, Ordered, func() {
		BeforeAll(func() {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName1,
					Labels: map[string]string{
						trackers.AKSClusterNodeSKULabelName: aksNodeSKU1,
					},
				},
				Spec: corev1.NodeSpec{},
			}
			Expect(memberClient.Create(ctx, node)).To(Succeed(), "Failed to create node")

			Expect(memberClient.Get(ctx, types.NamespacedName{Name: nodeName1}, node)).To(Succeed(), "Failed to get node")
			node.Status = corev1.NodeStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.8"),
					corev1.ResourceMemory: resource.MustParse("800Mi"),
				},
			}
			Expect(memberClient.Status().Update(ctx, node)).To(Succeed(), "Failed to update node status")
		})

		It("should report corrent metrics", func() {
			expectedRes := metricprovider.MetricCollectionResponse{
				Metrics: map[string]float64{
					NodeCountMetric:       1,
					PerCPUCoreCostMetric:  0.01,
					PerGBMemoryCostMetric: 0.01,
				},
				Resources: clusterv1beta1.ResourceUsage{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("0.8"),
						corev1.ResourceMemory: resource.MustParse("800Mi"),
					},
					Available: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("0.8"),
						corev1.ResourceMemory: resource.MustParse("800Mi"),
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
			}

			Eventually(func() error {
				res := p.Collect(ctx)
				if diff := cmp.Diff(res, expectedRes); diff != "" {
					return fmt.Errorf("metric collection response (-got, +want):\n%s", diff)
				}
				return nil
			}, eventuallyDuration, eventuallyInterval).Should(BeNil())
		})

		AfterAll(func() {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName1,
				},
			}
			Expect(memberClient.Delete(ctx, node)).To(Succeed(), "Failed to delete node")

			// Wait for the node to be deleted.
			Eventually(func() error {
				if err := memberClient.Get(ctx, types.NamespacedName{Name: nodeName1}, node); !errors.IsNotFound(err) {
					return fmt.Errorf("node has not been deleted yet")
				}
				return nil
			}, eventuallyDuration, eventuallyInterval).Should(BeNil())
		})
	})
})
