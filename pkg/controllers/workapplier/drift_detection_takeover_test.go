/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package workapplier

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

// TestTakeOverPreExistingObject tests the takeOverPreExistingObject method.
func TestTakeOverPreExistingObject(t *testing.T) {
	ctx := context.Background()

	nsWithNonFleetOwnerUnstructured := nsUnstructured.DeepCopy()
	nsWithNonFleetOwnerUnstructured.SetOwnerReferences([]metav1.OwnerReference{
		dummyOwnerRef,
	})

	nsWithFleetOwnerUnstructured := nsUnstructured.DeepCopy()
	nsWithFleetOwnerUnstructured.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "placement.kubernetes-fleet.io/v1beta1",
			Kind:       "AppliedWork",
			Name:       "dummy-work",
			UID:        "0987-6543-21",
		},
	})

	wantTakenOverObj := nsUnstructured.DeepCopy()
	wantTakenOverObj.SetOwnerReferences([]metav1.OwnerReference{
		*appliedWorkOwnerRef,
	})

	wantTakenOverObjWithAdditionalNonFleetOwner := nsUnstructured.DeepCopy()
	wantTakenOverObjWithAdditionalNonFleetOwner.SetOwnerReferences([]metav1.OwnerReference{
		dummyOwnerRef,
		*appliedWorkOwnerRef,
	})

	nsWithLabelsUnstructured := nsUnstructured.DeepCopy()
	nsWithLabelsUnstructured.SetLabels(map[string]string{
		"foo": "bar",
	})

	testCases := []struct {
		name                        string
		gvr                         *schema.GroupVersionResource
		manifestObj                 *unstructured.Unstructured
		inMemberClusterObj          *unstructured.Unstructured
		applyStrategy               *fleetv1beta1.ApplyStrategy
		expectedAppliedWorkOwnerRef *metav1.OwnerReference
		wantErred                   bool
		wantTakeOverObj             *unstructured.Unstructured
		wantPatchDetails            []fleetv1beta1.PatchDetail
	}{
		{
			name:               "existing non-Fleet owner, co-ownership not allowed",
			gvr:                &nsGVR,
			manifestObj:        nsUnstructured,
			inMemberClusterObj: nsWithNonFleetOwnerUnstructured,
			applyStrategy: &fleetv1beta1.ApplyStrategy{
				WhenToTakeOver:   fleetv1beta1.WhenToTakeOverTypeAlways,
				AllowCoOwnership: false,
			},
			expectedAppliedWorkOwnerRef: appliedWorkOwnerRef,
			wantErred:                   true,
		},
		{
			name:               "existing Fleet owner, co-ownership allowed",
			gvr:                &nsGVR,
			manifestObj:        nsUnstructured,
			inMemberClusterObj: nsWithFleetOwnerUnstructured,
			applyStrategy: &fleetv1beta1.ApplyStrategy{
				WhenToTakeOver:   fleetv1beta1.WhenToTakeOverTypeAlways,
				AllowCoOwnership: true,
			},
			expectedAppliedWorkOwnerRef: appliedWorkOwnerRef,
			wantErred:                   true,
		},
		{
			name:               "no owner, always take over",
			gvr:                &nsGVR,
			manifestObj:        nsUnstructured,
			inMemberClusterObj: nsUnstructured,
			applyStrategy: &fleetv1beta1.ApplyStrategy{
				WhenToTakeOver: fleetv1beta1.WhenToTakeOverTypeAlways,
			},
			expectedAppliedWorkOwnerRef: appliedWorkOwnerRef,
			wantTakeOverObj:             wantTakenOverObj,
		},
		{
			name:               "existing non-Fleet owner,co-ownership allowed, always take over",
			gvr:                &nsGVR,
			manifestObj:        nsUnstructured,
			inMemberClusterObj: nsWithNonFleetOwnerUnstructured,
			applyStrategy: &fleetv1beta1.ApplyStrategy{
				WhenToTakeOver:   fleetv1beta1.WhenToTakeOverTypeAlways,
				AllowCoOwnership: true,
			},
			expectedAppliedWorkOwnerRef: appliedWorkOwnerRef,
			wantTakeOverObj:             wantTakenOverObjWithAdditionalNonFleetOwner,
		},
		// The fake client Fleet uses for unit tests has trouble processing dry-run requests; such
		// test cases will be handled in integration tests instead.
		{
			name:               "no owner, take over if no diff, diff found, full comparison",
			gvr:                &nsGVR,
			manifestObj:        nsUnstructured,
			inMemberClusterObj: nsWithLabelsUnstructured,
			applyStrategy: &fleetv1beta1.ApplyStrategy{
				WhenToTakeOver:   fleetv1beta1.WhenToTakeOverTypeIfNoDiff,
				ComparisonOption: fleetv1beta1.ComparisonOptionTypeFullComparison,
			},
			expectedAppliedWorkOwnerRef: appliedWorkOwnerRef,
			wantPatchDetails: []fleetv1beta1.PatchDetail{
				{
					Path:          "/metadata/labels/foo",
					ValueInMember: "bar",
				},
			},
		},
		{
			name:               "no owner, take over if no diff, no diff",
			gvr:                &nsGVR,
			manifestObj:        nsUnstructured,
			inMemberClusterObj: nsUnstructured,
			applyStrategy: &fleetv1beta1.ApplyStrategy{
				WhenToTakeOver:   fleetv1beta1.WhenToTakeOverTypeIfNoDiff,
				ComparisonOption: fleetv1beta1.ComparisonOptionTypeFullComparison,
			},
			expectedAppliedWorkOwnerRef: appliedWorkOwnerRef,
			wantTakeOverObj:             wantTakenOverObj,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleDynamicClient(scheme.Scheme, tc.inMemberClusterObj)
			r := &Reconciler{
				spokeDynamicClient: fakeClient,
			}

			takenOverObj, patchDetails, err := r.takeOverPreExistingObject(
				ctx,
				tc.gvr,
				tc.manifestObj, tc.inMemberClusterObj,
				tc.applyStrategy,
				tc.expectedAppliedWorkOwnerRef)
			if tc.wantErred {
				if err == nil {
					t.Errorf("takeOverPreExistingObject() = nil, want erred")
				}
				return
			}

			if err != nil {
				t.Errorf("takeOverPreExistingObject() = %v, want no error", err)
			}
			if diff := cmp.Diff(takenOverObj, tc.wantTakeOverObj); diff != "" {
				t.Errorf("takenOverObject mismatches (-got, +want):\n%s", diff)
			}
			if diff := cmp.Diff(patchDetails, tc.wantPatchDetails); diff != "" {
				t.Errorf("patchDetails mismatches (-got, +want):\n%s", diff)
			}
		})
	}
}
