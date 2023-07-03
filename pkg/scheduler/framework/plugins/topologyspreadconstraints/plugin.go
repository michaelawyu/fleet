/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

// Package topologyspreadconstraints features a scheduler plugin that enforces the
// topology spread constraints (if any) defined on a CRP.
package topologyspreadconstraints

import (
	"context"

	"go.goms.io/fleet/pkg/scheduler/framework"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
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

// PostBatch allows the plugin to connect to the PostBatch extension point in the scheduling
// framework.
func (p *TopologySpreadConstraintsPlugin) PostBatch(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
) (int, *framework.Status) {
	// Not yet implemented.
	return 0, nil
}

// PreFilter allows the plugin to connect to the PreFilter extension point in the scheduling
// framework.
func (p *TopologySpreadConstraintsPlugin) PreFilter(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
) (status *framework.Status) {
	// Not yet implemented.
	return nil
}

// Filter allows the plugin to connect to the Filter extension point in the scheduling framework.
func (p *TopologySpreadConstraintsPlugin) Filter(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
	cluster *fleetv1beta1.MemberCluster,
) (status *framework.Status) {
	// Not yet implemented.
	return nil
}

// PreScore allows the plugin to connect to the PreScore extension point in the scheduling
// framework.
func (p *TopologySpreadConstraintsPlugin) PreScore(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
) (status *framework.Status) {
	// Not yet implemented.
	return nil
}

// Score allows the plugin to connect to the Score extension point in the scheduling framework.
func (p *TopologySpreadConstraintsPlugin) Score(
	ctx context.Context,
	state framework.CycleStatePluginReadWriter,
	policy *fleetv1beta1.ClusterSchedulingPolicySnapshot,
	cluster *fleetv1beta1.MemberCluster,
) (score *framework.ClusterScore, status *framework.Status) {
	// Not yet implemented.
	return nil, nil
}
