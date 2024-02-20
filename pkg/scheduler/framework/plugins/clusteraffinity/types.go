/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package clusteraffinity

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	clusterv1beta1 "go.goms.io/fleet/apis/cluster/v1beta1"
	placementv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

// extractResourceMetricDataFrom extracts a specific piece of resource metric data (usually
// a resource quantity) from the reported resource usage.
//
// This function facilitates arbitrary lookup of resource usage data (reported in a structured
// form) using a label name of a specific format.
func extractResourceMetricDataFrom(usage *clusterv1beta1.ResourceUsage, name string) (resource.Quantity, error) {
	capacityType, resourceName, found := strings.Cut(name, resourceMetricLabelNameSep)
	// Do some sanity checks.
	if !found || len(capacityType) == 0 || len(resourceName) == 0 {
		return resource.Quantity{}, fmt.Errorf("failed to parse resource metric name %s: the name is not of the correct format", name)
	}

	if usage == nil {
		return resource.Quantity{}, fmt.Errorf("failed to look up resource metric: the resource usage is nil")
	}

	// Use reflection to query the capacity type in the resource usage object.
	usageVal := reflect.ValueOf(usage).Elem()
	for i := 0; i < usageVal.NumField(); i++ {
		if strings.ToLower(usageVal.Type().Field(i).Name) == capacityType {
			// Found the capacity type; verify if it stores a resource list.
			capacityVal := usageVal.Field(i)
			if capacityVal.Kind() != reflect.Map {
				// The capacity type does not store a resource list.
				//
				// Normally this branch will never run.
				return resource.Quantity{}, fmt.Errorf("failed to look up resource metric name %s: the capacity type is not valid", name)
			}

			// Query the resource list for the resource name.
			for _, k := range capacityVal.MapKeys() {
				if k.Kind() == reflect.String && k.String() == resourceName {
					// Found the resource name; serialize the quantity.
					v := capacityVal.MapIndex(k)
					qt := reflect.TypeOf(resource.Quantity{})
					if !v.Type().AssignableTo(qt) {
						// The resource name exists, but it does not store a quantity.
						//
						// Normally this branch will never run.
						return resource.Quantity{}, fmt.Errorf("failed to look up resource metric name %s: the quantity is not valid (not a quantity type)", name)
					}

					// Retrieve a pointer to the quantity as the serialization method is only
					// available via the pointer type.
					vp := reflect.New(qt)
					vp.Elem().Set(v)
					strer := vp.MethodByName("String")
					if !strer.IsValid() {
						// A quantity for the resource exists, but it does not have the serialization method.
						//
						// Normally this branch will never run.
						return resource.Quantity{}, fmt.Errorf("failed to look up resource metric name %s: the quantity is not valid (no serialization available)", name)
					}

					s := strer.Call([]reflect.Value{})
					if len(s) != 1 || s[0].Kind() != reflect.String {
						// The serialization method does not return the expected value.
						//
						// Normally this branch will never run.
						return resource.Quantity{}, fmt.Errorf("failed to look up resource metric name %s: the quantity is not valid (serialization failed)", name)
					}

					// Deserialize the quantity.
					q, parseErr := resource.ParseQuantity(s[0].String())
					if parseErr != nil {
						return resource.Quantity{}, fmt.Errorf("failed to look up resource metric name %s:  the quantity is not valid (deserialization failed: %v)", name, parseErr)
					}

					return q, nil
				}
			}
			return resource.Quantity{}, fmt.Errorf("failed to look up resource metric name %s: the resource name does not exist", name)
		}
	}
	return resource.Quantity{}, fmt.Errorf("failed to look up resource metric name %s: the capacity type does not exist", name)
}

// metricSelector wraps the MetricMatcher API type in a struct that implements specific methods
// that help match metric requirements and/or preferences against clusters.
type metricSelector struct {
	metricMatchers []placementv1beta1.MetricMatcher
}

// matchResourceMetricWith matches a resource metric requirement against the observed resource
// usage data.
func matchResourceMetricWith(matcher *placementv1beta1.MetricMatcher, cluster *clusterv1beta1.MemberCluster) (bool, error) {
	// Remove the label prefix.
	metricName := strings.Replace(matcher.Name, resourceMetricLabelPrefix, "", 1)
	// Extract the resource metric data.
	q, err := extractResourceMetricDataFrom(&cluster.Status.ResourceUsage, metricName)
	if err != nil {
		return false, fmt.Errorf("failed to match metric selector %s: %w", matcher.Name, err)
	}

	for _, r := range matcher.Ranges {
		// Parse the given minimum and maximum values as resource quantities.
		//
		// Normally the values are already validated by the validation webhook; here
		// the plugin performs an additional sanity check just to be certain.
		isMinimumCheckPassed := true
		if len(r.Minimum) > 0 {
			// The minimum is set; if not, the minimum requirement is always satisfied.
			minQ, err := resource.ParseQuantity(r.Minimum)
			if err != nil {
				return false, fmt.Errorf("failed to match metric selector %s: minimum value %v cannot be parsed as a resource quantity: %w", matcher.Name, r.Minimum, err)
			}

			if q.Cmp(minQ) < 0 {
				isMinimumCheckPassed = false
			}
		}

		isMaximumCheckPassed := true
		if len(r.Maximum) > 0 {
			maxQ, err := resource.ParseQuantity(r.Maximum)
			if err != nil {
				return false, fmt.Errorf("failed to match metric selector %s: maximum value %v cannot be parsed as a resource quantity: %w", matcher.Name, r.Maximum, err)
			}

			if q.Cmp(maxQ) > 0 {
				isMaximumCheckPassed = false
			}
		}

		// Ignore the interpolation related fields. Validation should be performed
		// in the upper levels instead.

		if isMinimumCheckPassed && isMaximumCheckPassed {
			// The current data falls in the given range.
			//
			// Note that when there are multiple ranges specified for the same metric name,
			// the requirements are OR'd.
			return true, nil
		}
	}

	// The specific requirement cannot be satisified; or, in other words, for the given
	// resource metric requirements, the observed data does not fall within any of the specified
	// ranges.
	return false, nil
}

func matchNonResourceMetricWith(matcher *placementv1beta1.MetricMatcher, cluster *clusterv1beta1.MemberCluster) (bool, error) {
	// Query the given metric.
	s, ok := cluster.Status.Metrics[clusterv1beta1.MetricName(matcher.Name)]
	if !ok {
		// The metric is absent.
		//
		// A cluster with the required metric data missing is considered ineligible for
		// placement.
		return false, nil
	}

	// Verify is the observed data falls within the given range.

	// Cast the observed value from the string form to the float form. Normally this cast
	// is guaranteed to be successful as dictated by the metric provider interface; however,
	// here this plugin still runs an additional check just to be on the safer side.
	f, err := strconv.ParseFloat(string(s), 64)
	if err != nil {
		return false, fmt.Errorf("failed to match metric selector %s: the observed value %v cannot be parsed as a float: %w", matcher.Name, s, err)
	}

	for _, r := range matcher.Ranges {
		// Similarly, cast the given ranges from the string form to the float form. The correctness
		// of this operation should be guaranteed by the validation webhook; however, here
		// this plugin still runs an additional check just to be on the safer side.

		isMinimumCheckPassed := true
		if len(r.Minimum) > 0 {
			minF, err := strconv.ParseFloat(r.Minimum, 64)
			if err != nil {
				return false, fmt.Errorf("failed to match metric selector %s: minimum value %v cannot be parsed as a float: %w", matcher.Name, r.Minimum, err)
			}

			if f < minF {
				isMinimumCheckPassed = false
			}
		}

		isMaximumCheckPassed := true
		if len(r.Maximum) > 0 {
			maxF, err := strconv.ParseFloat(r.Maximum, 64)
			if err != nil {
				return false, fmt.Errorf("failed to match metric selector %s: maximum value %v cannot be parsed as a float: %w", matcher.Name, r.Maximum, err)
			}

			if f > maxF {
				isMaximumCheckPassed = false
			}
		}

		// Ignore the interpolation related fields. Validation should be performed
		// in the upper levels instead.
		if isMinimumCheckPassed && isMaximumCheckPassed {
			// The current data falls in the given range.
			//
			// Note that when there are multiple ranges specified for the same metric name,
			// the requirements are OR'd.
			return true, nil
		}
	}

	// The specific requirement cannot be satisified; or, in other words, for the given
	// non-resource metric requirements, the observed data does not fall within any of the specified
	// ranges.
	return false, nil
}

// Matches returns true if a metric selector can select a member cluster, i.e., the member cluster
// satisfies all the given metric requirements.
func (ms *metricSelector) Matches(cluster *clusterv1beta1.MemberCluster) (bool, error) {
	for _, matcher := range ms.metricMatchers {
		if strings.HasPrefix(matcher.Name, resourceMetricLabelPrefix) {
			// The selector attempts to select a resource metric.
			if isMatched, err := matchResourceMetricWith(&matcher, cluster); !isMatched || err != nil {
				// Multiple requirements in the same affinity term are AND'd; short-circuit
				// if a requirement is not satisfied.
				return isMatched, err
			}
		} else {
			// The selector attempts to select a non-resource metric.
			if isMatched, err := matchNonResourceMetricWith(&matcher, cluster); !isMatched || err != nil {
				// Similarly, short-circuit if a requirement is not satisfied.
				return isMatched, err
			}
		}
	}

	// All requirements are satisfied.
	return true, nil
}

// affinityTerm is a processed version of ClusterSelectorTerm.
type affinityTerm struct {
	lbls labels.Selector
	mets metricSelector
}

// Matches returns true if the cluster matches the specified selectors (label selectors and/or
// metric selectors).
func (at *affinityTerm) Matches(cluster *clusterv1beta1.MemberCluster) (bool, error) {
	isLabelSelectorsMatched := at.lbls.Matches(labels.Set(cluster.Labels))
	isMetricSelectorsMatched, err := at.mets.Matches(cluster)
	if err != nil {
		return false, err
	}

	return isLabelSelectorsMatched && isMetricSelectorsMatched, nil
}

// AffinityTerms is a "processed" representation of []ClusterSelectorTerms.
// The terms are `ORed`.
type AffinityTerms []affinityTerm

// Matches returns true if the cluster matches one of the terms.
func (at AffinityTerms) Matches(cluster *clusterv1beta1.MemberCluster) (bool, error) {
	for _, term := range at {
		isMatched, err := term.Matches(cluster)
		if err != nil {
			return false, err
		}

		if isMatched {
			// Multiple affinity terms are OR'd; short-circuit if any of the term is satisfied.
			return true, nil
		}
	}
	return false, nil
}

// preferredAffinityTerm is a "processed" representation of PreferredClusterSelector.
type preferredAffinityTerm struct {
	affinityTerm
	weight int32
}

func (pat *preferredAffinityTerm) Weighs(cluster *clusterv1beta1.MemberCluster) (int32, error) {
	// Verify if weight interpolation is needed.
	isWeightInterpolationNeeded := false
	rc := 0
	for _, m := range pat.affinityTerm.mets.metricMatchers {
		for _, r := range m.Ranges {
			if r.Interpolate != placementv1beta1.DoNotInterpolate {
				isWeightInterpolationNeeded = true
			}
		}

		rc += len(m.Ranges)
	}

	if isWeightInterpolationNeeded {
		if rc > 1 {
			// Weight interpolation is enabled, yet there are multiple metric selector ranges
			// included in the affinity term; this is a case which is not supported by Fleet
			// scheduling.
			return 0, fmt.Errorf("multiple ranges are present with weight interpolation is enabled")
		}

		// Interpolate the weight.

	}

	return 0, nil
}

// PreferredAffinityTerms is a "processed" representation of []PreferredClusterSelector.
type PreferredAffinityTerms []preferredAffinityTerm

// Score returns a score for a cluster: the sum of the weights of the terms that match the cluster.
func (t PreferredAffinityTerms) Score(cluster *clusterv1beta1.MemberCluster) (int32, error) {
	var score int32
	for _, term := range t {
		isMatched, err := term.affinityTerm.Matches(cluster)
		if err != nil {
			return 0, err
		}

		if isMatched {
			ps, err := term.Weighs(cluster)
			if err != nil {
				return 0, err
			}
			score += ps
		}
	}
	return score, nil
}

func newAffinityTerm(term *placementv1beta1.ClusterSelectorTerm) (*affinityTerm, error) {
	labelSelector, err := metav1.LabelSelectorAsSelector(&term.LabelSelector)
	if err != nil {
		return nil, err
	}

	metricSelector := metricSelector{
		metricMatchers: term.MetricSelector.MatchMetrics,
	}
	return &affinityTerm{lbls: labelSelector, mets: metricSelector}, nil
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
	return len(term.LabelSelector.MatchLabels) == 0 && len(term.LabelSelector.MatchExpressions) == 0 && len(term.MetricSelector.MatchMetrics) == 0
}
