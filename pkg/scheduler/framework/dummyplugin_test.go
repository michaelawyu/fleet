/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package framework

import (
	"context"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

const (
	dummyAllPurposePluginNameFormat = "dummyAllPurposePlugin-%d"
)

// A no-op, dummy plugin which connects to all extension points.
type DummyAllPurposePlugin struct {
	name            string
	postBatchRunner func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status)
	preFilterRunner func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status)
	filterRunner    func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status)
	preScoreRunner  func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status)
	scoreRunner     func(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status)
}

// Name returns the name of the dummy plugin.
func (p *DummyAllPurposePlugin) Name() string {
	return p.name
}

// PostBatch implements the PostBatch interface for the dummy plugin.
func (p *DummyAllPurposePlugin) PostBatch(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (size int, status *Status) { //nolint:revive
	return p.postBatchRunner(ctx, state, policy)
}

// PreFilter implements the PreFilter interface for the dummy plugin.
func (p *DummyAllPurposePlugin) PreFilter(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status) { //nolint:revive
	return p.preFilterRunner(ctx, state, policy)
}

// Filter implements the Filter interface for the dummy plugin.
func (p *DummyAllPurposePlugin) Filter(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (status *Status) { //nolint:revive
	return p.filterRunner(ctx, state, policy, cluster)
}

// PreScore implements the PreScore interface for the dummy plugin.
func (p *DummyAllPurposePlugin) PreScore(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot) (status *Status) { //nolint:revive
	return p.preScoreRunner(ctx, state, policy)
}

// Score implements the Score interface for the dummy plugin.
func (p *DummyAllPurposePlugin) Score(ctx context.Context, state CycleStatePluginReadWriter, policy *fleetv1beta1.ClusterPolicySnapshot, cluster *fleetv1beta1.MemberCluster) (score *ClusterScore, status *Status) { //nolint:revive
	return p.scoreRunner(ctx, state, policy, cluster)
}

// SetUpWithFramework is a no-op to satisfy the Plugin interface.
func (p *DummyAllPurposePlugin) SetUpWithFramework(handle Handle) {} // nolint:revive
