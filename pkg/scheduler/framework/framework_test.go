/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package framework

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
	"go.goms.io/fleet/pkg/scheduler/framework/parallelizer"
)

const (
	crpName            = "test-placement"
	policyName         = "test-policy"
	altPolicyName      = "another-test-policy"
	bindingName        = "test-binding"
	altBindingName     = "another-test-binding"
	clusterName        = "bravelion"
	altClusterName     = "smartcat"
	anotherClusterName = "singingbutterfly"
)

var (
	ignoreObjectMetaResourceVersionField = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")
	ignoreObjectMetaNameField            = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Name")
	ignoredCondFields                    = cmpopts.IgnoreFields(metav1.Condition{}, "LastTransitionTime")
	ignoredStatusFields                  = cmpopts.IgnoreFields(Status{}, "reasons", "err")
)

// TO-DO (chenyu1): expand the test cases as development stablizes.

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	// Add custom APIs to the runtime scheme.
	if err := fleetv1beta1.AddToScheme(scheme.Scheme); err != nil {
		log.Fatalf("failed to add custom APIs to the runtime scheme: %v", err)
	}

	os.Exit(m.Run())
}

// TestCollectClusters tests the collectClusters method.
func TestCollectClusters(t *testing.T) {
	cluster := fleetv1beta1.MemberCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-1",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(&cluster).
		Build()
	// Construct framework manually instead of using NewFramework() to avoid mocking the controller manager.
	f := &framework{
		client: fakeClient,
	}

	ctx := context.Background()
	clusters, err := f.collectClusters(ctx)
	if err != nil {
		t.Fatalf("collectClusters() = %v, want no error", err)
	}

	want := []fleetv1beta1.MemberCluster{cluster}
	if !cmp.Equal(clusters, want) {
		t.Fatalf("collectClusters() = %v, want %v", clusters, want)
	}
}

// TestCollectBindings tests the collectBindings method.
func TestCollectBindings(t *testing.T) {
	binding := &fleetv1beta1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
			Labels: map[string]string{
				fleetv1beta1.CRPTrackingLabel: crpName,
			},
		},
	}
	altCRPName := "another-test-placement"

	testCases := []struct {
		name    string
		binding *fleetv1beta1.ClusterResourceBinding
		crpName string
		want    []fleetv1beta1.ClusterResourceBinding
	}{
		{
			name:    "found matching bindings",
			binding: binding,
			crpName: crpName,
			want:    []fleetv1beta1.ClusterResourceBinding{*binding},
		},
		{
			name:    "no matching bindings",
			binding: binding,
			crpName: altCRPName,
			want:    []fleetv1beta1.ClusterResourceBinding{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tc.binding != nil {
				fakeClientBuilder.WithObjects(tc.binding)
			}
			fakeClient := fakeClientBuilder.Build()
			// Construct framework manually instead of using NewFramework() to avoid mocking the controller manager.
			f := &framework{
				uncachedReader: fakeClient,
			}

			ctx := context.Background()
			bindings, err := f.collectBindings(ctx, tc.crpName)
			if err != nil {
				t.Fatalf("collectBindings() = %v, want no error", err)
			}
			if !cmp.Equal(bindings, tc.want, ignoreObjectMetaResourceVersionField) {
				t.Fatalf("collectBindings() = %v, want %v", bindings, tc.want)
			}
		})
	}
}

func TestClassifyBindings(t *testing.T) {
	policy := &fleetv1beta1.ClusterPolicySnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
	}

	clusterName1 := "cluster-1"
	clusterName2 := "cluster-2"
	clusterName3 := "cluster-3"
	clusterName4 := "cluster-4"
	clusters := []fleetv1beta1.MemberCluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName1,
			},
			Spec: fleetv1beta1.MemberClusterSpec{
				State: fleetv1beta1.ClusterStateJoin,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName2,
			},
			Spec: fleetv1beta1.MemberClusterSpec{
				State: fleetv1beta1.ClusterStateJoin,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName3,
			},
			Spec: fleetv1beta1.MemberClusterSpec{
				State: fleetv1beta1.ClusterStateLeave,
			},
		},
	}

	deletingBinding := fleetv1beta1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "binding-3",
		},
		Spec: fleetv1beta1.ResourceBindingSpec{
			State: fleetv1beta1.BindingStateDeleting,
		},
	}
	associatedWithLeavingClusterBinding := fleetv1beta1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "binding-4",
		},
		Spec: fleetv1beta1.ResourceBindingSpec{
			State:              fleetv1beta1.BindingStateActive,
			TargetCluster:      clusterName3,
			PolicySnapshotName: altPolicyName,
		},
	}
	assocaitedWithDisappearedClusterBinding := fleetv1beta1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "binding-5",
		},
		Spec: fleetv1beta1.ResourceBindingSpec{
			State:              fleetv1beta1.BindingStateCreating,
			TargetCluster:      clusterName4,
			PolicySnapshotName: policyName,
		},
	}
	obsoleteBinding := fleetv1beta1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "binding-6",
		},
		Spec: fleetv1beta1.ResourceBindingSpec{
			State:              fleetv1beta1.BindingStateActive,
			TargetCluster:      clusterName1,
			PolicySnapshotName: altPolicyName,
		},
	}
	activeBinding := fleetv1beta1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "binding-7",
		},
		Spec: fleetv1beta1.ResourceBindingSpec{
			State:              fleetv1beta1.BindingStateActive,
			TargetCluster:      clusterName1,
			PolicySnapshotName: policyName,
		},
	}
	creatingBinding := fleetv1beta1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "binding-8",
		},
		Spec: fleetv1beta1.ResourceBindingSpec{
			State:              fleetv1beta1.BindingStateCreating,
			TargetCluster:      clusterName2,
			PolicySnapshotName: policyName,
		},
	}

	bindings := []fleetv1beta1.ClusterResourceBinding{
		deletingBinding,
		associatedWithLeavingClusterBinding,
		assocaitedWithDisappearedClusterBinding,
		obsoleteBinding,
		activeBinding,
		creatingBinding,
	}
	wantActive := []*fleetv1beta1.ClusterResourceBinding{&activeBinding}
	wantCreating := []*fleetv1beta1.ClusterResourceBinding{&creatingBinding}
	wantObsolete := []*fleetv1beta1.ClusterResourceBinding{&obsoleteBinding}
	wantDangling := []*fleetv1beta1.ClusterResourceBinding{&associatedWithLeavingClusterBinding, &assocaitedWithDisappearedClusterBinding}

	active, creating, obsolete, dangling := classifyBindings(policy, bindings, clusters)
	if !cmp.Equal(active, wantActive) {
		t.Errorf("classifyBindings() active = %v, want %v", active, wantActive)
	}

	if !cmp.Equal(creating, wantCreating) {
		t.Errorf("classifyBindings() creating = %v, want %v", creating, wantCreating)
	}

	if !cmp.Equal(obsolete, wantObsolete) {
		t.Errorf("classifyBindings() obsolete = %v, want %v", obsolete, wantObsolete)
	}

	if !cmp.Equal(dangling, wantDangling) {
		t.Errorf("classifyBindings() dangling = %v, want %v", dangling, wantDangling)
	}
}

// TestShouldDownscale tests the shouldDownscale function.
func TestShouldDownscale(t *testing.T) {
	testCases := []struct {
		name      string
		policy    *fleetv1beta1.ClusterPolicySnapshot
		desired   int
		present   int
		obsolete  int
		wantAct   bool
		wantCount int
	}{
		{
			name: "should not downscale (pick all)",
			policy: &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
				Spec: fleetv1beta1.SchedulingPolicySnapshotSpec{
					Policy: &fleetv1beta1.PlacementPolicy{
						PlacementType: fleetv1beta1.PickAllPlacementType,
					},
				},
			},
		},
		{
			name: "should not downscale (enough bindings)",
			policy: &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
				Spec: fleetv1beta1.SchedulingPolicySnapshotSpec{
					Policy: &fleetv1beta1.PlacementPolicy{
						PlacementType: fleetv1beta1.PickNPlacementType,
					},
				},
			},
			desired: 1,
			present: 1,
		},
		{
			name: "should downscale (not enough bindings)",
			policy: &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
				Spec: fleetv1beta1.SchedulingPolicySnapshotSpec{
					Policy: &fleetv1beta1.PlacementPolicy{
						PlacementType: fleetv1beta1.PickNPlacementType,
					},
				},
			},
			desired:   0,
			present:   1,
			wantAct:   true,
			wantCount: 1,
		},
		{
			name: "should downscale (obsolete bindings)",
			policy: &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
				Spec: fleetv1beta1.SchedulingPolicySnapshotSpec{
					Policy: &fleetv1beta1.PlacementPolicy{
						PlacementType: fleetv1beta1.PickNPlacementType,
					},
				},
			},
			desired:   1,
			present:   1,
			obsolete:  1,
			wantAct:   true,
			wantCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			act, count := shouldDownscale(tc.policy, tc.desired, tc.present, tc.obsolete)
			if act != tc.wantAct || count != tc.wantCount {
				t.Fatalf("shouldDownscale() = %v, %v, want %v, %v", act, count, tc.wantAct, tc.wantCount)
			}
		})
	}
}

// TestSortByCreationTimestampBindingsWrapper checks if the sortByCreationTimestampBindings wrapper implements
// sort.Interface correctly, i.e, if it is sortable by CreationTimestamp.
func TestSortByCreationTimestampBindingsWrapper(t *testing.T) {
	timestampA := metav1.Now()
	timestampB := metav1.NewTime(time.Now().Add(time.Second))

	sorted := sortByCreationTimestampBindings([]*fleetv1beta1.ClusterResourceBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              bindingName,
				CreationTimestamp: timestampB,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              altBindingName,
				CreationTimestamp: timestampA,
			},
		},
	})
	sort.Sort(sorted)

	want := sortByCreationTimestampBindings([]*fleetv1beta1.ClusterResourceBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              altBindingName,
				CreationTimestamp: timestampA,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              bindingName,
				CreationTimestamp: timestampB,
			},
		},
	})
	if !cmp.Equal(sorted, want) {
		t.Fatalf("sortByCreationTimestamp, got %v, want %v", sorted, want)
	}
}

// TestUpdatePolicySnapshotStatusFrom tests the updatePolicySnapshotStatusFrom method.
func TestUpdatePolicySnapshotStatusFrom(t *testing.T) {
	defaultClusterDecisionCount := 20

	affinityScore1 := int32(1)
	topologySpreadScore1 := int32(10)
	affinityScore2 := int32(0)
	topologySpreadScore2 := int32(20)

	filteredStatus := NewNonErrorStatus(ClusterUnschedulable, dummyPluginName, "filtered")

	policy := &fleetv1beta1.ClusterPolicySnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
	}

	testCases := []struct {
		name                    string
		maxClusterDecisionCount int
		filtered                []*filteredClusterWithStatus
		existing                [][]*fleetv1beta1.ClusterResourceBinding
		wantDecisions           []fleetv1beta1.ClusterDecision
		wantCondition           metav1.Condition
	}{
		{
			name:                    "no filtered",
			maxClusterDecisionCount: defaultClusterDecisionCount,
			existing: [][]*fleetv1beta1.ClusterResourceBinding{
				{
					&fleetv1beta1.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: bindingName,
						},
						Spec: fleetv1beta1.ResourceBindingSpec{
							ClusterDecision: fleetv1beta1.ClusterDecision{
								ClusterName: clusterName,
								Selected:    true,
								ClusterScore: &fleetv1beta1.ClusterScore{
									AffinityScore:       &affinityScore1,
									TopologySpreadScore: &topologySpreadScore1,
								},
								Reason: pickedByPolicyReason,
							},
						},
					},
					&fleetv1beta1.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: altBindingName,
						},
						Spec: fleetv1beta1.ResourceBindingSpec{
							ClusterDecision: fleetv1beta1.ClusterDecision{
								ClusterName: altClusterName,
								Selected:    true,
								ClusterScore: &fleetv1beta1.ClusterScore{
									AffinityScore:       &affinityScore2,
									TopologySpreadScore: &topologySpreadScore2,
								},
								Reason: pickedByPolicyReason,
							},
						},
					},
				},
			},
			wantDecisions: []fleetv1beta1.ClusterDecision{
				{
					ClusterName: clusterName,
					Selected:    true,
					ClusterScore: &fleetv1beta1.ClusterScore{
						AffinityScore:       &affinityScore1,
						TopologySpreadScore: &topologySpreadScore1,
					},
					Reason: pickedByPolicyReason,
				},
				{
					ClusterName: altClusterName,
					Selected:    true,
					ClusterScore: &fleetv1beta1.ClusterScore{
						AffinityScore:       &affinityScore2,
						TopologySpreadScore: &topologySpreadScore2,
					},
					Reason: pickedByPolicyReason,
				},
			},
			wantCondition: fullyScheduledCondition(policy),
		},
		{
			name:                    "filtered and existing",
			maxClusterDecisionCount: defaultClusterDecisionCount,
			existing: [][]*fleetv1beta1.ClusterResourceBinding{
				{
					&fleetv1beta1.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: bindingName,
						},
						Spec: fleetv1beta1.ResourceBindingSpec{
							ClusterDecision: fleetv1beta1.ClusterDecision{
								ClusterName: clusterName,
								Selected:    true,
								ClusterScore: &fleetv1beta1.ClusterScore{
									AffinityScore:       &affinityScore1,
									TopologySpreadScore: &topologySpreadScore1,
								},
								Reason: pickedByPolicyReason,
							},
						},
					},
					&fleetv1beta1.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: altBindingName,
						},
						Spec: fleetv1beta1.ResourceBindingSpec{
							ClusterDecision: fleetv1beta1.ClusterDecision{
								ClusterName: altClusterName,
								Selected:    true,
								ClusterScore: &fleetv1beta1.ClusterScore{
									AffinityScore:       &affinityScore2,
									TopologySpreadScore: &topologySpreadScore2,
								},
								Reason: pickedByPolicyReason,
							},
						},
					},
				},
			},
			filtered: []*filteredClusterWithStatus{
				{
					cluster: &fleetv1beta1.MemberCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: anotherClusterName,
						},
					},
					status: filteredStatus,
				},
			},
			wantDecisions: []fleetv1beta1.ClusterDecision{
				{
					ClusterName: clusterName,
					Selected:    true,
					ClusterScore: &fleetv1beta1.ClusterScore{
						AffinityScore:       &affinityScore1,
						TopologySpreadScore: &topologySpreadScore1,
					},
					Reason: pickedByPolicyReason,
				},
				{
					ClusterName: altClusterName,
					Selected:    true,
					ClusterScore: &fleetv1beta1.ClusterScore{
						AffinityScore:       &affinityScore2,
						TopologySpreadScore: &topologySpreadScore2,
					},
					Reason: pickedByPolicyReason,
				},
				{
					ClusterName: anotherClusterName,
					Selected:    false,
					Reason:      filteredStatus.String(),
				},
			},
			wantCondition: fullyScheduledCondition(policy),
		},
		{
			name:                    "none",
			maxClusterDecisionCount: defaultClusterDecisionCount,
			wantCondition:           fullyScheduledCondition(policy),
		},
		{
			name:                    "too many existing bindings",
			maxClusterDecisionCount: 2,
			existing: [][]*fleetv1beta1.ClusterResourceBinding{
				{
					&fleetv1beta1.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: bindingName,
						},
						Spec: fleetv1beta1.ResourceBindingSpec{
							ClusterDecision: fleetv1beta1.ClusterDecision{
								ClusterName: clusterName,
								Selected:    true,
								ClusterScore: &fleetv1beta1.ClusterScore{
									AffinityScore:       &affinityScore1,
									TopologySpreadScore: &topologySpreadScore1,
								},
								Reason: pickedByPolicyReason,
							},
						},
					},
					&fleetv1beta1.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: altBindingName,
						},
						Spec: fleetv1beta1.ResourceBindingSpec{
							ClusterDecision: fleetv1beta1.ClusterDecision{
								ClusterName: altClusterName,
								Selected:    true,
								ClusterScore: &fleetv1beta1.ClusterScore{
									AffinityScore:       &affinityScore2,
									TopologySpreadScore: &topologySpreadScore2,
								},
								Reason: pickedByPolicyReason,
							},
						},
					},
				},
			},
			filtered: []*filteredClusterWithStatus{
				{
					cluster: &fleetv1beta1.MemberCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: anotherClusterName,
						},
					},
					status: filteredStatus,
				},
			},
			wantDecisions: []fleetv1beta1.ClusterDecision{
				{
					ClusterName: clusterName,
					Selected:    true,
					ClusterScore: &fleetv1beta1.ClusterScore{
						AffinityScore:       &affinityScore1,
						TopologySpreadScore: &topologySpreadScore1,
					},
					Reason: pickedByPolicyReason,
				},
				{
					ClusterName: altClusterName,
					Selected:    true,
					ClusterScore: &fleetv1beta1.ClusterScore{
						AffinityScore:       &affinityScore2,
						TopologySpreadScore: &topologySpreadScore2,
					},
					Reason: pickedByPolicyReason,
				},
			},
			wantCondition: fullyScheduledCondition(policy),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(policy).
				Build()
			// Construct framework manually instead of using NewFramework() to avoid mocking the controller manager.
			f := &framework{
				client:                  fakeClient,
				maxClusterDecisionCount: tc.maxClusterDecisionCount,
			}

			ctx := context.Background()
			if err := f.updatePolicySnapshotStatusFrom(ctx, policy, tc.filtered, tc.existing...); err != nil {
				t.Fatalf("updatePolicySnapshotStatusFrom() = %v, want no error", err)
			}

			updatedPolicy := &fleetv1beta1.ClusterPolicySnapshot{}
			if err := f.client.Get(ctx, types.NamespacedName{Name: policyName}, updatedPolicy); err != nil {
				t.Fatalf("Get policy snapshot, got %v, want no error", err)
			}

			if diff := cmp.Diff(updatedPolicy.Status.ClusterDecisions, tc.wantDecisions); diff != "" {
				t.Errorf("policy snapshot status cluster decisions not equal (-got, +want): %s", diff)
			}

			updatedCondition := meta.FindStatusCondition(updatedPolicy.Status.Conditions, string(fleetv1beta1.PolicySnapshotScheduled))
			if diff := cmp.Diff(updatedCondition, &tc.wantCondition, ignoredCondFields); diff != "" {
				t.Errorf("policy snapshot scheduled condition not equal (-got, +want): %s", diff)
			}
		})
	}
}

// TestRunPostBatchPlugins tests the runPostBatchPlugins method.
func TestRunPostBatchPlugins(t *testing.T) {
	dummyPostBatchPluginNameA := fmt.Sprintf(dummyAllPurposePluginNameFormat, 0)
	dummyPostBatchPluginNameB := fmt.Sprintf(dummyAllPurposePluginNameFormat, 1)

	testCases := []struct {
		name             string
		postBatchPlugins []PostBatchPlugin
		desiredBatchSize int
		wantBatchLimit   int
		wantStatus       *Status
	}{
		{
			name: "single plugin, success",
			postBatchPlugins: []PostBatchPlugin{
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameA,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 1, nil
					},
				},
			},
			desiredBatchSize: 10,
			wantBatchLimit:   1,
		},
		{
			name: "single plugin, success, oversized",
			postBatchPlugins: []PostBatchPlugin{
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameA,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 15, nil
					},
				},
			},
			desiredBatchSize: 10,
			wantBatchLimit:   10,
		},
		{
			name: "multiple plugins, all success",
			postBatchPlugins: []PostBatchPlugin{
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameA,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 2, nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameB,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 1, nil
					},
				},
			},
			desiredBatchSize: 10,
			wantBatchLimit:   1,
		},
		{
			name: "multple plugins, one success, one error",
			postBatchPlugins: []PostBatchPlugin{
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameA,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 0, FromError(fmt.Errorf("internal error"), dummyPostBatchPluginNameA)
					},
				},
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameB,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 1, nil
					},
				},
			},
			desiredBatchSize: 10,
			wantStatus:       FromError(fmt.Errorf("internal error"), dummyPostBatchPluginNameA),
		},
		{
			name: "single plugin, skip",
			postBatchPlugins: []PostBatchPlugin{
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameA,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 0, NewNonErrorStatus(Skip, dummyPostBatchPluginNameA)
					},
				},
			},
			desiredBatchSize: 10,
			wantBatchLimit:   10,
		},
		{
			name: "single plugin, unschedulable",
			postBatchPlugins: []PostBatchPlugin{
				&DummyAllPurposePlugin{
					name: dummyPostBatchPluginNameA,
					postBatchRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) {
						return 1, NewNonErrorStatus(ClusterUnschedulable, dummyPostBatchPluginNameA)
					},
				},
			},
			desiredBatchSize: 10,
			wantStatus:       FromError(fmt.Errorf("internal error"), dummyPostBatchPluginNameA),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfile(dummyProfileName)
			for _, p := range tc.postBatchPlugins {
				profile.WithPostBatchPlugin(p)
			}
			f := &framework{
				profile: profile,
			}

			ctx := context.Background()
			state := NewCycleState()
			state.desiredBatchSize = tc.desiredBatchSize
			policy := &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
			}
			batchLimit, status := f.runPostBatchPlugins(ctx, state, policy)
			if batchLimit != tc.wantBatchLimit || !cmp.Equal(status, tc.wantStatus, cmpopts.IgnoreUnexported(Status{}), ignoredStatusFields) {
				t.Errorf("runPostBatchPlugins(%v, %v) = %v %v, want %v, %v", state, policy, batchLimit, status, tc.wantBatchLimit, tc.wantStatus)
			}
		})
	}
}

// TestRunPreFilterPlugins tests the runPreFilterPlugins method.
func TestRunPreFilterPlugins(t *testing.T) {
	dummyPreFilterPluginNameA := fmt.Sprintf(dummyAllPurposePluginNameFormat, 0)
	dummyPreFilterPluginNameB := fmt.Sprintf(dummyAllPurposePluginNameFormat, 1)

	testCases := []struct {
		name                   string
		preFilterPlugins       []PreFilterPlugin
		wantSkippedPluginNames []string
		wantStatus             *Status
	}{
		{
			name: "single plugin, success",
			preFilterPlugins: []PreFilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyPreFilterPluginNameA,
					preFilterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) *Status {
						return nil
					},
				},
			},
		},
		{
			name: "multiple plugins, one success, one skip",
			preFilterPlugins: []PreFilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyPreFilterPluginNameA,
					preFilterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status) {
						return nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyPreFilterPluginNameB,
					preFilterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status) {
						return NewNonErrorStatus(Skip, dummyPreFilterPluginNameB)
					},
				},
			},
			wantSkippedPluginNames: []string{dummyPreFilterPluginNameB},
		},
		{
			name: "single plugin, internal error",
			preFilterPlugins: []PreFilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyPreFilterPluginNameA,
					preFilterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) *Status {
						return FromError(fmt.Errorf("internal error"), dummyPreFilterPluginNameA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("internal error"), dummyPreFilterPluginNameA),
		},
		{
			name: "single plugin, unschedulable",
			preFilterPlugins: []PreFilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyPreFilterPluginNameA,
					preFilterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) *Status {
						return NewNonErrorStatus(ClusterUnschedulable, dummyPreFilterPluginNameA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("cluster is unschedulable"), dummyPreFilterPluginNameA),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfile(dummyProfileName)
			for _, p := range tc.preFilterPlugins {
				profile.WithPreFilterPlugin(p)
			}
			f := &framework{
				profile: profile,
			}

			ctx := context.Background()
			state := NewCycleState()
			policy := &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
			}

			status := f.runPreFilterPlugins(ctx, state, policy)
			if !cmp.Equal(status, tc.wantStatus, cmp.AllowUnexported(Status{}), ignoredStatusFields) {
				t.Errorf("runPreFilterPlugins(%v, %v) = %v, want %v", state, policy, status, tc.wantStatus)
			}
		})
	}
}

// TestRunFilterPluginsFor tests the runFilterPluginsFor method.
func TestRunFilterPluginsFor(t *testing.T) {
	dummyFilterPluginNameA := fmt.Sprintf(dummyAllPurposePluginNameFormat, 0)
	dummyFilterPluginNameB := fmt.Sprintf(dummyAllPurposePluginNameFormat, 1)

	testCases := []struct {
		name               string
		filterPlugins      []FilterPlugin
		skippedPluginNames []string
		wantStatus         *Status
	}{
		{
			name: "single plugin, success",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return nil
					},
				},
			},
		},
		{
			name: "multiple plugins, one success, one skipped",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameB,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return NewNonErrorStatus(ClusterUnschedulable, dummyFilterPluginNameB)
					},
				},
			},
			skippedPluginNames: []string{dummyFilterPluginNameB},
		},
		{
			name: "single plugin, internal error",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return FromError(fmt.Errorf("internal error"), dummyFilterPluginNameA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("internal error"), dummyFilterPluginNameA),
		},
		{
			name: "multiple plugins, one unschedulable, one success",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return NewNonErrorStatus(ClusterUnschedulable, dummyFilterPluginNameA)
					},
				},
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameB,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return nil
					},
				},
			},
			wantStatus: NewNonErrorStatus(ClusterUnschedulable, dummyFilterPluginNameA),
		},
		{
			name: "single plugin, skip",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return NewNonErrorStatus(Skip, dummyFilterPluginNameA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("internal error"), dummyFilterPluginNameA),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfile(dummyProfileName)
			for _, p := range tc.filterPlugins {
				profile.WithFilterPlugin(p)
			}
			f := &framework{
				profile: profile,
			}

			ctx := context.Background()
			state := NewCycleState()
			for _, name := range tc.skippedPluginNames {
				state.skippedFilterPlugins.Insert(name)
			}
			policy := &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
			}
			cluster := &fleetv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}

			status := f.runFilterPluginsFor(ctx, state, policy, cluster)
			if !cmp.Equal(status, tc.wantStatus, cmpopts.IgnoreUnexported(Status{}), ignoredStatusFields) {
				t.Errorf("runFilterPluginsFor(%v, %v, %v) = %v, want %v", state, policy, cluster, status, tc.wantStatus)
			}
		})
	}
}

// TestRunFilterPlugins tests the runFilterPlugins method.
func TestRunFilterPlugins(t *testing.T) {
	dummyFilterPluginNameA := fmt.Sprintf(dummyAllPurposePluginNameFormat, 0)
	dummyFilterPluginNameB := fmt.Sprintf(dummyAllPurposePluginNameFormat, 1)

	clusters := []fleetv1beta1.MemberCluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: altClusterName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: anotherClusterName,
			},
		},
	}

	testCases := []struct {
		name                     string
		filterPlugins            []FilterPlugin
		wantPassedClusterNames   []string
		wantFilteredClusterNames []string
		expectedToFail           bool
	}{
		{
			name: "three clusters, two filter plugins, all passed",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameB,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return nil
					},
				},
			},
			wantPassedClusterNames: []string{clusterName, altClusterName, anotherClusterName},
		},
		{
			name: "three clusters, two filter plugins, two filtered",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						if cluster.Name == clusterName {
							return NewNonErrorStatus(ClusterUnschedulable, dummyFilterPluginNameA)
						}
						return nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameB,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						if cluster.Name == anotherClusterName {
							return NewNonErrorStatus(ClusterUnschedulable, dummyFilterPluginNameB)
						}
						return nil
					},
				},
			},
			wantPassedClusterNames:   []string{altClusterName},
			wantFilteredClusterNames: []string{clusterName, anotherClusterName},
		},
		{
			name: "three clusters, two filter plugins, one success, one internal error on specific cluster",
			filterPlugins: []FilterPlugin{
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameA,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						return nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyFilterPluginNameB,
					filterRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) {
						if cluster.Name == anotherClusterName {
							return FromError(fmt.Errorf("internal error"), dummyFilterPluginNameB)
						}
						return nil
					},
				},
			},
			expectedToFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfile(dummyProfileName)
			for _, p := range tc.filterPlugins {
				profile.WithFilterPlugin(p)
			}
			f := &framework{
				profile:      profile,
				parallelizer: parallelizer.NewParallelizer(parallelizer.DefaultNumOfWorkers),
			}

			ctx := context.Background()
			state := NewCycleState()
			policy := &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
			}

			passed, filtered, err := f.runFilterPlugins(ctx, state, policy, clusters)
			if tc.expectedToFail {
				if err == nil {
					t.Errorf("runFilterPlugins(%v, %v, %v) = %v %v %v, want error", state, policy, clusters, passed, filtered, err)
				}
				return
			}

			// The method runs in parallel; as a result the order cannot be guaranteed.
			// Organize the results into maps for easier comparison.
			passedMap := make(map[string]bool)
			for _, cluster := range passed {
				passedMap[cluster.Name] = true
			}
			wantPassedMap := make(map[string]bool)
			for _, name := range tc.wantPassedClusterNames {
				wantPassedMap[name] = true
			}

			if !cmp.Equal(passedMap, wantPassedMap) {
				t.Errorf("passed clusters, got %v, want %v", passedMap, wantPassedMap)
			}

			filteredMap := make(map[string]bool)
			for _, item := range filtered {
				filteredMap[item.cluster.Name] = true
				// As a sanity check, verify if all status are of the ClusterUnschedulable status code.
				if !item.status.IsClusterUnschedulable() {
					t.Errorf("filtered cluster %s status, got %v, want status code ClusterUnschedulable", item.cluster.Name, item.status)
				}
			}
			wantFilteredMap := make(map[string]bool)
			for _, name := range tc.wantFilteredClusterNames {
				wantFilteredMap[name] = true
			}

			if !cmp.Equal(filteredMap, wantFilteredMap) {
				t.Errorf("filtered clusters, got %v, want %v", filteredMap, wantFilteredMap)
			}
		})
	}
}

// TestRunPreScorePlugins tests the runPreScorePlugins method.
func TestRunPreScorePlugins(t *testing.T) {
	dummyPreScorePluginNameA := fmt.Sprintf(dummyAllPurposePluginNameFormat, 0)
	dummyPreScorePluginNameB := fmt.Sprintf(dummyAllPurposePluginNameFormat, 1)

	testCases := []struct {
		name                   string
		preScorePlugins        []PreScorePlugin
		wantSkippedPluginNames []string
		wantStatus             *Status
	}{
		{
			name: "single plugin, success",
			preScorePlugins: []PreScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyPreScorePluginNameA,
					preScoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) *Status {
						return nil
					},
				},
			},
		},
		{
			name: "multiple plugins, one success, one skip",
			preScorePlugins: []PreScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyPreScorePluginNameA,
					preScoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) *Status {
						return nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyPreScorePluginNameB,
					preScoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status) {
						return NewNonErrorStatus(Skip, dummyPreScorePluginNameB)
					},
				},
			},
			wantSkippedPluginNames: []string{dummyPreScorePluginNameB},
		},
		{
			name: "single plugin, internal error",
			preScorePlugins: []PreScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyPreScorePluginNameA,
					preScoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status) {
						return FromError(fmt.Errorf("internal error"), dummyPreScorePluginNameA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("internal error"), dummyPreScorePluginNameA),
		},
		{
			name: "single plugin, unschedulable",
			preScorePlugins: []PreScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyPreScorePluginNameA,
					preScoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status) {
						return NewNonErrorStatus(ClusterUnschedulable, dummyPreScorePluginNameA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("cluster is unschedulable"), dummyPreScorePluginNameA),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfile(dummyProfileName)
			for _, p := range tc.preScorePlugins {
				profile.WithPreScorePlugin(p)
			}
			f := &framework{
				profile: profile,
			}

			ctx := context.Background()
			state := NewCycleState()
			policy := &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
			}

			status := f.runPreScorePlugins(ctx, state, policy)
			if !cmp.Equal(status, tc.wantStatus, cmp.AllowUnexported(Status{}), ignoredStatusFields) {
				t.Errorf("runPreScorePlugins(%v, %v) = %v, want %v", state, policy, status, tc.wantStatus)
			}
		})
	}
}

// TestRunScorePluginsFor tests the runScorePluginsFor method.
func TestRunScorePluginsFor(t *testing.T) {
	dummyScorePluginA := fmt.Sprintf(dummyAllPurposePluginNameFormat, 0)
	dummyScorePluginB := fmt.Sprintf(dummyAllPurposePluginNameFormat, 1)

	testCases := []struct {
		name               string
		scorePlugins       []ScorePlugin
		skippedPluginNames []string
		wantStatus         *Status
		wantScoreList      map[string]*ClusterScore
	}{
		{
			name: "single plugin, success",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return &ClusterScore{
							TopologySpreadScore: 1,
							AffinityScore:       20,
						}, nil
					},
				},
			},
			wantScoreList: map[string]*ClusterScore{
				dummyScorePluginA: {
					TopologySpreadScore: 1,
					AffinityScore:       20,
				},
			},
		},
		{
			name: "multiple plugins, all success",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return &ClusterScore{
							TopologySpreadScore: 1,
							AffinityScore:       20,
						}, nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyScorePluginB,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return &ClusterScore{
							TopologySpreadScore: 0,
							AffinityScore:       10,
						}, nil
					},
				},
			},
			wantScoreList: map[string]*ClusterScore{
				dummyScorePluginA: {
					TopologySpreadScore: 1,
					AffinityScore:       20,
				},
				dummyScorePluginB: {
					TopologySpreadScore: 0,
					AffinityScore:       10,
				},
			},
		},
		{
			name: "multiple plugin, one success, one skipped",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return &ClusterScore{
							TopologySpreadScore: 1,
							AffinityScore:       20,
						}, nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyScorePluginB,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return &ClusterScore{
							TopologySpreadScore: 0,
							AffinityScore:       10,
						}, nil
					},
				},
			},
			skippedPluginNames: []string{dummyScorePluginB},
			wantScoreList: map[string]*ClusterScore{
				dummyScorePluginA: {
					TopologySpreadScore: 1,
					AffinityScore:       20,
				},
			},
		},
		{
			name: "single plugin, internal error",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return nil, FromError(fmt.Errorf("internal error"), dummyScorePluginA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("internal error"), dummyScorePluginA),
		},
		{
			name: "single plugin, skip",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return nil, NewNonErrorStatus(Skip, dummyScorePluginA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("unexpected status"), dummyScorePluginA),
		},
		{
			name: "single plugin, unschedulable",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						return nil, NewNonErrorStatus(ClusterUnschedulable, dummyScorePluginA)
					},
				},
			},
			wantStatus: FromError(fmt.Errorf("unexpected status"), dummyScorePluginA),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfile(dummyProfileName)
			for _, p := range tc.scorePlugins {
				profile.WithScorePlugin(p)
			}
			f := &framework{
				profile: profile,
			}

			ctx := context.Background()
			state := NewCycleState()
			for _, name := range tc.skippedPluginNames {
				state.skippedScorePlugins.Insert(name)
			}
			policy := &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
			}
			cluster := &fleetv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}

			scoreList, status := f.runScorePluginsFor(ctx, state, policy, cluster)
			if !cmp.Equal(status, tc.wantStatus, cmp.AllowUnexported(Status{}), ignoredStatusFields) {
				t.Errorf("runScorePluginsFor() status = %v, want %v", status, tc.wantStatus)
			}

			if !cmp.Equal(scoreList, tc.wantScoreList) {
				t.Errorf("runScorePluginsFor() scoreList = %v, want %v", scoreList, tc.wantScoreList)
			}
		})
	}
}

// TestRunScorePlugins tests the runScorePlugins method.
func TestRunScorePlugins(t *testing.T) {
	dummyScorePluginNameA := fmt.Sprintf(dummyAllPurposePluginNameFormat, 0)
	dummyScorePluginNameB := fmt.Sprintf(dummyAllPurposePluginNameFormat, 1)

	clusters := []*fleetv1beta1.MemberCluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: altClusterName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: anotherClusterName,
			},
		},
	}

	testCases := []struct {
		name               string
		scorePlugins       []ScorePlugin
		wantScoredClusters ScoredClusters
		expectedToFail     bool
	}{
		{
			name: "three clusters, two score plugins, all scored",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginNameA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						switch cluster.Name {
						case clusterName:
							return &ClusterScore{
								TopologySpreadScore: 1,
							}, nil
						case altClusterName:
							return &ClusterScore{
								TopologySpreadScore: 0,
							}, nil
						case anotherClusterName:
							return &ClusterScore{
								TopologySpreadScore: 2,
							}, nil
						}
						return &ClusterScore{}, nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyScorePluginNameB,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						switch cluster.Name {
						case clusterName:
							return &ClusterScore{
								AffinityScore: 10,
							}, nil
						case altClusterName:
							return &ClusterScore{
								AffinityScore: 20,
							}, nil
						case anotherClusterName:
							return &ClusterScore{
								AffinityScore: 15,
							}, nil
						}
						return &ClusterScore{}, nil
					},
				},
			},
			wantScoredClusters: ScoredClusters{
				{
					Cluster: clusters[0],
					Score: &ClusterScore{
						TopologySpreadScore: 1,
						AffinityScore:       10,
					},
				},
				{
					Cluster: clusters[1],
					Score: &ClusterScore{
						TopologySpreadScore: 0,
						AffinityScore:       20,
					},
				},
				{
					Cluster: clusters[2],
					Score: &ClusterScore{
						TopologySpreadScore: 2,
						AffinityScore:       15,
					},
				},
			},
		},
		{
			name: "three clusters, two score plugins, one internal error on specific cluster",
			scorePlugins: []ScorePlugin{
				&DummyAllPurposePlugin{
					name: dummyScorePluginNameA,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						switch cluster.Name {
						case clusterName:
							return &ClusterScore{
								TopologySpreadScore: 1,
							}, nil
						case altClusterName:
							return &ClusterScore{
								TopologySpreadScore: 0,
							}, nil
						case anotherClusterName:
							return &ClusterScore{
								TopologySpreadScore: 2,
							}, nil
						}
						return &ClusterScore{}, nil
					},
				},
				&DummyAllPurposePlugin{
					name: dummyScorePluginNameB,
					scoreRunner: func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) {
						switch cluster.Name {
						case clusterName:
							return &ClusterScore{
								AffinityScore: 10,
							}, nil
						case altClusterName:
							return &ClusterScore{}, FromError(fmt.Errorf("internal error"), dummyScorePluginNameB)
						case anotherClusterName:
							return &ClusterScore{
								AffinityScore: 15,
							}, nil
						}
						return &ClusterScore{}, nil
					},
				},
			},
			expectedToFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := NewProfile(dummyProfileName)
			for _, p := range tc.scorePlugins {
				profile.WithScorePlugin(p)
			}
			f := &framework{
				profile:      profile,
				parallelizer: parallelizer.NewParallelizer(parallelizer.DefaultNumOfWorkers),
			}

			ctx := context.Background()
			state := NewCycleState()
			policy := &fleetv1beta1.ClusterPolicySnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: policyName,
				},
			}

			scoredClusters, err := f.runScorePlugins(ctx, state, policy, clusters)
			if tc.expectedToFail {
				if err == nil {
					t.Errorf("runScorePlugins(), got no error, want error")
				}
				return
			}

			// The method runs in parallel; as a result the order cannot be guaranteed.
			// Organize the results into maps for easier comparison.
			scoreMap := make(map[string]*ClusterScore)
			for _, scoredCluster := range scoredClusters {
				scoreMap[scoredCluster.Cluster.Name] = scoredCluster.Score
			}

			wantScoreMap := make(map[string]*ClusterScore)
			for _, scoredCluster := range tc.wantScoredClusters {
				wantScoreMap[scoredCluster.Cluster.Name] = scoredCluster.Score
			}

			if !cmp.Equal(scoreMap, wantScoreMap) {
				t.Errorf("runScorePlugins() scored clusters, got %v, want %v", scoreMap, wantScoreMap)
			}
		})
	}
}

// TestCalcNumOfClustersToSelect tests the calcNumOfClustersToSelect function.
func TestCalcNumOfClustersToSelect(t *testing.T) {
	testCases := []struct {
		name    string
		desired int
		limit   int
		scored  int
		want    int
	}{
		{
			name:    "no limit, enough bindings to pick",
			desired: 3,
			limit:   3,
			scored:  10,
			want:    3,
		},
		{
			name:    "limit imposed, enough bindings to pick",
			desired: 3,
			limit:   2,
			scored:  10,
			want:    2,
		},
		{
			name:    "limit imposed, not enough bindings to pick",
			desired: 3,
			limit:   2,
			scored:  1,
			want:    1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			toSelect := calcNumOfClustersToSelect(tc.desired, tc.limit, tc.scored)
			if toSelect != tc.want {
				t.Errorf("calcNumOfClustersToSelect(), got %d, want %d", toSelect, tc.want)
			}
		})
	}
}

// TestPickTopNScoredClusters tests the pickTopNScoredClusters function.
func TestPickTopNScoredClusters(t *testing.T) {
	scs := ScoredClusters{
		{
			Cluster: &fleetv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			},
			Score: &ClusterScore{
				TopologySpreadScore:          1,
				AffinityScore:                20,
				ActiveOrCreatingBindingScore: 0,
			},
		},
		{
			Cluster: &fleetv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: altClusterName,
				},
			},
			Score: &ClusterScore{
				TopologySpreadScore:          2,
				AffinityScore:                10,
				ActiveOrCreatingBindingScore: 1,
			},
		},
	}

	testCases := []struct {
		name               string
		scoredClusters     ScoredClusters
		picks              int
		wantScoredClusters ScoredClusters
	}{
		{
			name:               "no scored clusters",
			scoredClusters:     ScoredClusters{},
			picks:              1,
			wantScoredClusters: ScoredClusters{},
		},
		{
			name:               "zero to pick",
			scoredClusters:     scs,
			picks:              0,
			wantScoredClusters: ScoredClusters{},
		},
		{
			name:           "not enough to pick",
			scoredClusters: scs,
			picks:          10,
			wantScoredClusters: ScoredClusters{
				{
					Cluster: &fleetv1beta1.MemberCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: altClusterName,
						},
					},
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: &fleetv1beta1.MemberCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: clusterName,
						},
					},
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                20,
						ActiveOrCreatingBindingScore: 0,
					},
				},
			},
		},
		{
			name:           "enough to pick",
			scoredClusters: scs,
			picks:          1,
			wantScoredClusters: ScoredClusters{
				{
					Cluster: &fleetv1beta1.MemberCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: altClusterName,
						},
					},
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			picked := pickTopNScoredClusters(tc.scoredClusters, tc.picks)
			if !cmp.Equal(picked, tc.wantScoredClusters) {
				t.Errorf("pickTopNScoredClusters(), got %v, want %v", picked, tc.wantScoredClusters)
			}
		})
	}
}

// TestCrossReferencePickedClustersAndObsoleteBindings tests the crossReferencePickedClustersAndObsoleteBindings function.
func TestCrossReferencePickedCustersAndObsoleteBindings(t *testing.T) {
	policy := &fleetv1beta1.ClusterPolicySnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
	}

	clusterName1 := "cluster-1"
	clusterName2 := "cluster-2"
	clusterName3 := "cluster-3"
	clusterName4 := "cluster-4"

	affinityScore1 := int32(10)
	topologySpreadScore1 := int32(2)
	affinityScore2 := int32(20)
	topologySpreadScore2 := int32(1)
	affinityScore3 := int32(30)
	topologySpreadScore3 := int32(0)

	sorted := ScoredClusters{
		{
			Cluster: &fleetv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName1,
				},
			},
			Score: &ClusterScore{
				TopologySpreadScore:          int(topologySpreadScore1),
				AffinityScore:                int(affinityScore1),
				ActiveOrCreatingBindingScore: 1,
			},
		},
		{
			Cluster: &fleetv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName2,
				},
			},
			Score: &ClusterScore{
				TopologySpreadScore:          int(topologySpreadScore2),
				AffinityScore:                int(affinityScore2),
				ActiveOrCreatingBindingScore: 0,
			},
		},
		{
			Cluster: &fleetv1beta1.MemberCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName3,
				},
			},
			Score: &ClusterScore{
				TopologySpreadScore:          int(topologySpreadScore3),
				AffinityScore:                int(affinityScore3),
				ActiveOrCreatingBindingScore: 1,
			},
		},
	}

	// Note that these names are placeholders only; actual names should be generated one.
	bindingName1 := "binding-1"
	bindingName2 := "binding-2"
	bindingName3 := "binding-3"
	bindingName4 := "binding-4"

	testCases := []struct {
		name         string
		picked       ScoredClusters
		obsolete     []*fleetv1beta1.ClusterResourceBinding
		wantToCreate []*fleetv1beta1.ClusterResourceBinding
		wantToUpdate []*fleetv1beta1.ClusterResourceBinding
		wantToDelete []*fleetv1beta1.ClusterResourceBinding
	}{
		{
			name:   "no matching obsolete bindings",
			picked: sorted,
			obsolete: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName4,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName4,
					},
				},
			},
			wantToCreate: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName1,
						Labels: map[string]string{
							fleetv1beta1.CRPTrackingLabel: crpName,
						},
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						State:              fleetv1beta1.BindingStateCreating,
						PolicySnapshotName: policyName,
						TargetCluster:      clusterName1,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName1,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore1,
								TopologySpreadScore: &topologySpreadScore1,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName2,
						Labels: map[string]string{
							fleetv1beta1.CRPTrackingLabel: crpName,
						},
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						State:              fleetv1beta1.BindingStateCreating,
						PolicySnapshotName: policyName,
						TargetCluster:      clusterName2,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName2,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore2,
								TopologySpreadScore: &topologySpreadScore2,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName3,
						Labels: map[string]string{
							fleetv1beta1.CRPTrackingLabel: crpName,
						},
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						State:              fleetv1beta1.BindingStateCreating,
						PolicySnapshotName: policyName,
						TargetCluster:      clusterName3,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName3,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore3,
								TopologySpreadScore: &topologySpreadScore3,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
			},
			wantToUpdate: []*fleetv1beta1.ClusterResourceBinding{},
			wantToDelete: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName4,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName4,
					},
				},
			},
		},
		{
			name:   "all matching obsolete bindings",
			picked: sorted,
			obsolete: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName1,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName1,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName2,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName2,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName3,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName3,
					},
				},
			},
			wantToCreate: []*fleetv1beta1.ClusterResourceBinding{},
			wantToUpdate: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName1,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster:      clusterName1,
						PolicySnapshotName: policyName,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName1,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore1,
								TopologySpreadScore: &topologySpreadScore1,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName2,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster:      clusterName2,
						PolicySnapshotName: policyName,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName2,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore2,
								TopologySpreadScore: &topologySpreadScore2,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName3,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster:      clusterName3,
						PolicySnapshotName: policyName,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName3,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore3,
								TopologySpreadScore: &topologySpreadScore3,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
			},
			wantToDelete: []*fleetv1beta1.ClusterResourceBinding{},
		},
		{
			name:   "mixed",
			picked: sorted,
			obsolete: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName1,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName1,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName2,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName2,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName4,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName4,
					},
				},
			},
			wantToCreate: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName3,
						Labels: map[string]string{
							fleetv1beta1.CRPTrackingLabel: crpName,
						},
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						State:              fleetv1beta1.BindingStateCreating,
						PolicySnapshotName: policyName,
						TargetCluster:      clusterName3,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName3,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore3,
								TopologySpreadScore: &topologySpreadScore3,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
			},
			wantToUpdate: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName1,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster:      clusterName1,
						PolicySnapshotName: policyName,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName1,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore1,
								TopologySpreadScore: &topologySpreadScore1,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName2,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster:      clusterName2,
						PolicySnapshotName: policyName,
						ClusterDecision: fleetv1beta1.ClusterDecision{
							ClusterName: clusterName2,
							Selected:    true,
							ClusterScore: &fleetv1beta1.ClusterScore{
								AffinityScore:       &affinityScore2,
								TopologySpreadScore: &topologySpreadScore2,
							},
							Reason: pickedByPolicyReason,
						},
					},
				},
			},
			wantToDelete: []*fleetv1beta1.ClusterResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: bindingName4,
					},
					Spec: fleetv1beta1.ResourceBindingSpec{
						TargetCluster: clusterName4,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			toCreate, toUpdate, toDelete, err := crossReferencePickedCustersAndObsoleteBindings(crpName, policy, tc.picked, tc.obsolete)
			if err != nil {
				t.Errorf("crossReferencePickedClustersAndObsoleteBindings() = %v, want no error", err)
				return
			}

			if !cmp.Equal(toCreate, tc.wantToCreate, ignoreObjectMetaNameField) {
				t.Errorf("crossReferencePickedClustersAndObsoleteBindings() toCreate = %v, got %v", toCreate, tc.wantToCreate)
			}

			// Verify names separately.
			for _, binding := range toCreate {
				prefix := fmt.Sprintf("%s-%s", crpName, binding.Spec.TargetCluster)
				if !strings.HasPrefix(binding.Name, prefix) {
					t.Errorf("toCreate binding name not valid, got %s, want prefix %s", binding.Name, prefix)
				}
			}

			if !cmp.Equal(toUpdate, tc.wantToUpdate) {
				t.Errorf("crossReferencePickedClustersAndObsoleteBindings() toUpdate = %v, got %v", toUpdate, tc.wantToUpdate)
			}

			if !cmp.Equal(toDelete, tc.wantToDelete) {
				t.Errorf("crossReferencePickedClustersAndObsoleteBindings() toDelete = %v, got %v", toDelete, tc.wantToDelete)
			}
		})
	}
}
