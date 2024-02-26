//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Affinity) DeepCopyInto(out *Affinity) {
	*out = *in
	if in.ClusterAffinity != nil {
		in, out := &in.ClusterAffinity, &out.ClusterAffinity
		*out = new(ClusterAffinity)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Affinity.
func (in *Affinity) DeepCopy() *Affinity {
	if in == nil {
		return nil
	}
	out := new(Affinity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppliedResourceMeta) DeepCopyInto(out *AppliedResourceMeta) {
	*out = *in
	out.WorkResourceIdentifier = in.WorkResourceIdentifier
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppliedResourceMeta.
func (in *AppliedResourceMeta) DeepCopy() *AppliedResourceMeta {
	if in == nil {
		return nil
	}
	out := new(AppliedResourceMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppliedWork) DeepCopyInto(out *AppliedWork) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppliedWork.
func (in *AppliedWork) DeepCopy() *AppliedWork {
	if in == nil {
		return nil
	}
	out := new(AppliedWork)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AppliedWork) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppliedWorkList) DeepCopyInto(out *AppliedWorkList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AppliedWork, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppliedWorkList.
func (in *AppliedWorkList) DeepCopy() *AppliedWorkList {
	if in == nil {
		return nil
	}
	out := new(AppliedWorkList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AppliedWorkList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppliedWorkSpec) DeepCopyInto(out *AppliedWorkSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppliedWorkSpec.
func (in *AppliedWorkSpec) DeepCopy() *AppliedWorkSpec {
	if in == nil {
		return nil
	}
	out := new(AppliedWorkSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppliedWorkStatus) DeepCopyInto(out *AppliedWorkStatus) {
	*out = *in
	if in.AppliedResources != nil {
		in, out := &in.AppliedResources, &out.AppliedResources
		*out = make([]AppliedResourceMeta, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppliedWorkStatus.
func (in *AppliedWorkStatus) DeepCopy() *AppliedWorkStatus {
	if in == nil {
		return nil
	}
	out := new(AppliedWorkStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplyStrategy) DeepCopyInto(out *ApplyStrategy) {
	*out = *in
	if in.ServerSideApplyConfig != nil {
		in, out := &in.ServerSideApplyConfig, &out.ServerSideApplyConfig
		*out = new(ServerSideApplyConfig)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplyStrategy.
func (in *ApplyStrategy) DeepCopy() *ApplyStrategy {
	if in == nil {
		return nil
	}
	out := new(ApplyStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterAffinity) DeepCopyInto(out *ClusterAffinity) {
	*out = *in
	if in.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		in, out := &in.RequiredDuringSchedulingIgnoredDuringExecution, &out.RequiredDuringSchedulingIgnoredDuringExecution
		*out = new(ClusterSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		in, out := &in.PreferredDuringSchedulingIgnoredDuringExecution, &out.PreferredDuringSchedulingIgnoredDuringExecution
		*out = make([]PreferredClusterSelector, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterAffinity.
func (in *ClusterAffinity) DeepCopy() *ClusterAffinity {
	if in == nil {
		return nil
	}
	out := new(ClusterAffinity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterDecision) DeepCopyInto(out *ClusterDecision) {
	*out = *in
	if in.ClusterScore != nil {
		in, out := &in.ClusterScore, &out.ClusterScore
		*out = new(ClusterScore)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterDecision.
func (in *ClusterDecision) DeepCopy() *ClusterDecision {
	if in == nil {
		return nil
	}
	out := new(ClusterDecision)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourceBinding) DeepCopyInto(out *ClusterResourceBinding) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourceBinding.
func (in *ClusterResourceBinding) DeepCopy() *ClusterResourceBinding {
	if in == nil {
		return nil
	}
	out := new(ClusterResourceBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterResourceBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourceBindingList) DeepCopyInto(out *ClusterResourceBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterResourceBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourceBindingList.
func (in *ClusterResourceBindingList) DeepCopy() *ClusterResourceBindingList {
	if in == nil {
		return nil
	}
	out := new(ClusterResourceBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterResourceBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourcePlacement) DeepCopyInto(out *ClusterResourcePlacement) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourcePlacement.
func (in *ClusterResourcePlacement) DeepCopy() *ClusterResourcePlacement {
	if in == nil {
		return nil
	}
	out := new(ClusterResourcePlacement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterResourcePlacement) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourcePlacementList) DeepCopyInto(out *ClusterResourcePlacementList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterResourcePlacement, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourcePlacementList.
func (in *ClusterResourcePlacementList) DeepCopy() *ClusterResourcePlacementList {
	if in == nil {
		return nil
	}
	out := new(ClusterResourcePlacementList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterResourcePlacementList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourcePlacementSpec) DeepCopyInto(out *ClusterResourcePlacementSpec) {
	*out = *in
	if in.ResourceSelectors != nil {
		in, out := &in.ResourceSelectors, &out.ResourceSelectors
		*out = make([]ClusterResourceSelector, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Policy != nil {
		in, out := &in.Policy, &out.Policy
		*out = new(PlacementPolicy)
		(*in).DeepCopyInto(*out)
	}
	in.Strategy.DeepCopyInto(&out.Strategy)
	if in.RevisionHistoryLimit != nil {
		in, out := &in.RevisionHistoryLimit, &out.RevisionHistoryLimit
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourcePlacementSpec.
func (in *ClusterResourcePlacementSpec) DeepCopy() *ClusterResourcePlacementSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterResourcePlacementSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourcePlacementStatus) DeepCopyInto(out *ClusterResourcePlacementStatus) {
	*out = *in
	if in.SelectedResources != nil {
		in, out := &in.SelectedResources, &out.SelectedResources
		*out = make([]ResourceIdentifier, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.PlacementStatuses != nil {
		in, out := &in.PlacementStatuses, &out.PlacementStatuses
		*out = make([]ResourcePlacementStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourcePlacementStatus.
func (in *ClusterResourcePlacementStatus) DeepCopy() *ClusterResourcePlacementStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterResourcePlacementStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourceSelector) DeepCopyInto(out *ClusterResourceSelector) {
	*out = *in
	if in.LabelSelector != nil {
		in, out := &in.LabelSelector, &out.LabelSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourceSelector.
func (in *ClusterResourceSelector) DeepCopy() *ClusterResourceSelector {
	if in == nil {
		return nil
	}
	out := new(ClusterResourceSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourceSnapshot) DeepCopyInto(out *ClusterResourceSnapshot) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourceSnapshot.
func (in *ClusterResourceSnapshot) DeepCopy() *ClusterResourceSnapshot {
	if in == nil {
		return nil
	}
	out := new(ClusterResourceSnapshot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterResourceSnapshot) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterResourceSnapshotList) DeepCopyInto(out *ClusterResourceSnapshotList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterResourceSnapshot, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterResourceSnapshotList.
func (in *ClusterResourceSnapshotList) DeepCopy() *ClusterResourceSnapshotList {
	if in == nil {
		return nil
	}
	out := new(ClusterResourceSnapshotList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterResourceSnapshotList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSchedulingPolicySnapshot) DeepCopyInto(out *ClusterSchedulingPolicySnapshot) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSchedulingPolicySnapshot.
func (in *ClusterSchedulingPolicySnapshot) DeepCopy() *ClusterSchedulingPolicySnapshot {
	if in == nil {
		return nil
	}
	out := new(ClusterSchedulingPolicySnapshot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterSchedulingPolicySnapshot) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSchedulingPolicySnapshotList) DeepCopyInto(out *ClusterSchedulingPolicySnapshotList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterSchedulingPolicySnapshot, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSchedulingPolicySnapshotList.
func (in *ClusterSchedulingPolicySnapshotList) DeepCopy() *ClusterSchedulingPolicySnapshotList {
	if in == nil {
		return nil
	}
	out := new(ClusterSchedulingPolicySnapshotList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterSchedulingPolicySnapshotList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterScore) DeepCopyInto(out *ClusterScore) {
	*out = *in
	if in.AffinityScore != nil {
		in, out := &in.AffinityScore, &out.AffinityScore
		*out = new(int32)
		**out = **in
	}
	if in.TopologySpreadScore != nil {
		in, out := &in.TopologySpreadScore, &out.TopologySpreadScore
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterScore.
func (in *ClusterScore) DeepCopy() *ClusterScore {
	if in == nil {
		return nil
	}
	out := new(ClusterScore)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSelector) DeepCopyInto(out *ClusterSelector) {
	*out = *in
	if in.ClusterSelectorTerms != nil {
		in, out := &in.ClusterSelectorTerms, &out.ClusterSelectorTerms
		*out = make([]ClusterSelectorTerm, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSelector.
func (in *ClusterSelector) DeepCopy() *ClusterSelector {
	if in == nil {
		return nil
	}
	out := new(ClusterSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSelectorTerm) DeepCopyInto(out *ClusterSelectorTerm) {
	*out = *in
	in.LabelSelector.DeepCopyInto(&out.LabelSelector)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSelectorTerm.
func (in *ClusterSelectorTerm) DeepCopy() *ClusterSelectorTerm {
	if in == nil {
		return nil
	}
	out := new(ClusterSelectorTerm)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EnvelopeIdentifier) DeepCopyInto(out *EnvelopeIdentifier) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EnvelopeIdentifier.
func (in *EnvelopeIdentifier) DeepCopy() *EnvelopeIdentifier {
	if in == nil {
		return nil
	}
	out := new(EnvelopeIdentifier)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FailedResourcePlacement) DeepCopyInto(out *FailedResourcePlacement) {
	*out = *in
	in.ResourceIdentifier.DeepCopyInto(&out.ResourceIdentifier)
	in.Condition.DeepCopyInto(&out.Condition)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FailedResourcePlacement.
func (in *FailedResourcePlacement) DeepCopy() *FailedResourcePlacement {
	if in == nil {
		return nil
	}
	out := new(FailedResourcePlacement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Manifest) DeepCopyInto(out *Manifest) {
	*out = *in
	in.RawExtension.DeepCopyInto(&out.RawExtension)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Manifest.
func (in *Manifest) DeepCopy() *Manifest {
	if in == nil {
		return nil
	}
	out := new(Manifest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManifestCondition) DeepCopyInto(out *ManifestCondition) {
	*out = *in
	out.Identifier = in.Identifier
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManifestCondition.
func (in *ManifestCondition) DeepCopy() *ManifestCondition {
	if in == nil {
		return nil
	}
	out := new(ManifestCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedName) DeepCopyInto(out *NamespacedName) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespacedName.
func (in *NamespacedName) DeepCopy() *NamespacedName {
	if in == nil {
		return nil
	}
	out := new(NamespacedName)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PlacementPolicy) DeepCopyInto(out *PlacementPolicy) {
	*out = *in
	if in.ClusterNames != nil {
		in, out := &in.ClusterNames, &out.ClusterNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NumberOfClusters != nil {
		in, out := &in.NumberOfClusters, &out.NumberOfClusters
		*out = new(int32)
		**out = **in
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.TopologySpreadConstraints != nil {
		in, out := &in.TopologySpreadConstraints, &out.TopologySpreadConstraints
		*out = make([]TopologySpreadConstraint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]Toleration, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PlacementPolicy.
func (in *PlacementPolicy) DeepCopy() *PlacementPolicy {
	if in == nil {
		return nil
	}
	out := new(PlacementPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PreferredClusterSelector) DeepCopyInto(out *PreferredClusterSelector) {
	*out = *in
	in.Preference.DeepCopyInto(&out.Preference)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PreferredClusterSelector.
func (in *PreferredClusterSelector) DeepCopy() *PreferredClusterSelector {
	if in == nil {
		return nil
	}
	out := new(PreferredClusterSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceBindingSpec) DeepCopyInto(out *ResourceBindingSpec) {
	*out = *in
	if in.ResourceOverrideSnapshots != nil {
		in, out := &in.ResourceOverrideSnapshots, &out.ResourceOverrideSnapshots
		*out = make([]NamespacedName, len(*in))
		copy(*out, *in)
	}
	if in.ClusterResourceOverrideSnapshots != nil {
		in, out := &in.ClusterResourceOverrideSnapshots, &out.ClusterResourceOverrideSnapshots
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.ClusterDecision.DeepCopyInto(&out.ClusterDecision)
	if in.ApplyStrategy != nil {
		in, out := &in.ApplyStrategy, &out.ApplyStrategy
		*out = new(ApplyStrategy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceBindingSpec.
func (in *ResourceBindingSpec) DeepCopy() *ResourceBindingSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceBindingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceBindingStatus) DeepCopyInto(out *ResourceBindingStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceBindingStatus.
func (in *ResourceBindingStatus) DeepCopy() *ResourceBindingStatus {
	if in == nil {
		return nil
	}
	out := new(ResourceBindingStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceContent) DeepCopyInto(out *ResourceContent) {
	*out = *in
	in.RawExtension.DeepCopyInto(&out.RawExtension)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceContent.
func (in *ResourceContent) DeepCopy() *ResourceContent {
	if in == nil {
		return nil
	}
	out := new(ResourceContent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceIdentifier) DeepCopyInto(out *ResourceIdentifier) {
	*out = *in
	if in.Envelope != nil {
		in, out := &in.Envelope, &out.Envelope
		*out = new(EnvelopeIdentifier)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceIdentifier.
func (in *ResourceIdentifier) DeepCopy() *ResourceIdentifier {
	if in == nil {
		return nil
	}
	out := new(ResourceIdentifier)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourcePlacementStatus) DeepCopyInto(out *ResourcePlacementStatus) {
	*out = *in
	if in.ApplicableResourceOverrides != nil {
		in, out := &in.ApplicableResourceOverrides, &out.ApplicableResourceOverrides
		*out = make([]NamespacedName, len(*in))
		copy(*out, *in)
	}
	if in.ApplicableClusterResourceOverrides != nil {
		in, out := &in.ApplicableClusterResourceOverrides, &out.ApplicableClusterResourceOverrides
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.FailedPlacements != nil {
		in, out := &in.FailedPlacements, &out.FailedPlacements
		*out = make([]FailedResourcePlacement, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourcePlacementStatus.
func (in *ResourcePlacementStatus) DeepCopy() *ResourcePlacementStatus {
	if in == nil {
		return nil
	}
	out := new(ResourcePlacementStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSnapshotSpec) DeepCopyInto(out *ResourceSnapshotSpec) {
	*out = *in
	if in.SelectedResources != nil {
		in, out := &in.SelectedResources, &out.SelectedResources
		*out = make([]ResourceContent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSnapshotSpec.
func (in *ResourceSnapshotSpec) DeepCopy() *ResourceSnapshotSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceSnapshotSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSnapshotStatus) DeepCopyInto(out *ResourceSnapshotStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSnapshotStatus.
func (in *ResourceSnapshotStatus) DeepCopy() *ResourceSnapshotStatus {
	if in == nil {
		return nil
	}
	out := new(ResourceSnapshotStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RollingUpdateConfig) DeepCopyInto(out *RollingUpdateConfig) {
	*out = *in
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(intstr.IntOrString)
		**out = **in
	}
	if in.MaxSurge != nil {
		in, out := &in.MaxSurge, &out.MaxSurge
		*out = new(intstr.IntOrString)
		**out = **in
	}
	if in.UnavailablePeriodSeconds != nil {
		in, out := &in.UnavailablePeriodSeconds, &out.UnavailablePeriodSeconds
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RollingUpdateConfig.
func (in *RollingUpdateConfig) DeepCopy() *RollingUpdateConfig {
	if in == nil {
		return nil
	}
	out := new(RollingUpdateConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RolloutStrategy) DeepCopyInto(out *RolloutStrategy) {
	*out = *in
	if in.RollingUpdate != nil {
		in, out := &in.RollingUpdate, &out.RollingUpdate
		*out = new(RollingUpdateConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.ApplyStrategy != nil {
		in, out := &in.ApplyStrategy, &out.ApplyStrategy
		*out = new(ApplyStrategy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RolloutStrategy.
func (in *RolloutStrategy) DeepCopy() *RolloutStrategy {
	if in == nil {
		return nil
	}
	out := new(RolloutStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchedulingPolicySnapshotSpec) DeepCopyInto(out *SchedulingPolicySnapshotSpec) {
	*out = *in
	if in.Policy != nil {
		in, out := &in.Policy, &out.Policy
		*out = new(PlacementPolicy)
		(*in).DeepCopyInto(*out)
	}
	if in.PolicyHash != nil {
		in, out := &in.PolicyHash, &out.PolicyHash
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchedulingPolicySnapshotSpec.
func (in *SchedulingPolicySnapshotSpec) DeepCopy() *SchedulingPolicySnapshotSpec {
	if in == nil {
		return nil
	}
	out := new(SchedulingPolicySnapshotSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchedulingPolicySnapshotStatus) DeepCopyInto(out *SchedulingPolicySnapshotStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ClusterDecisions != nil {
		in, out := &in.ClusterDecisions, &out.ClusterDecisions
		*out = make([]ClusterDecision, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchedulingPolicySnapshotStatus.
func (in *SchedulingPolicySnapshotStatus) DeepCopy() *SchedulingPolicySnapshotStatus {
	if in == nil {
		return nil
	}
	out := new(SchedulingPolicySnapshotStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServerSideApplyConfig) DeepCopyInto(out *ServerSideApplyConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServerSideApplyConfig.
func (in *ServerSideApplyConfig) DeepCopy() *ServerSideApplyConfig {
	if in == nil {
		return nil
	}
	out := new(ServerSideApplyConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Toleration) DeepCopyInto(out *Toleration) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Toleration.
func (in *Toleration) DeepCopy() *Toleration {
	if in == nil {
		return nil
	}
	out := new(Toleration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopologySpreadConstraint) DeepCopyInto(out *TopologySpreadConstraint) {
	*out = *in
	if in.MaxSkew != nil {
		in, out := &in.MaxSkew, &out.MaxSkew
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopologySpreadConstraint.
func (in *TopologySpreadConstraint) DeepCopy() *TopologySpreadConstraint {
	if in == nil {
		return nil
	}
	out := new(TopologySpreadConstraint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Work) DeepCopyInto(out *Work) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Work.
func (in *Work) DeepCopy() *Work {
	if in == nil {
		return nil
	}
	out := new(Work)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Work) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkList) DeepCopyInto(out *WorkList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Work, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkList.
func (in *WorkList) DeepCopy() *WorkList {
	if in == nil {
		return nil
	}
	out := new(WorkList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WorkList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkResourceIdentifier) DeepCopyInto(out *WorkResourceIdentifier) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkResourceIdentifier.
func (in *WorkResourceIdentifier) DeepCopy() *WorkResourceIdentifier {
	if in == nil {
		return nil
	}
	out := new(WorkResourceIdentifier)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkSpec) DeepCopyInto(out *WorkSpec) {
	*out = *in
	in.Workload.DeepCopyInto(&out.Workload)
	if in.ApplyStrategy != nil {
		in, out := &in.ApplyStrategy, &out.ApplyStrategy
		*out = new(ApplyStrategy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkSpec.
func (in *WorkSpec) DeepCopy() *WorkSpec {
	if in == nil {
		return nil
	}
	out := new(WorkSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkStatus) DeepCopyInto(out *WorkStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ManifestConditions != nil {
		in, out := &in.ManifestConditions, &out.ManifestConditions
		*out = make([]ManifestCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkStatus.
func (in *WorkStatus) DeepCopy() *WorkStatus {
	if in == nil {
		return nil
	}
	out := new(WorkStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WorkloadTemplate) DeepCopyInto(out *WorkloadTemplate) {
	*out = *in
	if in.Manifests != nil {
		in, out := &in.Manifests, &out.Manifests
		*out = make([]Manifest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadTemplate.
func (in *WorkloadTemplate) DeepCopy() *WorkloadTemplate {
	if in == nil {
		return nil
	}
	out := new(WorkloadTemplate)
	in.DeepCopyInto(out)
	return out
}
