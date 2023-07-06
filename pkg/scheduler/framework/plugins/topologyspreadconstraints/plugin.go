/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

// Package topologyspreadconstraints features a scheduler plugin that enforces the
// topology spread constraints (if any) defined on a CRP.
package topologyspreadconstraints

import (
	"context"
	"fmt"

	"go.goms.io/fleet/pkg/scheduler/framework"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

const (
	skewChangeScoreFactor    = 1
	maxSkewViolationPenality = 1000
)

var (
	doNotScheduleConstraintViolationReasonTemplate = "violated topology spread constraint %q (max skew %d)"
)

// TopologySpreadConstraintsPlugin is the scheduler plugin that enforces the
// topology spread constraints (if any) defined on a CRP.
type TopologySpreadConstraintsPlugin struct {
	// The name of the plugin.
	name string

	// The framework handle.
	handle framework.Handle
}

var (
	// Verify that TopologySpreadConstraintsPlugin can connect to revelant extension points
	// at compile time.
	//
	// This plugin leverages the following the extension points:
	// * PostBatch
	// * PreFilter
	// * Filter
	// * PreScore
	// * Score
	_ framework.PostBatchPlugin = &TopologySpreadConstraintsPlugin{}
	_ framework.PreFilterPlugin = &TopologySpreadConstraintsPlugin{}
	_ framework.FilterPlugin    = &TopologySpreadConstraintsPlugin{}
	_ framework.PreScorePlugin  = &TopologySpreadConstraintsPlugin{}
	_ framework.ScorePlugin     = &TopologySpreadConstraintsPlugin{}
)

type topologySpreadConstraintsPluginOptions struct {
	// The name of the plugin.
	name string
}

type Option func(*topologySpreadConstraintsPluginOptions)

var defaultTopologySpreadConstraintsPluginOptions = topologySpreadConstraintsPluginOptions{
	name: "TopologySpreadConstraints",
}

// WithName sets the name of the plugin.
func WithName(name string) Option {
	return func(o *topologySpreadConstraintsPluginOptions) {
		o.name = name
	}
}

// New returns a new TopologySpreadConstraintsPlugin.
func New(opts ...Option) TopologySpreadConstraintsPlugin {
	options := defaultTopologySpreadConstraintsPluginOptions
	for _, opt := range opts {
		opt(&options)
	}

	return TopologySpreadConstraintsPlugin{
		name: options.name,
	}
}

// Name returns the name of the plugin.
func (p *TopologySpreadConstraintsPlugin) Name() string {
	return p.name
}

// SetUpWithFramework sets up this plugin with a scheduler framework.
func (p *TopologySpreadConstraintsPlugin) SetUpWithFramework(handle framework.Handle) {
	p.handle = handle

	// This plugin does not need to set up any informer.
}

// readPluginState reads the plugin state from the cycle state.
func (p *TopologySpreadConstraintsPlugin) readPluginState(state framework.CycleStatePluginReadWriter) (*topologySpreadConstraintsPluginState, error) {
	// Read from the cycle state.
	val, err := state.Read(framework.StateKey(p.Name()))
	if err != nil {
		return nil, fmt.Errorf("failed to read value from the cycle state: %w", err)
	}

	// Cast the value to the right type.
	pluginState, ok := val.(*topologySpreadConstraintsPluginState)
	if !ok {
		return nil, fmt.Errorf("failed to cast value %v to the right type", val)
	}
	return pluginState, nil
}

// PostBatch allows the plugin to connect to the PostBatch extension point in the scheduling
// framework.
//
// Note that the scheduler will not run this extension point in parallel.
func (p *TopologySpreadConstraintsPlugin) PostBatch(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
) (int, *framework.Status) {
	if len(policy.Spec.Policy.TopologySpreadConstraints) == 0 {
		// There are no topology spread constraints to enforce; skip.
		return 0, framework.NewNonErrorStatus(framework.Skip, p.Name(), "no topology spread constraint is present")
	}

	// Prepare some common states for future use. This helps avoid the cost of repeatedly
	// calculating the same states at each extension point.
	//
	// Note that this will happen as long as there is one or more topology spread constraints
	// in presence in the scheduling policy, regardless of its settings.
	pluginState, err := prepareTopologySpreadConstraintsPluginState(state, policy)
	if err != nil {
		return 0, framework.FromError(err, p.Name(), "failed to prepare plugin state")
	}

	// Save the plugin state.
	state.Write(framework.StateKey(p.Name()), pluginState)

	// Set a batch limit of 1 if there are topology spread constraints in presence.
	//
	// TO-DO (chenyu1): when there is only one single topology spread constraint, it is possible
	// to calculate the optimal spread in one go as an optimization.
	return 1, nil
}

// PreFilter allows the plugin to connect to the PreFilter extension point in the scheduling
// framework.
//
// Note that the scheduler will not run this extension point in parallel.
func (p *TopologySpreadConstraintsPlugin) PreFilter(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
) (status *framework.Status) {
	if len(policy.Spec.Policy.TopologySpreadConstraints) == 0 {
		// There are no topology spread constraints to enforce; skip.
		//
		// Note that this will lead the scheduler to skip this plugin in the next stage
		// (Filter).
		return framework.NewNonErrorStatus(framework.Skip, p.Name(), "no topology spread constraint is present")
	}

	// Read the plugin state.
	pluginState, err := p.readPluginState(state)
	if err != nil {
		// This branch should never be reached, as the plugin state has been set at the very
		// first extension point.
		return framework.FromError(err, p.Name(), "failed to read plugin state")
	}

	if len(pluginState.doNotScheduleConstraints) == 0 {
		// There are no DoNotSchedule topology spread constraints to enforce; skip.
		//
		// Note that this will lead the scheduler to skip this plugin in the next stage
		// (Filter).
		return framework.NewNonErrorStatus(framework.Skip, p.Name(), "no DoNotSchedule topology spread constraint is present")
	}

	// All done.
	return nil
}

// Filter allows the plugin to connect to the Filter extension point in the scheduling framework.
func (p *TopologySpreadConstraintsPlugin) Filter(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
	cluster *fleetv1beta1.MemberCluster,
) (status *framework.Status) {
	// Read the plugin state.
	pluginState, err := p.readPluginState(state)
	if err != nil {
		// This branch should never be reached, as the plugin state has been set at the very
		// first extension point.
		return framework.FromError(err, p.Name(), "failed to read plugin state")
	}

	// The state is safe for concurrent reads.
	reasons, ok := pluginState.violations[clusterName(cluster.Name)]
	if ok {
		// Violation is found; filter this cluster out.
		return framework.NewNonErrorStatus(framework.ClusterUnschedulable, p.Name(), reasons...)
	}

	// No violation is found; all done.
	return nil
}

// PreScore allows the plugin to connect to the PreScore extension point in the scheduling
// framework.
func (p *TopologySpreadConstraintsPlugin) PreScore(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
) (status *framework.Status) {
	// Read the plugin state.
	pluginState, err := p.readPluginState(state)
	if err != nil {
		// This branch should never be reached, as the plugin state has been set at the very
		// first extension point.
		return framework.FromError(err, p.Name(), "failed to read plugin state")
	}

	if len(pluginState.scheduleAnywayConstraints) == 0 && len(pluginState.doNotScheduleConstraints) == 0 {
		// There are no topology spread constraints to enforce; skip.
		//
		// Note that this will lead the scheduler to skip this plugin in the next stage
		// (Score).
		//
		// Also note that the plugin will still score clusters even if there are only DoNotSchedule
		// topology spread constraints in presence; this can help reduce the skew (if applicable).
		return framework.NewNonErrorStatus(framework.Skip, p.Name(), "no topology spread constraint is present")
	}

	// All done.
	return nil
}

// Score allows the plugin to connect to the Score extension point in the scheduling framework.
func (p *TopologySpreadConstraintsPlugin) Score(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
	cluster *fleetv1beta1.MemberCluster,
) (score *framework.ClusterScore, status *framework.Status) {
	// Read the plugin state.
	pluginState, err := p.readPluginState(state)
	if err != nil {
		// This branch should never be reached, as the plugin state has been set at the very
		// first extension point.
		return nil, framework.FromError(err, p.Name(), "failed to read plugin state")
	}

	// The state is safe for concurrent reads.
	topologySpreadScore, ok := pluginState.scores[clusterName(cluster.Name)]
	if !ok {
		// No score is found; normally this should never happen, as the state is consistent,
		// and each cluster that does not violate DoNotSchedule topology spread constraints
		// will have a score assigned.
		return nil, framework.FromError(fmt.Errorf("no score found for cluster %s", cluster.Name), p.Name())
	}

	// Return a ClusterScore.
	score = &framework.ClusterScore{
		TopologySpreadScore: topologySpreadScore,
	}
	// All done.
	return score, nil
}
