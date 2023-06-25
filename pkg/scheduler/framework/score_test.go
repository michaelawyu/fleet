/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package framework

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

func TestClusterScoreAdd(t *testing.T) {
	s1 := &ClusterScore{
		TopologySpreadScore:          0,
		AffinityScore:                0,
		ActiveOrCreatingBindingScore: 0,
	}

	s2 := &ClusterScore{
		TopologySpreadScore:          1,
		AffinityScore:                5,
		ActiveOrCreatingBindingScore: 1,
	}

	s1.Add(s2)
	want := &ClusterScore{
		TopologySpreadScore:          1,
		AffinityScore:                5,
		ActiveOrCreatingBindingScore: 1,
	}
	if !cmp.Equal(s1, want) {
		t.Fatalf("Add() = %v, want %v", s1, want)
	}
}

func TestClusterScoreLess(t *testing.T) {
	testCases := []struct {
		name string
		s1   *ClusterScore
		s2   *ClusterScore
		want bool
	}{
		{
			name: "s1 is less than s2 in topology spread score",
			s1: &ClusterScore{
				TopologySpreadScore: 0,
				AffinityScore:       10,
			},
			s2: &ClusterScore{
				TopologySpreadScore: 1,
				AffinityScore:       20,
			},
			want: true,
		},
		{
			name: "s1 is less than s2 in affinity score",
			s1: &ClusterScore{
				TopologySpreadScore: 1,
				AffinityScore:       10,
			},
			s2: &ClusterScore{
				TopologySpreadScore: 1,
				AffinityScore:       20,
			},
			want: true,
		},
		{
			name: "s1 is less than s2 in active or creating binding score",
			s1: &ClusterScore{
				TopologySpreadScore:          1,
				AffinityScore:                10,
				ActiveOrCreatingBindingScore: 0,
			},
			s2: &ClusterScore{
				TopologySpreadScore:          1,
				AffinityScore:                10,
				ActiveOrCreatingBindingScore: 1,
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.s1.Less(tc.s2) != tc.want {
				t.Fatalf("Less(%v, %v) = %t, want %t", tc.s1, tc.s2, !tc.want, tc.want)
			}

			if tc.s2.Less(tc.s1) != !tc.want {
				t.Fatalf("Less(%v, %v) = %t, want %t", tc.s2, tc.s1, tc.want, !tc.want)
			}
		})
	}
}

func TestClusterScoreEqual(t *testing.T) {
	s1 := &ClusterScore{
		TopologySpreadScore:          0,
		AffinityScore:                0,
		ActiveOrCreatingBindingScore: 0,
	}

	s2 := &ClusterScore{
		TopologySpreadScore:          0,
		AffinityScore:                0,
		ActiveOrCreatingBindingScore: 0,
	}

	if s1.Less(s2) || s2.Less(s1) {
		t.Fatalf("Less(%v, %v) = %v, Less(%v, %v) = %v, want both to be false", s1, s2, s1.Less(s2), s2, s1, s2.Less(s1))
	}
}

func TestScoredClustersSort(t *testing.T) {
	clusterA := &fleetv1beta1.MemberCluster{}
	clusterB := &fleetv1beta1.MemberCluster{}
	clusterC := &fleetv1beta1.MemberCluster{}
	clusterD := &fleetv1beta1.MemberCluster{}
	clusterE := &fleetv1beta1.MemberCluster{}

	testCases := []struct {
		name string
		scs  ScoredClusters
		want ScoredClusters
	}{
		{
			name: "sort asc values",
			scs: ScoredClusters{
				{
					Cluster: clusterB,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterA,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterC,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterD,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                20,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterE,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 0,
					},
				},
			},
			want: ScoredClusters{
				{
					Cluster: clusterE,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterD,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                20,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterC,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterA,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterB,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 0,
					},
				},
			},
		},
		{
			name: "sort desc values",
			scs: ScoredClusters{
				{
					Cluster: clusterD,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterE,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterC,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                20,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterB,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterA,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
			},
			want: ScoredClusters{
				{
					Cluster: clusterD,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterE,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterC,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                20,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterB,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterA,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
			},
		},
		{
			name: "sort values in random",
			scs: ScoredClusters{
				{
					Cluster: clusterC,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                20,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterD,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterA,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterE,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterB,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
			},
			want: ScoredClusters{
				{
					Cluster: clusterD,
					Score: &ClusterScore{
						TopologySpreadScore:          2,
						AffinityScore:                30,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterC,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                20,
						ActiveOrCreatingBindingScore: 0,
					},
				},
				{
					Cluster: clusterB,
					Score: &ClusterScore{
						TopologySpreadScore:          1,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterE,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 1,
					},
				},
				{
					Cluster: clusterA,
					Score: &ClusterScore{
						TopologySpreadScore:          0,
						AffinityScore:                10,
						ActiveOrCreatingBindingScore: 0,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sort.Sort(sort.Reverse(tc.scs))
			if !cmp.Equal(tc.scs, tc.want) {
				t.Fatalf("Sort() = %v, want %v", tc.scs, tc.want)
			}
		})
	}
}
