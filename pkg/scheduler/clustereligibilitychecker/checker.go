/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

// Package clustereligibilitychecker features a utility for verifying if a member cluster is
// eligible for resource placement.
package clustereligibilitychecker

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1beta1 "go.goms.io/fleet/apis/placement/v1beta1"
)

const (
	// defaultClusterHeartbeatTimeout is the default timeout value this checker uses for checking
	// if a cluster has been disconnected from the fleet for a prolonged period of time.
<<<<<<< HEAD
	defaultClusterHeartbeatCheckTimeout = time.Minute * 5
=======
	defaultClusterHeartbeatTimeout = time.Minute * 5
>>>>>>> 464da6ba3fdf37486efd8e997f0a6432fd4e4e7b

	// defaultClusterHealthCheckTimeout is the default timeout value this checker uses for checking
	// if a cluster is still in a healthy state.
	defaultClusterHealthCheckTimeout = time.Minute * 5
)

type ClusterEligibilityChecker struct {
<<<<<<< HEAD
	// clusterHeartbeatCheckTimeout is the timeout value this checker uses for checking if a cluster
	// has been disconnected from the fleet for a prolonged period of time.
	clusterHeartbeatCheckTimeout time.Duration

	// clusterHealthCheckTimeout is the timeout value this checker uses for checking if a cluster is
	// still in a healthy state.
=======
	// The timeout value this checker uses for checking if a cluster has been disconnected
	// from the fleet for a prolonged period of time.
	clusterHeartbeatTimeout time.Duration

	// The timeout value this checker uses for checking if a cluster is still in a healthy state.
>>>>>>> 464da6ba3fdf37486efd8e997f0a6432fd4e4e7b
	clusterHealthCheckTimeout time.Duration
}

// checkerOptions is the options for this checker.
type checkerOptions struct {
<<<<<<< HEAD
	// clusterHeartbeatCheckTimeout is the timeout value this checker uses for checking if a cluster
	// has been disconnected from the fleet for a prolonged period of time.
	clusterHeartbeatCheckTimeout time.Duration

	// clusterHealthCheckTimeout is the timeout value this checker uses for checking if a cluster is
	// still in a healthy state.
=======
	// The timeout value this checker uses for checking if a cluster has been disconnected
	// from the fleet for a prolonged period of time.
	clusterHeartbeatTimeout time.Duration

	// The timeout value this checker uses for checking if a cluster is still in a healthy state.
>>>>>>> 464da6ba3fdf37486efd8e997f0a6432fd4e4e7b
	clusterHealthCheckTimeout time.Duration
}

// Option helps set up the plugin.
type Option func(*checkerOptions)

// WithClusterHeartbeatTimeout sets the timeout value this plugin uses for checking
// if a cluster has been disconnected from the fleet for a prolonged period of time.
func WithClusterHeartbeatTimeout(timeout time.Duration) Option {
	return func(o *checkerOptions) {
<<<<<<< HEAD
		o.clusterHeartbeatCheckTimeout = timeout
=======
		o.clusterHeartbeatTimeout = timeout
>>>>>>> 464da6ba3fdf37486efd8e997f0a6432fd4e4e7b
	}
}

// WithClusterHealthCheckTimeout sets the timeout value this plugin uses for checking
// if a cluster is still in a healthy state.
func WithClusterHealthCheckTimeout(timeout time.Duration) Option {
	return func(o *checkerOptions) {
		o.clusterHealthCheckTimeout = timeout
	}
}

// defaultPluginOptions is the default options for this plugin.
var defaultCheckerOptions = checkerOptions{
<<<<<<< HEAD
	clusterHeartbeatCheckTimeout: defaultClusterHeartbeatCheckTimeout,
	clusterHealthCheckTimeout:    defaultClusterHealthCheckTimeout,
=======
	clusterHeartbeatTimeout:   defaultClusterHeartbeatTimeout,
	clusterHealthCheckTimeout: defaultClusterHealthCheckTimeout,
>>>>>>> 464da6ba3fdf37486efd8e997f0a6432fd4e4e7b
}

// New returns a new cluster eligibility checker.
func New(opts ...Option) *ClusterEligibilityChecker {
	options := defaultCheckerOptions
	for _, opt := range opts {
		opt(&options)
	}

	return &ClusterEligibilityChecker{
<<<<<<< HEAD
		clusterHeartbeatCheckTimeout: options.clusterHeartbeatCheckTimeout,
		clusterHealthCheckTimeout:    options.clusterHealthCheckTimeout,
=======
		clusterHeartbeatTimeout:   options.clusterHeartbeatTimeout,
		clusterHealthCheckTimeout: options.clusterHealthCheckTimeout,
>>>>>>> 464da6ba3fdf37486efd8e997f0a6432fd4e4e7b
	}
}

// IsEligible returns if a cluster is eligible for resource placement; if not, it will
// also return the reason.
func (checker *ClusterEligibilityChecker) IsEligible(cluster *fleetv1beta1.MemberCluster) (eligible bool, reason string) {
	// Filter out clusters that have left the fleet.
	if cluster.Spec.State == fleetv1beta1.ClusterStateLeave {
		return false, "cluster has left the fleet"
	}

	// Note that the following checks are performed against one specific agent, i.e., the member
	// agent, which is critical for the work orchestration related tasks in the fleet; non-related
	// agents (e.g., networking) are not accounted for in this plugin.

	// Filter out clusters that are no longer connected to the fleet, i.e., its heartbeat signals
	// have stopped for a prolonged period of time.
	memberAgentStatus := cluster.GetAgentStatus(fleetv1beta1.MemberAgent)
	if memberAgentStatus == nil {
		// The member agent has not updated its status with the hub cluster yet.
		return false, "cluster is not connected to the fleet: member agent not online yet"
	}

	sinceLastHeartbeat := time.Since(memberAgentStatus.LastReceivedHeartbeat.Time)
	if sinceLastHeartbeat > checker.clusterHeartbeatCheckTimeout {
		// The member agent has not sent heartbeat signals for a prolonged period of time.
		//
		// Note that this plugin assumes minimum clock drifts between clusters in the fleet.
		return false, fmt.Sprintf("cluster is not connected to the fleet: no recent heartbeat signals (last received %.2f minutes ago)", sinceLastHeartbeat.Minutes())
	}

	memberAgentJoinedCond := cluster.GetAgentCondition(fleetv1beta1.MemberAgent, fleetv1beta1.AgentJoined)
	if memberAgentJoinedCond == nil || memberAgentJoinedCond.Status != metav1.ConditionTrue {
		// The member agent has not joined yet; i.e., some of the controllers have not been
		// spun up.
		//
		// Note that here no generation check is performed, as
		// a) the member cluster object spec is most of the time not touched after creation; and
		// b) as long as the heartbeat signal does not timeout, a little drift in genrations
		//    should not exclude a cluster from resource scheduling.
		return false, "cluster is not connected to the fleet: member agent not joined yet"
	}

	memberAgentHealthyCond := cluster.GetAgentCondition(fleetv1beta1.MemberAgent, fleetv1beta1.AgentHealthy)
	if memberAgentHealthyCond == nil {
		// The health condition is absent.
<<<<<<< HEAD
		return false, "cluster is not connected to the fleet: health condition from member agent is not available"
=======
		return false, "cluster is not connected to the fleet: health condition not available"
>>>>>>> 464da6ba3fdf37486efd8e997f0a6432fd4e4e7b
	}

	sinceLastTransition := time.Since(memberAgentHealthyCond.LastTransitionTime.Time)
	if memberAgentHealthyCond.Status != metav1.ConditionTrue && sinceLastTransition > checker.clusterHealthCheckTimeout {
		// The cluster health check fails.
		//
		// Note that sporadic (isolated) health check failures will not preclude a cluster.
		//
		// Also note that at this moment, the member cluster will report a cluster as unhealth if
		// and only if it fails to list all the nodes in the cluster; this could simply be the
		// result of a temporary network issue, and the report will be disregarded by this plugin
		// if the health check passes within a reasonable amount of time.
		//
		// Note that this plugin assumes minimum clock drifts between clusters in the fleet.
		//
		// Also note that no generation check is performed here, for the same reason as above.
		return false, fmt.Sprintf("cluster is not connected to the fleet: unhealthy for a prolonged period of time (last transitioned %.2f minutes ago)", sinceLastTransition.Minutes())
	}

	return true, ""
}
