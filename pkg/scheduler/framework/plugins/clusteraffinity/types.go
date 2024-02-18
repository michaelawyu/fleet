/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package clusteraffinity

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	placementv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

// metricSelector wraps the MetricMatcher API type in a struct that implements specific methods
// that help match metric requirements and/or preferences against clusters.
type metricSelector struct {
	metricMatchers []placementv1beta1.MetricMatcher
}

func (ms *metricSelector) Matches(cluster *clusterv1beta1.MemberCluster) (bool, error) {
	for _, matcher := range ms.metricMatchers {
		mv, ok := cluster.Status.Metrics[clusterv1beta1.MetricName(matcher.Name)]
	}
}

// affinityTerm is a processed version of ClusterSelectorTerm.
type affinityTerm struct {
	lbls labels.Selector
	mets metricSelector
}

// Matches returns true if the cluster matches the specified selectors (label selectors and/or
// metric selectors).
func (at *affinityTerm) Matches(cluster *clusterv1beta1.MemberCluster) bool {
	return at.lbls.Matches(labels.Set(cluster.Labels)) && at.mets.Matches(cluster)
}

// AffinityTerms is a "processed" representation of []ClusterSelectorTerms.
// The terms are `ORed`.
type AffinityTerms []affinityTerm

// Matches returns true if the cluster matches one of the terms.
func (at AffinityTerms) Matches(cluster *clusterv1beta1.MemberCluster) bool {
	for _, term := range at {
		if term.Matches(cluster) {
			return true
		}
	}
	return false
}

// preferredAffinityTerm is a "processed" representation of PreferredClusterSelector.
type preferredAffinityTerm struct {
	affinityTerm
	weight int32
}

// PreferredAffinityTerms is a "processed" representation of []PreferredClusterSelector.
type PreferredAffinityTerms []preferredAffinityTerm

// Score returns a score for a cluster: the sum of the weights of the terms that match the cluster.
func (t PreferredAffinityTerms) Score(cluster *clusterv1beta1.MemberCluster) int32 {
	var score int32
	for _, term := range t {
		if term.affinityTerm.Matches(cluster) {
			score += term.weight
		}
	}
	return score
}

func newAffinityTerm(term *placementv1beta1.ClusterSelectorTerm) (*affinityTerm, error) {
	selector, err := metav1.LabelSelectorAsSelector(&term.LabelSelector)
	if err != nil {
		return nil, err
	}
	return &affinityTerm{selector: selector}, nil
}

// NewAffinityTerms returns the list of processed affinity terms.
func NewAffinityTerms(terms []placementv1beta1.ClusterSelectorTerm) (AffinityTerms, error) {
	res := make([]affinityTerm, 0, len(terms))
	for i := range terms {
		// skipping for empty terms
		if isEmptyClusterSelectorTerm(terms[i]) {
			continue
		}
		t, err := newAffinityTerm(&terms[i])
		if err != nil {
			// We get here if the label selector failed to process
			return nil, err
		}
		res = append(res, *t)
	}
	return res, nil
}

// NewPreferredAffinityTerms returns the list of processed preferred affinity terms.
func NewPreferredAffinityTerms(terms []placementv1beta1.PreferredClusterSelector) (PreferredAffinityTerms, error) {
	res := make([]preferredAffinityTerm, 0, len(terms))
	for i, term := range terms {
		// skipping for weight == 0 or empty terms
		if term.Weight == 0 || isEmptyClusterSelectorTerm(term.Preference) {
			continue
		}
		t, err := newAffinityTerm(&term.Preference)
		if err != nil {
			// We get here if the label selector failed to process
			return nil, err
		}
		res = append(res, preferredAffinityTerm{affinityTerm: *t, weight: terms[i].Weight})
	}
	return res, nil
}

func isEmptyClusterSelectorTerm(term placementv1beta1.ClusterSelectorTerm) bool {
	return len(term.LabelSelector.MatchLabels) == 0 && len(term.LabelSelector.MatchExpressions) == 0
}
