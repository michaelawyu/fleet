/*
Copyright 2025 The KubeFleet Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package updaterun

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	placementv1alpha1 "go.goms.io/fleet/apis/placement/v1alpha1"
	placementv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
	"go.goms.io/fleet/pkg/utils"
	"go.goms.io/fleet/pkg/utils/condition"
)

var _ = Describe("UpdateRun execution tests", func() {
	var updateRun *placementv1beta1.ClusterStagedUpdateRun
	var crp *placementv1beta1.ClusterResourcePlacement
	var policySnapshot *placementv1beta1.ClusterSchedulingPolicySnapshot
	var updateStrategy *placementv1beta1.ClusterStagedUpdateStrategy
	var resourceBindings []*placementv1beta1.ClusterResourceBinding
	var targetClusters []*clusterv1beta1.MemberCluster
	var unscheduledCluster []*clusterv1beta1.MemberCluster
	var resourceSnapshot *placementv1beta1.ClusterResourceSnapshot
	var clusterResourceOverride *placementv1alpha1.ClusterResourceOverrideSnapshot
	var wantStatus *placementv1beta1.StagedUpdateRunStatus
	var customRegistry *prometheus.Registry

	BeforeEach(OncePerOrdered, func() {
		testUpdateRunName = "updaterun-" + utils.RandStr()
		testCRPName = "crp-" + utils.RandStr()
		testResourceSnapshotName = testCRPName + "-" + testResourceSnapshotIndex + "-snapshot"
		testUpdateStrategyName = "updatestrategy-" + utils.RandStr()
		testCROName = "cro-" + utils.RandStr()
		updateRunNamespacedName = types.NamespacedName{Name: testUpdateRunName}

		updateRun = generateTestClusterStagedUpdateRun()
		crp = generateTestClusterResourcePlacement()
		policySnapshot = generateTestClusterSchedulingPolicySnapshot(1)
		updateStrategy = generateTestClusterStagedUpdateStrategy()
		clusterResourceOverride = generateTestClusterResourceOverride()

		resourceBindings = make([]*placementv1beta1.ClusterResourceBinding, numTargetClusters+numUnscheduledClusters)
		targetClusters = make([]*clusterv1beta1.MemberCluster, numTargetClusters)
		for i := range targetClusters {
			// split the clusters into 2 regions
			region := regionEastus
			if i%2 == 0 {
				region = regionWestus
			}
			// reserse the order of the clusters by index
			targetClusters[i] = generateTestMemberCluster(numTargetClusters-1-i, "cluster-"+strconv.Itoa(i), map[string]string{"group": "prod", "region": region})
			resourceBindings[i] = generateTestClusterResourceBinding(policySnapshot.Name, targetClusters[i].Name, placementv1beta1.BindingStateScheduled)
		}

		unscheduledCluster = make([]*clusterv1beta1.MemberCluster, numUnscheduledClusters)
		for i := range unscheduledCluster {
			unscheduledCluster[i] = generateTestMemberCluster(i, "unscheduled-cluster-"+strconv.Itoa(i), map[string]string{"group": "staging"})
			// update the policySnapshot name so that these clusters are considered to-be-deleted
			resourceBindings[numTargetClusters+i] = generateTestClusterResourceBinding(policySnapshot.Name+"a", unscheduledCluster[i].Name, placementv1beta1.BindingStateUnscheduled)
		}

		var err error
		testNamespace, err = json.Marshal(corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace",
				Labels: map[string]string{
					"fleet.azure.com/name": "test-namespace",
				},
			},
		})
		Expect(err).To(Succeed())
		resourceSnapshot = generateTestClusterResourceSnapshot()

		// Set smaller wait time for testing
		stageUpdatingWaitTime = time.Second * 3
		clusterUpdatingWaitTime = time.Second * 2

		customRegistry = initializeUpdateRunMetricsRegistry()

		By("Creating a new clusterResourcePlacement")
		Expect(k8sClient.Create(ctx, crp)).To(Succeed())

		By("Creating scheduling policy snapshot")
		Expect(k8sClient.Create(ctx, policySnapshot)).To(Succeed())

		By("Setting the latest policy snapshot condition as fully scheduled")
		meta.SetStatusCondition(&policySnapshot.Status.Conditions, metav1.Condition{
			Type:               string(placementv1beta1.PolicySnapshotScheduled),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: policySnapshot.Generation,
			Reason:             "scheduled",
		})
		Expect(k8sClient.Status().Update(ctx, policySnapshot)).Should(Succeed(), "failed to update the policy snapshot condition")

		By("Creating the member clusters")
		for _, cluster := range targetClusters {
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		}
		for _, cluster := range unscheduledCluster {
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		}

		By("Creating a bunch of ClusterResourceBindings")
		for _, binding := range resourceBindings {
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		}

		By("Creating a clusterStagedUpdateStrategy")
		Expect(k8sClient.Create(ctx, updateStrategy)).To(Succeed())

		By("Creating a new resource snapshot")
		Expect(k8sClient.Create(ctx, resourceSnapshot)).To(Succeed())

		By("Creating a new cluster resource override")
		Expect(k8sClient.Create(ctx, clusterResourceOverride)).To(Succeed())
	})

	AfterEach(OncePerOrdered, func() {
		By("Deleting the clusterStagedUpdateRun")
		Expect(k8sClient.Delete(ctx, updateRun)).Should(Succeed())
		updateRun = nil

		By("Deleting the clusterResourcePlacement")
		Expect(k8sClient.Delete(ctx, crp)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		crp = nil

		By("Deleting the clusterSchedulingPolicySnapshot")
		Expect(k8sClient.Delete(ctx, policySnapshot)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		policySnapshot = nil

		By("Deleting the clusterResourceBindings")
		for _, binding := range resourceBindings {
			Expect(k8sClient.Delete(ctx, binding)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		}
		resourceBindings = nil

		By("Deleting the member clusters")
		for _, cluster := range targetClusters {
			Expect(k8sClient.Delete(ctx, cluster)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		}
		for _, cluster := range unscheduledCluster {
			Expect(k8sClient.Delete(ctx, cluster)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		}
		targetClusters, unscheduledCluster = nil, nil

		By("Deleting the clusterStagedUpdateStrategy")
		Expect(k8sClient.Delete(ctx, updateStrategy)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		updateStrategy = nil

		By("Deleting the clusterResourceSnapshot")
		Expect(k8sClient.Delete(ctx, resourceSnapshot)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		resourceSnapshot = nil

		By("Deleting the clusterResourceOverride")
		Expect(k8sClient.Delete(ctx, clusterResourceOverride)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		clusterResourceOverride = nil

		By("Checking update run status metrics are removed")
		// No metrics are emitted as all are removed after updateRun is deleted.
		validateUpdateRunMetricsEmitted(customRegistry)
		unregisterUpdateRunMetrics(customRegistry)
	})

	Context("Cluster staged update run should update clusters one by one - strategy with double afterStageTasks", Ordered, func() {
		BeforeAll(func() {
			By("Creating a new clusterStagedUpdateRun")
			Expect(k8sClient.Create(ctx, updateRun)).To(Succeed())

			By("Validating the initialization succeeded and the execution started")
			initialized := generateSucceededInitializationStatus(crp, updateRun, policySnapshot, updateStrategy, clusterResourceOverride)
			wantStatus = generateExecutionStartedStatus(updateRun, initialized)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 1st cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 1st clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-1] // cluster-9
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 1st clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 1st cluster has succeeded and 2nd cluster has started")
			wantStatus.StagesStatus[0].Clusters[0].Conditions = append(wantStatus.StagesStatus[0].Clusters[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[1].Conditions = append(wantStatus.StagesStatus[0].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 1st stage has startTime set")
			Expect(updateRun.Status.StagesStatus[0].StartTime).ShouldNot(BeNil())

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 2nd cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 2nd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-3] // cluster-7
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 2nd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 2nd cluster has succeeded and 3rd cluster has started")
			wantStatus.StagesStatus[0].Clusters[1].Conditions = append(wantStatus.StagesStatus[0].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[2].Conditions = append(wantStatus.StagesStatus[0].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 3rd cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 3rd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-5] // cluster-5
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 3rd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 3rd cluster has succeeded and 4th cluster has started")
			wantStatus.StagesStatus[0].Clusters[2].Conditions = append(wantStatus.StagesStatus[0].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[3].Conditions = append(wantStatus.StagesStatus[0].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 4th cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 4th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-7] // cluster-3
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 4th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 4th cluster has succeeded and 5th cluster has started")
			wantStatus.StagesStatus[0].Clusters[3].Conditions = append(wantStatus.StagesStatus[0].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[4].Conditions = append(wantStatus.StagesStatus[0].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 5th cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 5th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-9] // cluster-1
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 5th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 5th cluster has succeeded and stage waiting for AfterStageTasks")
			wantStatus.StagesStatus[0].Clusters[4].Conditions = append(wantStatus.StagesStatus[0].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Conditions[0] = generateFalseCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing) // The progressing condition now becomes false with waiting reason.
			wantStatus.StagesStatus[0].AfterStageTaskStatus[1].Conditions = append(wantStatus.StagesStatus[0].AfterStageTaskStatus[1].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestCreated))
			wantStatus.Conditions[1] = generateFalseCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun), generateWaitingMetric(updateRun))
		})

		It("Should complete the 1st stage after wait time passed and approval request approved and move on to the 2nd stage", func() {
			By("Validating the approvalRequest has been created")
			wantApprovalRequest := &placementv1beta1.ClusterApprovalRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: updateRun.Status.StagesStatus[0].AfterStageTaskStatus[1].ApprovalRequestName,
					Labels: map[string]string{
						placementv1beta1.TargetUpdatingStageNameLabel:   updateRun.Status.StagesStatus[0].StageName,
						placementv1beta1.TargetUpdateRunLabel:           updateRun.Name,
						placementv1beta1.IsLatestUpdateRunApprovalLabel: "true",
					},
				},
				Spec: placementv1beta1.ApprovalRequestSpec{
					TargetUpdateRun: updateRun.Name,
					TargetStage:     updateRun.Status.StagesStatus[0].StageName,
				},
			}
			validateApprovalRequestCreated(wantApprovalRequest)

			By("Approving the approvalRequest")
			approveClusterApprovalRequest(ctx, wantApprovalRequest.Name)

			By("Validating both after stage tasks have completed and 2nd stage has started")
			// Timedwait afterStageTask completed.
			wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionWaitTimeElapsed))
			// Approval afterStageTask completed.
			wantStatus.StagesStatus[0].AfterStageTaskStatus[1].Conditions = append(wantStatus.StagesStatus[0].AfterStageTaskStatus[1].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestApproved))
			// 1st stage completed, mark progressing condition reason as succeeded and add succeeded condition.
			wantStatus.StagesStatus[0].Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true)
			wantStatus.StagesStatus[0].Conditions = append(wantStatus.StagesStatus[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			// 2nd stage started.
			wantStatus.StagesStatus[1].Conditions = append(wantStatus.StagesStatus[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing))
			// 1st cluster in 2nd stage started.
			wantStatus.StagesStatus[1].Clusters[0].Conditions = append(wantStatus.StagesStatus[1].Clusters[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			wantStatus.Conditions[1] = generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 1st stage has endTime set")
			Expect(updateRun.Status.StagesStatus[0].EndTime).ShouldNot(BeNil())

			By("Validating the waitTime after stage task only completes after the wait time")
			waitStartTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].Conditions, string(placementv1beta1.StageUpdatingConditionProgressing)).LastTransitionTime.Time
			waitEndTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].AfterStageTaskStatus[0].Conditions, string(placementv1beta1.AfterStageTaskConditionWaitTimeElapsed)).LastTransitionTime.Time
			Expect(waitStartTime.Add(updateStrategy.Spec.Stages[0].AfterStageTasks[0].WaitTime.Duration).After(waitEndTime)).Should(BeFalse(),
				fmt.Sprintf("waitEndTime %v did not pass waitStartTime %v long enough, want at least %v", waitEndTime, waitStartTime, updateStrategy.Spec.Stages[0].AfterStageTasks[0].WaitTime.Duration))

			By("Validating the creation time of the approval request is before the complete time of the timedwait task")
			approvalCreateTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].AfterStageTaskStatus[1].Conditions, string(placementv1beta1.AfterStageTaskConditionApprovalRequestCreated)).LastTransitionTime.Time
			Expect(approvalCreateTime.Before(waitEndTime)).Should(BeTrue())

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 1st cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 1st clusterResourceBinding is updated to Bound")
			binding := resourceBindings[0] // cluster-0
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 1st clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 1st cluster has succeeded and 2nd cluster has started")
			wantStatus.StagesStatus[1].Clusters[0].Conditions = append(wantStatus.StagesStatus[1].Clusters[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[1].Conditions = append(wantStatus.StagesStatus[1].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 2nd stage has startTime set")
			Expect(updateRun.Status.StagesStatus[0].StartTime).ShouldNot(BeNil())

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 2nd cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 2nd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[2] // cluster-2
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 2nd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 2nd cluster has succeeded and 3rd cluster has started")
			wantStatus.StagesStatus[1].Clusters[1].Conditions = append(wantStatus.StagesStatus[1].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[2].Conditions = append(wantStatus.StagesStatus[1].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 3rd cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 3rd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[4] // cluster-4
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 3rd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 3rd cluster has succeeded and 4th cluster has started")
			wantStatus.StagesStatus[1].Clusters[2].Conditions = append(wantStatus.StagesStatus[1].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[3].Conditions = append(wantStatus.StagesStatus[1].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 4th cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 4th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[6] // cluster-6
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 4th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 4th cluster has succeeded and 5th cluster has started")
			wantStatus.StagesStatus[1].Clusters[3].Conditions = append(wantStatus.StagesStatus[1].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[4].Conditions = append(wantStatus.StagesStatus[1].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 5th cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 5th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[8] // cluster-8
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 5th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 5th cluster has succeeded and the stage waiting for AfterStageTask")
			wantStatus.StagesStatus[1].Clusters[4].Conditions = append(wantStatus.StagesStatus[1].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Conditions[0] = generateFalseCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing) // The progressing condition now becomes false with waiting reason.
			wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestCreated))
			wantStatus.Conditions[1] = generateFalseCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun), generateWaitingMetric(updateRun))
		})

		It("Should complete the 2nd stage after both after stage tasks are completed and move on to the delete stage", func() {
			By("Validating the approvalRequest has been created")
			wantApprovalRequest := &placementv1beta1.ClusterApprovalRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: updateRun.Status.StagesStatus[1].AfterStageTaskStatus[0].ApprovalRequestName,
					Labels: map[string]string{
						placementv1beta1.TargetUpdatingStageNameLabel:   updateRun.Status.StagesStatus[1].StageName,
						placementv1beta1.TargetUpdateRunLabel:           updateRun.Name,
						placementv1beta1.IsLatestUpdateRunApprovalLabel: "true",
					},
				},
				Spec: placementv1beta1.ApprovalRequestSpec{
					TargetUpdateRun: updateRun.Name,
					TargetStage:     updateRun.Status.StagesStatus[1].StageName,
				},
			}
			validateApprovalRequestCreated(wantApprovalRequest)

			By("Approving the approvalRequest")
			approveClusterApprovalRequest(ctx, wantApprovalRequest.Name)

			By("Validating the 2nd stage has completed and the delete stage has started")
			wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestApproved))
			wantStatus.StagesStatus[1].AfterStageTaskStatus[1].Conditions = append(wantStatus.StagesStatus[1].AfterStageTaskStatus[1].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionWaitTimeElapsed))
			wantStatus.StagesStatus[1].Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true)
			wantStatus.StagesStatus[1].Conditions = append(wantStatus.StagesStatus[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			wantStatus.Conditions[1] = generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)

			wantStatus.DeletionStageStatus.Conditions = append(wantStatus.DeletionStageStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing))
			for i := range wantStatus.DeletionStageStatus.Clusters {
				wantStatus.DeletionStageStatus.Clusters[i].Conditions = append(wantStatus.DeletionStageStatus.Clusters[i].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			}
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 2nd stage has endTime set")
			Expect(updateRun.Status.StagesStatus[1].EndTime).ShouldNot(BeNil())

			By("Validating the waitTime after stage task only completes after the wait time")
			waitStartTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[1].Conditions, string(placementv1beta1.StageUpdatingConditionProgressing)).LastTransitionTime.Time
			waitEndTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[1].AfterStageTaskStatus[1].Conditions, string(placementv1beta1.AfterStageTaskConditionWaitTimeElapsed)).LastTransitionTime.Time
			Expect(waitStartTime.Add(updateStrategy.Spec.Stages[1].AfterStageTasks[1].WaitTime.Duration).After(waitEndTime)).Should(BeFalse(),
				fmt.Sprintf("waitEndTime %v did not pass waitStartTime %v long enough, want at least %v", waitEndTime, waitStartTime, updateStrategy.Spec.Stages[1].AfterStageTasks[1].WaitTime.Duration))

			By("Validating the creation time of the approval request is before the complete time of the timedwait task")
			approvalCreateTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[1].AfterStageTaskStatus[0].Conditions, string(placementv1beta1.AfterStageTaskConditionApprovalRequestCreated)).LastTransitionTime.Time
			Expect(approvalCreateTime.Before(waitEndTime)).Should(BeTrue())

			By("Validating the approvalRequest has ApprovalAccepted status")
			Eventually(func() (bool, error) {
				var approvalRequest placementv1beta1.ClusterApprovalRequest
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, &approvalRequest); err != nil {
					return false, err
				}
				return condition.IsConditionStatusTrue(meta.FindStatusCondition(approvalRequest.Status.Conditions, string(placementv1beta1.ApprovalRequestConditionApprovalAccepted)), approvalRequest.Generation), nil
			}, timeout, interval).Should(BeTrue(), "failed to validate the approvalRequest approval accepted")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should delete all the clusterResourceBindings in the delete stage and complete the update run", func() {
			By("Validating the to-be-deleted bindings are all deleted")
			Eventually(func() error {
				for i := numTargetClusters; i < numTargetClusters+numUnscheduledClusters; i++ {
					binding := &placementv1beta1.ClusterResourceBinding{}
					err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceBindings[i].Name}, binding)
					if err == nil {
						return fmt.Errorf("binding %s is not deleted", binding.Name)
					}
					if !apierrors.IsNotFound(err) {
						return fmt.Errorf("Get binding %s does not return a not-found error: %w", binding.Name, err)
					}
				}
				return nil
			}, timeout, interval).Should(Succeed(), "failed to validate the deletion of the to-be-deleted bindings")

			By("Validating the delete stage and the clusterStagedUpdateRun has completed")
			for i := range wantStatus.DeletionStageStatus.Clusters {
				wantStatus.DeletionStageStatus.Clusters[i].Conditions = append(wantStatus.DeletionStageStatus.Clusters[i].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			}
			// Mark the stage progressing condition as false with succeeded reason and add succeeded condition.
			wantStatus.DeletionStageStatus.Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true)
			wantStatus.DeletionStageStatus.Conditions = append(wantStatus.DeletionStageStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			// Mark updateRun progressing condition as false with succeeded reason and add succeeded condition.
			wantStatus.Conditions[1] = generateFalseProgressingCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing, true)
			wantStatus.Conditions = append(wantStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionSucceeded))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun), generateSucceededMetric(updateRun))
		})
	})

	Context("Cluster staged update run should update clusters one by one - strategy with single afterStageTask", Ordered, func() {
		BeforeAll(func() {
			By("Updating the strategy to have single afterStageTask")
			updateStrategy.Spec.Stages[0].AfterStageTasks = updateStrategy.Spec.Stages[0].AfterStageTasks[:1]
			updateStrategy.Spec.Stages[1].AfterStageTasks = updateStrategy.Spec.Stages[1].AfterStageTasks[:1]
			Expect(k8sClient.Update(ctx, updateStrategy)).To(Succeed())

			By("Creating a new clusterStagedUpdateRun")
			Expect(k8sClient.Create(ctx, updateRun)).To(Succeed())

			By("Validating the initialization succeeded and the execution started")
			initialized := generateSucceededInitializationStatus(crp, updateRun, policySnapshot, updateStrategy, clusterResourceOverride)
			wantStatus = generateExecutionStartedStatus(updateRun, initialized)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 1st cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 1st clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-1] // cluster-9
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 1st clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 1st cluster has succeeded and 2nd cluster has started")
			wantStatus.StagesStatus[0].Clusters[0].Conditions = append(wantStatus.StagesStatus[0].Clusters[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[1].Conditions = append(wantStatus.StagesStatus[0].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 1st stage has startTime set")
			Expect(updateRun.Status.StagesStatus[0].StartTime).ShouldNot(BeNil())

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 2nd cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 2nd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-3] // cluster-7
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 2nd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 2nd cluster has succeeded and 3rd cluster has started")
			wantStatus.StagesStatus[0].Clusters[1].Conditions = append(wantStatus.StagesStatus[0].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[2].Conditions = append(wantStatus.StagesStatus[0].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 3rd cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 3rd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-5] // cluster-5
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 3rd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 3rd cluster has succeeded and 4th cluster has started")
			wantStatus.StagesStatus[0].Clusters[2].Conditions = append(wantStatus.StagesStatus[0].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[3].Conditions = append(wantStatus.StagesStatus[0].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 4th cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 4th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-7] // cluster-3
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 4th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 4th cluster has succeeded and 5th cluster has started")
			wantStatus.StagesStatus[0].Clusters[3].Conditions = append(wantStatus.StagesStatus[0].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[4].Conditions = append(wantStatus.StagesStatus[0].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		It("Should mark the 5th cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 5th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-9] // cluster-1
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 5th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 5th cluster has succeeded and stage waiting for AfterStageTasks")
			wantStatus.StagesStatus[0].Clusters[4].Conditions = append(wantStatus.StagesStatus[0].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Conditions[0] = generateFalseCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing) // The progressing condition now becomes false with waiting reason.
			wantStatus.Conditions[1] = generateFalseCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun), generateWaitingMetric(updateRun))
		})

		It("Should complete the 1st stage after wait time passed and move on to the 2nd stage", func() {
			By("Validating the TimedWait after stage task has completed and 2nd stage has started")
			// Timedwait afterStageTask completed.
			wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionWaitTimeElapsed))
			// 1st stage completed.
			wantStatus.StagesStatus[0].Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true)
			wantStatus.StagesStatus[0].Conditions = append(wantStatus.StagesStatus[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			// 2nd stage started.
			wantStatus.StagesStatus[1].Conditions = append(wantStatus.StagesStatus[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing))
			// 1st cluster in 2nd stage started.
			wantStatus.StagesStatus[1].Clusters[0].Conditions = append(wantStatus.StagesStatus[1].Clusters[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			wantStatus.Conditions[1] = generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 1st stage has endTime set")
			Expect(updateRun.Status.StagesStatus[0].EndTime).ShouldNot(BeNil())

			By("Validating the waitTime after stage task only completes after the wait time")
			waitStartTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].Conditions, string(placementv1beta1.StageUpdatingConditionProgressing)).LastTransitionTime.Time
			waitEndTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].AfterStageTaskStatus[0].Conditions, string(placementv1beta1.AfterStageTaskConditionWaitTimeElapsed)).LastTransitionTime.Time
			Expect(waitStartTime.Add(updateStrategy.Spec.Stages[0].AfterStageTasks[0].WaitTime.Duration).After(waitEndTime)).Should(BeFalse(),
				fmt.Sprintf("waitEndTime %v did not pass waitStartTime %v long enough, want at least %v", waitEndTime, waitStartTime, updateStrategy.Spec.Stages[0].AfterStageTasks[0].WaitTime.Duration))

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 1st cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 1st clusterResourceBinding is updated to Bound")
			binding := resourceBindings[0] // cluster-0
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 1st clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 1st cluster has succeeded and 2nd cluster has started")
			wantStatus.StagesStatus[1].Clusters[0].Conditions = append(wantStatus.StagesStatus[1].Clusters[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[1].Conditions = append(wantStatus.StagesStatus[1].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 2nd stage has startTime set")
			Expect(updateRun.Status.StagesStatus[0].StartTime).ShouldNot(BeNil())

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 2nd cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 2nd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[2] // cluster-2
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 2nd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 2nd cluster has succeeded and 3rd cluster has started")
			wantStatus.StagesStatus[1].Clusters[1].Conditions = append(wantStatus.StagesStatus[1].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[2].Conditions = append(wantStatus.StagesStatus[1].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 3rd cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 3rd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[4] // cluster-4
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 3rd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 3rd cluster has succeeded and 4th cluster has started")
			wantStatus.StagesStatus[1].Clusters[2].Conditions = append(wantStatus.StagesStatus[1].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[3].Conditions = append(wantStatus.StagesStatus[1].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 4th cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 4th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[6] // cluster-6
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 4th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 4th cluster has succeeded and 5th cluster has started")
			wantStatus.StagesStatus[1].Clusters[3].Conditions = append(wantStatus.StagesStatus[1].Clusters[3].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Clusters[4].Conditions = append(wantStatus.StagesStatus[1].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should mark the 5th cluster in the 2nd stage as succeeded after marking the binding available", func() {
			By("Validating the 5th clusterResourceBinding is updated to Bound")
			binding := resourceBindings[8] // cluster-8
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 1)

			By("Updating the 5th clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 5th cluster has succeeded and the stage waiting for AfterStageTask")
			wantStatus.StagesStatus[1].Clusters[4].Conditions = append(wantStatus.StagesStatus[1].Clusters[4].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[1].Conditions[0] = generateFalseCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing) // The progressing condition now becomes false with waiting reason.
			wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestCreated))
			wantStatus.Conditions[1] = generateFalseCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun), generateWaitingMetric(updateRun))
		})

		It("Should complete the 2nd stage after the after stage task is completed and move on to the delete stage", func() {
			By("Validating the approvalRequest has been created")
			wantApprovalRequest := &placementv1beta1.ClusterApprovalRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: updateRun.Status.StagesStatus[1].AfterStageTaskStatus[0].ApprovalRequestName,
					Labels: map[string]string{
						placementv1beta1.TargetUpdatingStageNameLabel:   updateRun.Status.StagesStatus[1].StageName,
						placementv1beta1.TargetUpdateRunLabel:           updateRun.Name,
						placementv1beta1.IsLatestUpdateRunApprovalLabel: "true",
					},
				},
				Spec: placementv1beta1.ApprovalRequestSpec{
					TargetUpdateRun: updateRun.Name,
					TargetStage:     updateRun.Status.StagesStatus[1].StageName,
				},
			}
			validateApprovalRequestCreated(wantApprovalRequest)

			By("Approving the approvalRequest")
			approveClusterApprovalRequest(ctx, wantApprovalRequest.Name)

			By("Validating the 2nd stage has completed and the delete stage has started")
			wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[1].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestApproved))
			wantStatus.StagesStatus[1].Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true)
			wantStatus.StagesStatus[1].Conditions = append(wantStatus.StagesStatus[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))

			wantStatus.DeletionStageStatus.Conditions = append(wantStatus.DeletionStageStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing))
			for i := range wantStatus.DeletionStageStatus.Clusters {
				wantStatus.DeletionStageStatus.Clusters[i].Conditions = append(wantStatus.DeletionStageStatus.Clusters[i].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			}
			wantStatus.Conditions[1] = generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 2nd stage has endTime set")
			Expect(updateRun.Status.StagesStatus[1].EndTime).ShouldNot(BeNil())

			By("Validating the approvalRequest has ApprovalAccepted status")
			Eventually(func() (bool, error) {
				var approvalRequest placementv1beta1.ClusterApprovalRequest
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, &approvalRequest); err != nil {
					return false, err
				}
				return condition.IsConditionStatusTrue(meta.FindStatusCondition(approvalRequest.Status.Conditions, string(placementv1beta1.ApprovalRequestConditionApprovalAccepted)), approvalRequest.Generation), nil
			}, timeout, interval).Should(BeTrue(), "failed to validate the approvalRequest approval accepted")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun))
		})

		It("Should delete all the clusterResourceBindings in the delete stage and complete the update run", func() {
			By("Validating the to-be-deleted bindings are all deleted")
			Eventually(func() error {
				for i := numTargetClusters; i < numTargetClusters+numUnscheduledClusters; i++ {
					binding := &placementv1beta1.ClusterResourceBinding{}
					err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceBindings[i].Name}, binding)
					if err == nil {
						return fmt.Errorf("binding %s is not deleted", binding.Name)
					}
					if !apierrors.IsNotFound(err) {
						return fmt.Errorf("Get binding %s does not return a not-found error: %w", binding.Name, err)
					}
				}
				return nil
			}, timeout, interval).Should(Succeed(), "failed to validate the deletion of the to-be-deleted bindings")

			By("Validating the delete stage and the clusterStagedUpdateRun has completed")
			for i := range wantStatus.DeletionStageStatus.Clusters {
				wantStatus.DeletionStageStatus.Clusters[i].Conditions = append(wantStatus.DeletionStageStatus.Clusters[i].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			}
			wantStatus.DeletionStageStatus.Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true)
			wantStatus.DeletionStageStatus.Conditions = append(wantStatus.DeletionStageStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			wantStatus.Conditions[1] = generateFalseProgressingCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing, true)
			wantStatus.Conditions = append(wantStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionSucceeded))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateWaitingMetric(updateRun), generateProgressingMetric(updateRun), generateSucceededMetric(updateRun))
		})
	})

	Context("Cluster staged update run should abort the execution within a failed updating stage", Ordered, func() {
		var oldUpdateRunStuckThreshold time.Duration
		BeforeAll(func() {
			// Set the updateRunStuckThreshold to 1 second for this test.
			oldUpdateRunStuckThreshold = updateRunStuckThreshold
			updateRunStuckThreshold = 1 * time.Second

			By("Creating a new clusterStagedUpdateRun")
			Expect(k8sClient.Create(ctx, updateRun)).To(Succeed())

			By("Validating the initialization succeeded and the execution started")
			initialized := generateSucceededInitializationStatus(crp, updateRun, policySnapshot, updateStrategy, clusterResourceOverride)
			wantStatus = generateExecutionStartedStatus(updateRun, initialized)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun))
		})

		AfterAll(func() {
			// Restore the updateRunStuckThreshold to the original value.
			updateRunStuckThreshold = oldUpdateRunStuckThreshold
		})

		It("Should keep waiting for the 1st cluster while it's not available", func() {
			By("Validating the 1st clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-1] // cluster-9
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 1st clusterResourceBinding to ApplyFailed")
			meta.SetStatusCondition(&binding.Status.Conditions, generateFalseCondition(binding, placementv1beta1.ResourceBindingApplied))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the updateRun is stuck in the 1st cluster of the 1st stage")
			wantStatus.Conditions[1] = generateFalseCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			wantStatus.Conditions[1].Reason = condition.UpdateRunStuckReason
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")
			validateClusterStagedUpdateRunStatusConsistently(ctx, updateRun, wantStatus, "")
		})

		It("Should emit stuck status metrics after time waiting for the 1st cluster reaches threshold", func() {
			By("Checking update run stuck metrics is emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun), generateStuckMetric(updateRun))
		})

		It("Should abort the execution if the binding has unexpected state", func() {
			By("Validating the 1st clusterResourceBinding is updated to Bound")
			binding := resourceBindings[numTargetClusters-1] // cluster-9
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 1st clusterResourceBinding's state to Scheduled (from Bound)")
			binding.Spec.State = placementv1beta1.BindingStateScheduled
			Expect(k8sClient.Update(ctx, binding)).Should(Succeed(), "failed to update the binding state")

			By("Validating the updateRun has failed")
			wantStatus.StagesStatus[0].Clusters[0].Conditions = append(wantStatus.StagesStatus[0].Clusters[0].Conditions, generateFalseCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, false)
			wantStatus.StagesStatus[0].Conditions = append(wantStatus.StagesStatus[0].Conditions, generateFalseCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			wantStatus.Conditions[1] = generateFalseProgressingCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing, false)
			wantStatus.Conditions = append(wantStatus.Conditions, generateFalseCondition(updateRun, placementv1beta1.StagedUpdateRunConditionSucceeded))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Checking update run status metrics are emitted")
			validateUpdateRunMetricsEmitted(customRegistry, generateProgressingMetric(updateRun), generateStuckMetric(updateRun), generateFailedMetric(updateRun))
		})
	})
})

var _ = Describe("UpdateRun execution tests - delete ClusterApprovalRequest, don't recreate", func() {
	var updateRun *placementv1beta1.ClusterStagedUpdateRun
	var crp *placementv1beta1.ClusterResourcePlacement
	var policySnapshot *placementv1beta1.ClusterSchedulingPolicySnapshot
	var updateStrategy *placementv1beta1.ClusterStagedUpdateStrategy
	var resourceBindings []*placementv1beta1.ClusterResourceBinding
	var targetClusters []*clusterv1beta1.MemberCluster
	var resourceSnapshot *placementv1beta1.ClusterResourceSnapshot
	var wantStatus *placementv1beta1.StagedUpdateRunStatus

	BeforeEach(OncePerOrdered, func() {
		testUpdateRunName = "updaterun-" + utils.RandStr()
		testCRPName = "crp-" + utils.RandStr()
		testResourceSnapshotName = testCRPName + "-" + testResourceSnapshotIndex + "-snapshot"
		testUpdateStrategyName = "updatestrategy-" + utils.RandStr()
		testCROName = "cro-" + utils.RandStr()
		updateRunNamespacedName = types.NamespacedName{Name: testUpdateRunName}

		updateRun = generateTestClusterStagedUpdateRun()
		crp = generateTestClusterResourcePlacement()
		policySnapshot = &placementv1beta1.ClusterSchedulingPolicySnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf(placementv1beta1.PolicySnapshotNameFmt, testCRPName, 1),
				Labels: map[string]string{
					"kubernetes-fleet.io/parent-CRP":         testCRPName,
					"kubernetes-fleet.io/is-latest-snapshot": "true",
					"kubernetes-fleet.io/policy-index":       strconv.Itoa(1),
				},
				Annotations: map[string]string{
					"kubernetes-fleet.io/number-of-clusters": strconv.Itoa(3),
				},
			},
			Spec: placementv1beta1.SchedulingPolicySnapshotSpec{
				Policy: &placementv1beta1.PlacementPolicy{
					PlacementType: placementv1beta1.PickNPlacementType,
				},
				PolicyHash: []byte("hash"),
			},
		}
		updateStrategy = &placementv1beta1.ClusterStagedUpdateStrategy{
			ObjectMeta: metav1.ObjectMeta{
				Name: testUpdateStrategyName,
			},
			Spec: placementv1beta1.StagedUpdateStrategySpec{
				Stages: []placementv1beta1.StageConfig{
					{
						Name: "stage1",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"group":  "prod",
								"region": "eastus",
							},
						},
						SortingLabelKey: ptr.To("index"),
						AfterStageTasks: []placementv1beta1.AfterStageTask{
							{
								Type: placementv1beta1.AfterStageTaskTypeApproval,
							},
							{
								Type: placementv1beta1.AfterStageTaskTypeTimedWait,
								WaitTime: &metav1.Duration{
									// Set a large wait time to approve, delete the approval request
									// and trigger an update run reconcile after time elapses.
									Duration: time.Second * 90,
								},
							},
						},
					},
				},
			},
		}

		resourceBindings = make([]*placementv1beta1.ClusterResourceBinding, 3)
		targetClusters = make([]*clusterv1beta1.MemberCluster, 3)
		region := regionEastus
		for i := range targetClusters {
			targetClusters[i] = generateTestMemberCluster(i, "cluster-"+strconv.Itoa(i), map[string]string{"group": "prod", "region": region})
			resourceBindings[i] = generateTestClusterResourceBinding(policySnapshot.Name, targetClusters[i].Name, placementv1beta1.BindingStateScheduled)
		}

		var err error
		testNamespace, err = json.Marshal(corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace",
				Labels: map[string]string{
					"fleet.azure.com/name": "test-namespace",
				},
			},
		})
		Expect(err).To(Succeed())
		resourceSnapshot = generateTestClusterResourceSnapshot()

		// Set smaller wait time for testing
		stageUpdatingWaitTime = time.Second * 3
		clusterUpdatingWaitTime = time.Second * 2

		By("Creating a new clusterResourcePlacement")
		Expect(k8sClient.Create(ctx, crp)).To(Succeed())

		By("Creating scheduling policy snapshot")
		Expect(k8sClient.Create(ctx, policySnapshot)).To(Succeed())

		By("Setting the latest policy snapshot condition as fully scheduled")
		meta.SetStatusCondition(&policySnapshot.Status.Conditions, metav1.Condition{
			Type:               string(placementv1beta1.PolicySnapshotScheduled),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: policySnapshot.Generation,
			Reason:             "scheduled",
		})
		Expect(k8sClient.Status().Update(ctx, policySnapshot)).Should(Succeed(), "failed to update the policy snapshot condition")

		By("Creating the member clusters")
		for _, cluster := range targetClusters {
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		}

		By("Creating a bunch of ClusterResourceBindings")
		for _, binding := range resourceBindings {
			Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		}

		By("Creating a clusterStagedUpdateStrategy")
		Expect(k8sClient.Create(ctx, updateStrategy)).To(Succeed())

		By("Creating a new resource snapshot")
		Expect(k8sClient.Create(ctx, resourceSnapshot)).To(Succeed())
	})

	AfterEach(OncePerOrdered, func() {
		By("Deleting the clusterStagedUpdateRun")
		Expect(k8sClient.Delete(ctx, updateRun)).Should(Succeed())
		updateRun = nil

		By("Deleting the clusterResourcePlacement")
		Expect(k8sClient.Delete(ctx, crp)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		crp = nil

		By("Deleting the clusterSchedulingPolicySnapshot")
		Expect(k8sClient.Delete(ctx, policySnapshot)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		policySnapshot = nil

		By("Deleting the clusterResourceBindings")
		for _, binding := range resourceBindings {
			Expect(k8sClient.Delete(ctx, binding)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		}
		resourceBindings = nil

		By("Deleting the member clusters")
		for _, cluster := range targetClusters {
			Expect(k8sClient.Delete(ctx, cluster)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		}
		targetClusters = nil

		By("Deleting the clusterStagedUpdateStrategy")
		Expect(k8sClient.Delete(ctx, updateStrategy)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		updateStrategy = nil

		By("Deleting the clusterResourceSnapshot")
		Expect(k8sClient.Delete(ctx, resourceSnapshot)).Should(SatisfyAny(Succeed(), utils.NotFoundMatcher{}))
		resourceSnapshot = nil
	})

	Context("Cluster staged update run should update clusters one by one - strategy with double afterStageTasks", Ordered, func() {
		BeforeAll(func() {
			By("Creating a new clusterStagedUpdateRun")
			Expect(k8sClient.Create(ctx, updateRun)).To(Succeed())

			By("Validating the initialization succeeded and the execution started")
			status := &placementv1beta1.StagedUpdateRunStatus{
				PolicySnapshotIndexUsed:      policySnapshot.Labels[placementv1beta1.PolicyIndexLabel],
				PolicyObservedClusterCount:   3,
				ApplyStrategy:                crp.Spec.Strategy.ApplyStrategy.DeepCopy(),
				StagedUpdateStrategySnapshot: &updateStrategy.Spec,
				StagesStatus: []placementv1beta1.StageUpdatingStatus{
					{
						StageName: "stage1",
						Clusters: []placementv1beta1.ClusterUpdatingStatus{
							{ClusterName: "cluster-0"},
							{ClusterName: "cluster-1"},
							{ClusterName: "cluster-2"},
						},
					},
				},
				DeletionStageStatus: &placementv1beta1.StageUpdatingStatus{
					StageName: "kubernetes-fleet.io/deleteStage",
					Clusters:  []placementv1beta1.ClusterUpdatingStatus{},
				},
				Conditions: []metav1.Condition{
					// initialization should succeed!
					generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionInitialized),
				},
			}
			for i := range status.StagesStatus {
				var tasks []placementv1beta1.AfterStageTaskStatus
				for _, task := range updateStrategy.Spec.Stages[i].AfterStageTasks {
					taskStatus := placementv1beta1.AfterStageTaskStatus{Type: task.Type}
					if task.Type == placementv1beta1.AfterStageTaskTypeApproval {
						taskStatus.ApprovalRequestName = updateRun.Name + "-" + status.StagesStatus[i].StageName
					}
					tasks = append(tasks, taskStatus)
				}
				status.StagesStatus[i].AfterStageTaskStatus = tasks
			}
			initialized := status
			wantStatus = generateExecutionStartedStatus(updateRun, initialized)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")
		})

		It("Should mark the 1st cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 1st clusterResourceBinding is updated to Bound")
			binding := resourceBindings[0] // cluster-0
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 1st clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 1st cluster has succeeded and 2nd cluster has started")
			wantStatus.StagesStatus[0].Clusters[0].Conditions = append(wantStatus.StagesStatus[0].Clusters[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[1].Conditions = append(wantStatus.StagesStatus[0].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 1st stage has startTime set")
			Expect(updateRun.Status.StagesStatus[0].StartTime).ShouldNot(BeNil())
		})

		It("Should mark the 2nd cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 2nd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[1] // cluster-1
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 2nd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 2nd cluster has succeeded and 3rd cluster has started")
			wantStatus.StagesStatus[0].Clusters[1].Conditions = append(wantStatus.StagesStatus[0].Clusters[1].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Clusters[2].Conditions = append(wantStatus.StagesStatus[0].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionStarted))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")
		})

		It("Should mark the 3rd cluster in the 1st stage as succeeded after marking the binding available", func() {
			By("Validating the 3rd clusterResourceBinding is updated to Bound")
			binding := resourceBindings[2] // cluster-3
			validateBindingState(ctx, binding, resourceSnapshot.Name, updateRun, 0)

			By("Updating the 3rd clusterResourceBinding to Available")
			meta.SetStatusCondition(&binding.Status.Conditions, generateTrueCondition(binding, placementv1beta1.ResourceBindingAvailable))
			Expect(k8sClient.Status().Update(ctx, binding)).Should(Succeed(), "failed to update the binding status")

			By("Validating the 3rd cluster has succeeded and 4th cluster has started")
			wantStatus.StagesStatus[0].Clusters[2].Conditions = append(wantStatus.StagesStatus[0].Clusters[2].Conditions, generateTrueCondition(updateRun, placementv1beta1.ClusterUpdatingConditionSucceeded))
			wantStatus.StagesStatus[0].Conditions[0] = generateFalseCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing) // The progressing condition now becomes false with waiting reason.
			wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestCreated))
			wantStatus.Conditions[1] = generateFalseCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing)
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")
		})

		It("Should complete the 1st stage after wait time passed and approval request approved", func() {
			By("Validating the approvalRequest has been created")
			approvalRequest := &placementv1beta1.ClusterApprovalRequest{}
			wantApprovalRequest := &placementv1beta1.ClusterApprovalRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: updateRun.Status.StagesStatus[0].AfterStageTaskStatus[0].ApprovalRequestName,
					Labels: map[string]string{
						placementv1beta1.TargetUpdatingStageNameLabel:   updateRun.Status.StagesStatus[0].StageName,
						placementv1beta1.TargetUpdateRunLabel:           updateRun.Name,
						placementv1beta1.IsLatestUpdateRunApprovalLabel: "true",
					},
				},
				Spec: placementv1beta1.ApprovalRequestSpec{
					TargetUpdateRun: updateRun.Name,
					TargetStage:     updateRun.Status.StagesStatus[0].StageName,
				},
			}
			validateApprovalRequestCreated(wantApprovalRequest)

			By("Deleting the approvalRequest")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, approvalRequest)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, approvalRequest)).Should(Succeed())

			By("Validating the approvalRequest has been recreated immediately")
			validateApprovalRequestCreated(wantApprovalRequest)

			By("Approving the approvalRequest")
			approveClusterApprovalRequest(ctx, wantApprovalRequest.Name)

			By("Check the updateRun status")
			wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions = append(wantStatus.StagesStatus[0].AfterStageTaskStatus[0].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionApprovalRequestApproved))
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Deleting the approvalRequest")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, approvalRequest)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, approvalRequest)).Should(Succeed(), "failed to delete the approvalRequest")

			By("Validating the approvalRequest has not been recreated")
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, approvalRequest))
			}, timeout, interval).Should(BeTrue(), "failed to ensure the approvalRequest is not recreated")
			Consistently(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, approvalRequest))
			}, timeout, interval).Should(BeTrue(), "failed to ensure the approvalRequest is not recreated")

			By("Check the updateRun status to ensure the waitTime elapsed condition is not set")
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")

			By("Validating the 1st stage has completed")
			wantStatus.StagesStatus[0].AfterStageTaskStatus[1].Conditions = append(wantStatus.StagesStatus[0].AfterStageTaskStatus[1].Conditions,
				generateTrueCondition(updateRun, placementv1beta1.AfterStageTaskConditionWaitTimeElapsed))
			wantStatus.StagesStatus[0].Conditions[0] = generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true)
			wantStatus.StagesStatus[0].Conditions = append(wantStatus.StagesStatus[0].Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			// Mark the deletion stage progressing condition as false with succeeded reason and add succeeded condition.
			wantStatus.DeletionStageStatus.Conditions = append(wantStatus.DeletionStageStatus.Conditions, generateFalseProgressingCondition(updateRun, placementv1beta1.StageUpdatingConditionProgressing, true))
			wantStatus.DeletionStageStatus.Conditions = append(wantStatus.DeletionStageStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StageUpdatingConditionSucceeded))
			// Mark updateRun progressing condition as false with succeeded reason and add succeeded condition.
			wantStatus.Conditions[1] = generateFalseProgressingCondition(updateRun, placementv1beta1.StagedUpdateRunConditionProgressing, true)
			wantStatus.Conditions = append(wantStatus.Conditions, generateTrueCondition(updateRun, placementv1beta1.StagedUpdateRunConditionSucceeded))
			// Need to have a longer wait time for the test to pass, because of the long wait time specified in the update strategy.
			timeout = time.Second * 90
			validateClusterStagedUpdateRunStatus(ctx, updateRun, wantStatus, "")
			// Reset the timeout to the default value.
			timeout = time.Second * 10

			By("Validating the 1st stage has endTime set")
			Expect(updateRun.Status.StagesStatus[0].EndTime).ShouldNot(BeNil())

			By("Validating the waitTime after stage task only completes after the wait time")
			waitStartTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].Conditions, string(placementv1beta1.StageUpdatingConditionProgressing)).LastTransitionTime.Time
			waitEndTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].AfterStageTaskStatus[1].Conditions, string(placementv1beta1.AfterStageTaskConditionWaitTimeElapsed)).LastTransitionTime.Time
			Expect(waitStartTime.Add(updateStrategy.Spec.Stages[0].AfterStageTasks[1].WaitTime.Duration).After(waitEndTime)).Should(BeFalse(),
				fmt.Sprintf("waitEndTime %v did not pass waitStartTime %v long enough, want at least %v", waitEndTime, waitStartTime, updateStrategy.Spec.Stages[0].AfterStageTasks[1].WaitTime.Duration))

			By("Validating the creation time of the approval request is before the complete time of the timedwait task")
			approvalCreateTime := meta.FindStatusCondition(updateRun.Status.StagesStatus[0].AfterStageTaskStatus[0].Conditions, string(placementv1beta1.AfterStageTaskConditionApprovalRequestCreated)).LastTransitionTime.Time
			Expect(approvalCreateTime.Before(waitEndTime)).Should(BeTrue())

			By("Validating the approvalRequest has not been recreated")
			Consistently(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, approvalRequest))
			}, timeout, interval).Should(BeTrue(), "failed to ensure the approvalRequest is not recreated")
		})
	})
})

func validateBindingState(ctx context.Context, binding *placementv1beta1.ClusterResourceBinding, resourceSnapshotName string, updateRun *placementv1beta1.ClusterStagedUpdateRun, stage int) {
	Eventually(func() error {
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: binding.Name}, binding); err != nil {
			return err
		}

		if binding.Spec.State != placementv1beta1.BindingStateBound {
			return fmt.Errorf("binding %s is not in Bound state, got %s", binding.Name, binding.Spec.State)
		}
		if binding.Spec.ResourceSnapshotName != resourceSnapshotName {
			return fmt.Errorf("binding %s has different resourceSnapshot name, got %s, want %s", binding.Name, binding.Spec.ResourceSnapshotName, resourceSnapshotName)
		}
		if diff := cmp.Diff(binding.Spec.ResourceOverrideSnapshots, updateRun.Status.StagesStatus[stage].Clusters[0].ResourceOverrideSnapshots); diff != "" {
			return fmt.Errorf("binding %s has different resourceOverrideSnapshots (-want +got):\n%s", binding.Name, diff)
		}
		if diff := cmp.Diff(binding.Spec.ClusterResourceOverrideSnapshots, updateRun.Status.StagesStatus[stage].Clusters[0].ClusterResourceOverrideSnapshots); diff != "" {
			return fmt.Errorf("binding %s has different clusterResourceOverrideSnapshots(-want +got):\n%s", binding.Name, diff)
		}
		if diff := cmp.Diff(binding.Spec.ApplyStrategy, updateRun.Status.ApplyStrategy); diff != "" {
			return fmt.Errorf("binding %s has different applyStrategy (-want +got):\n%s", binding.Name, diff)
		}

		rolloutStartedCond := binding.GetCondition(string(placementv1beta1.ResourceBindingRolloutStarted))
		if !condition.IsConditionStatusTrue(rolloutStartedCond, binding.Generation) {
			return fmt.Errorf("binding %s does not have RolloutStarted condition", binding.Name)
		}
		return nil
	}, timeout, interval).Should(Succeed(), "failed to validate the binding state")
}

func approveClusterApprovalRequest(ctx context.Context, approvalRequestName string) {
	Eventually(func() error {
		var approvalRequest placementv1beta1.ClusterApprovalRequest
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: approvalRequestName}, &approvalRequest); err != nil {
			return err
		}
		meta.SetStatusCondition(&approvalRequest.Status.Conditions, generateTrueCondition(&approvalRequest, placementv1beta1.ApprovalRequestConditionApproved))
		return k8sClient.Status().Update(ctx, &approvalRequest)
	}, timeout, interval).Should(Succeed(), "failed to approve the approvalRequest")
}

func validateApprovalRequestCreated(wantApprovalRequest *placementv1beta1.ClusterApprovalRequest) {
	approvalRequest := &placementv1beta1.ClusterApprovalRequest{}
	Eventually(func() error {
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: wantApprovalRequest.Name}, approvalRequest); err != nil {
			return err
		}
		if diff := cmp.Diff(wantApprovalRequest.Spec, approvalRequest.Spec); diff != "" {
			return fmt.Errorf("approvalRequest has different spec (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(wantApprovalRequest.Labels, approvalRequest.Labels); diff != "" {
			return fmt.Errorf("approvalRequest has different labels (-want +got):\n%s", diff)
		}
		return nil
	}, timeout, interval).Should(Succeed(), "failed to validate the approvalRequest")
}
