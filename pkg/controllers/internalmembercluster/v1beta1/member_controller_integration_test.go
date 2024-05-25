/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package v1beta1

import (
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	"go.goms.io/fleet/pkg/propertyprovider"
)

const (
	propertiesManuallyUpdatedConditionType   = "PropertiesManuallyUpdated"
	propertiesManuallyUpdatedConditionReason = "NewPropertiesPushed"
	propertiesManuallyUpdatedConditionMsg    = "Properties have been manually updated"
)

const (
	eventuallyTimeout  = time.Second * 30
	eventuallyInterval = time.Millisecond * 500
)

var (
	ignoreTimeFields    = cmpopts.IgnoreTypes(time.Time{})
	sortConditionByType = cmpopts.SortSlices(func(a, b metav1.Condition) bool {
		return a.Type < b.Type
	})
)

var _ = Describe("Test InternalMemberCluster Controller", func() {
	// Note that specs in this context run in serial, however, they might run in parallel with
	// the other contexts if parallelization is enabled.
	//
	// This is safe as the controller managers have been configured to watch only their own
	// respective namespaces.
	Context("Test setup with property provider", Ordered, func() {
		//timeStarted := time.Now()

		BeforeAll(func() {
			// Create the InternalMemberCluster object.
			imc := &clusterv1beta1.InternalMemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      member1Name,
					Namespace: member1ReservedNSName,
				},
				Spec: clusterv1beta1.InternalMemberClusterSpec{
					State: clusterv1beta1.ClusterStateJoin,
					// Use a shorter heartbeat period to improve responsiveness.
					HeartbeatPeriodSeconds: 2,
				},
			}
			Expect(hubClient.Create(ctx, imc)).Should(Succeed())

			// Report properties via the property provider.
			propertyProvider1.Update(&propertyprovider.PropertyCollectionResponse{
				Properties: map[clusterv1beta1.PropertyName]clusterv1beta1.PropertyValue{
					propertyprovider.NodeCountProperty: {
						Value:           "1",
						ObservationTime: metav1.Now(),
					},
				},
				Resources: clusterv1beta1.ResourceUsage{
					Capacity: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
					Available: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
				},
				Conditions: []metav1.Condition{
					{
						Type:    propertiesManuallyUpdatedConditionType,
						Status:  metav1.ConditionTrue,
						Reason:  propertiesManuallyUpdatedConditionReason,
						Message: propertiesManuallyUpdatedConditionMsg,
					},
				},
			})
		})

		It("Should join the cluster", func() {
			// Verify that the agent status has been updated.
			Eventually(func() error {
				imc := &clusterv1beta1.InternalMemberCluster{}
				objKey := types.NamespacedName{
					Name:      member1Name,
					Namespace: member1ReservedNSName,
				}
				if err := hubClient.Get(ctx, objKey, imc); err != nil {
					return fmt.Errorf("failed to get InternalMemberCluster: %w", err)
				}

				wantIMCStatus := clusterv1beta1.InternalMemberClusterStatus{
					Conditions: []metav1.Condition{
						{
							Type:               propertiesManuallyUpdatedConditionType,
							Status:             metav1.ConditionTrue,
							Reason:             propertiesManuallyUpdatedConditionReason,
							Message:            propertiesManuallyUpdatedConditionMsg,
							ObservedGeneration: imc.Generation,
						},
					},
					Properties: map[clusterv1beta1.PropertyName]clusterv1beta1.PropertyValue{
						propertyprovider.NodeCountProperty: {
							Value: "1",
						},
					},
					ResourceUsage: clusterv1beta1.ResourceUsage{
						Capacity: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("10Gi"),
						},
						Allocatable: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("8"),
							corev1.ResourceMemory: resource.MustParse("8Gi"),
						},
						Available: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					AgentStatus: []clusterv1beta1.AgentStatus{
						{
							Type: clusterv1beta1.MemberAgent,
							Conditions: []metav1.Condition{
								{
									Type:               string(clusterv1beta1.AgentJoined),
									Status:             metav1.ConditionTrue,
									Reason:             EventReasonInternalMemberClusterJoined,
									ObservedGeneration: imc.Generation,
								},
								{
									Type:               string(clusterv1beta1.AgentHealthy),
									Status:             metav1.ConditionTrue,
									Reason:             EventReasonInternalMemberClusterHealthy,
									ObservedGeneration: imc.Generation,
								},
							},
						},
					},
				}

				if diff := cmp.Diff(
					imc.Status, wantIMCStatus,
					ignoreTimeFields,
					sortConditionByType,
				); diff != "" {
					return fmt.Errorf("InternalMemberCluster status diff (-got, +want):\n%s", diff)
				}

				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(Succeed(), "Failed to update the agent status")
		})
	})

	// Note that specs in this context run in serial, however, they might run in parallel with
	// the other contexts if parallelization is enabled.
	//
	// This is safe as the controller managers have been configured to watch only their own
	// respective namespaces.
	Context("Test setup with no property provider", Ordered, func() {})
})
