/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package framework

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCycleStateBasicOps tests the basic ops of a CycleState.
func TestCycleStateBasicOps(t *testing.T) {
	clusters := []fleetv1beta1.MemberCluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		},
	}

	cs := NewCycleState(clusters)

	k, v := "key", "value"
	cs.Write(StateKey(k), StateValue(v))
	if out, err := cs.Read("key"); out != "value" || err != nil {
		t.Fatalf("Read(%v) = %v, %v, want %v, nil", k, out, err, v)
	}
	cs.Delete(StateKey(k))
	if out, err := cs.Read("key"); out != nil || err == nil {
		t.Fatalf("Read(%v) = %v, %v, want nil, not found error", k, out, err)
	}

	clustersInState := cs.ListClusters()
	if diff := cmp.Diff(clustersInState, clusters); diff != "" {
		t.Fatalf("ListClusters() diff (-got, +want): %s", diff)
	}
}
