/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package authtokenissuer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterinventoryv1alpha1 "go.goms.io/fleet/apis/clusterinventory/v1alpha1"
	conditionutil "go.goms.io/fleet/pkg/utils/condition"
)

const (
	// internalAuthTokenRequestCleanupFinalizer is the finalizer added to the
	// InternalAuthTokenRequest object if the requested RBAC setup has been processed
	// and/or a service account token has been issued.
	internalAuthTokenRequestCleanupFinalizer = "kubernetes-fleet.io/internal-auth-token-request-cleanup"

	// svcAccountTokenSecretNameFormat is the name format of the secret that stores
	// the issued service account token.
	//
	// The format is {SVC-ACCOUNT-NAME}-{INTERNAL-AUTH-TOKEN-REQUEST-NAME}-token.
	//
	// TO-DO (chenyu1): this format guarantees uniqueness (1:1 mapping), but no length check is performed.
	svcAccountTokenSecretNameFormat = "%s-%s-token"

	// roleBindingNameFormat is the name format of the role binding that associates a
	// specified role with the target service account.
	//
	// The format is {SVC-ACCOUNT-NAME}-{ROLE-NAME}-role-binding.
	//
	// TO-DO (chenyu1): this format guarantees uniqueness (1:1 mapping), but no length check is performed.
	roleBindingNameFormat = "%s-%s-role-binding"

	// clusterRoleBindingNameFormat is the name format of the cluster role binding that
	// associates a specified cluster role with the target service account.
	//
	// The format is {SVC-ACCOUNT-NAME}-{CLUSTER-ROLE-NAME}-cluster-role-binding.
	//
	// TO-DO (chenyu1): this format guarantees uniqueness (1:1 mapping), but no length check is performed.
	clusterRoleBindingNameFormat = "%s-%s-cluster-role-binding"

	// ownerInternalAuthTokenRequestNameAnnotation is the label key that stores
	// the name of the InternalAuthTokenRequest object that is associated with a RBAC setup.
	ownerInternalAuthTokenRequestNameLabel = "kubernetes-fleet.io/internal-auth-token-request.name"
)

const (
	defaultRetryRequeueAfterPeriod = 10 * time.Second
	defaultWatchRequeueAfterPeriod = 5 * time.Minute
)

// Reconciler reconciles an InternalAuthTokenRequest object. Essentially, this controller
// attempts to fulfill an AuthTokenRequest by performing RBAC setup as requested and
// preparing a kubeconfig that grants access to the host member cluster.
type Reconciler struct {
	HubClient                  client.Client
	MemberClient               client.Client
	MemberClusterName          string
	MemberCLusterAPIServerHost string
	MemberClusterReservedNS    string
	ServiceAccountDefaultNS    string
}

// Reconcile reconciles an InternalAuthTokenRequest object.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	iatrRef := klog.KRef(req.Namespace, req.Name)
	startTime := time.Now()
	klog.V(2).InfoS("Reconciliation starts (auth token issuer controller)", "InternalAuthTokenRequest", iatrRef)
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		klog.V(2).InfoS("Reconciliation ends (auth token issuer controller)", "InternalAuthTokenRequest", iatrRef, "Latency", latency)
	}()

	// Retrieve the InternalAuthTokenRequest object.
	iatr := &clusterinventoryv1alpha1.InternalAuthTokenRequest{}
	if err := r.HubClient.Get(ctx, req.NamespacedName, iatr); err != nil {
		if errors.IsNotFound(err) {
			// InternalAuthTokenRequest object is not found.

			// TO-DO (chenyu1): address a corner case and clean up the corresponding RBAC setup
			// (if any left) in the member cluster.
			return ctrl.Result{}, nil
		}
		klog.ErrorS(err, "Failed to get internal auth token request", "InternalAuthTokenRequest", iatrRef)
	}

	// Perform some sanity check first.
	svcAccountName := iatr.Spec.ServiceAccountName
	svcAccountNS := iatr.Spec.ServiceAccountNamespace
	if len(svcAccountNS) == 0 {
		svcAccountNS = r.ServiceAccountDefaultNS
		iatr.Spec.ServiceAccountNamespace = svcAccountNS
	}
	// Perform a sanity check just in case.
	if len(svcAccountName) == 0 {
		klog.ErrorS(fmt.Errorf("service account name is empty"), "Service account name is empty", "InternalAuthTokenRequest", iatrRef)
		return ctrl.Result{}, nil
	}

	// Check if the InternalAuthTokenRequest object has been marked for deletion.
	if !iatr.DeletionTimestamp.IsZero() {
		if err := r.cleanupRBACSetup(ctx, iatr); err != nil {
			klog.ErrorS(err, "Failed to clean up RBAC setup", "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}

		// Remove the cleanup finalizer from the InternalAuthTokenRequest object.
		controllerutil.RemoveFinalizer(iatr, internalAuthTokenRequestCleanupFinalizer)
		if err := r.HubClient.Update(ctx, iatr); err != nil {
			klog.ErrorS(err, "Failed to remove cleanup finalizer", "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Add the cleanup finalizer to the InternalAuthTokenRequest object (if none exists).
	if !controllerutil.ContainsFinalizer(iatr, internalAuthTokenRequestCleanupFinalizer) {
		controllerutil.AddFinalizer(iatr, internalAuthTokenRequestCleanupFinalizer)
		if err := r.HubClient.Update(ctx, iatr); err != nil {
			klog.ErrorS(err, "Failed to add cleanup finalizer", "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
	}

	// Verify that if the namespace of the linked service account exists; will create it if the
	// namespace is not found.
	ns := &corev1.Namespace{}
	nsRef := klog.KRef("", svcAccountNS)
	err := r.MemberClient.Get(ctx, types.NamespacedName{Name: svcAccountNS}, ns)
	switch {
	case err == nil && !ns.DeletionTimestamp.IsZero():
		// The namespace already exists but has been marked for deletion.
		klog.V(2).InfoS("Namespace is marked for deletion", "Namespace", nsRef, "InternalAuthTokenRequest", iatrRef)
		// Retry later to see if the namespace has been deleted successfully.
		if err := r.setInternalAuthTokenRequestStatusSvcAccountFoundOrCreatedCondition(ctx, iatr, metav1.ConditionFalse, "ServiceAccountNamespaceMarkedForDeletion", "Service account's namespace is marked for deletion"); err != nil {
			klog.ErrorS(err, "Failed to set service account found or created condition (namespace marked for deletion) in status", "Namespace", nsRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: defaultRetryRequeueAfterPeriod}, nil
	case err == nil:
		// The namespace already exists.
		klog.V(2).InfoS("Service account's namespace already exists", "Namespace", nsRef, "InternalAuthTokenRequest", iatrRef)
	case errors.IsNotFound(err):
		// The namespace does not exist; one should be created.
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: svcAccountNS,
				Labels: map[string]string{
					clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
					ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
				},
			},
		}
		if err := r.MemberClient.Create(ctx, ns); err != nil {
			klog.ErrorS(err, "Failed to create service account namespace", "Namespace", nsRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
	}

	// Verify that if the linked service account exists; will create it if the service
	// acount is not found.
	svcAccount := &corev1.ServiceAccount{}
	svcAccountRef := klog.KRef(svcAccountNS, svcAccountName)
	err = r.MemberClient.Get(ctx, types.NamespacedName{Name: svcAccountName, Namespace: svcAccountNS}, svcAccount)
	switch {
	case err == nil && !svcAccount.DeletionTimestamp.IsZero():
		// The service account already exists but has been marked for deletion.
		klog.V(2).InfoS("Service account is marked for deletion", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		// Retry later to see if the service account has been deleted successfully.
		if err := r.setInternalAuthTokenRequestStatusSvcAccountFoundOrCreatedCondition(ctx, iatr, metav1.ConditionFalse, "ServiceAccountMarkedForDeletion", "Service account is marked for deletion"); err != nil {
			klog.ErrorS(err, "Failed to set service account found or created condition (service account marked for deletion) in status", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: defaultRetryRequeueAfterPeriod}, nil
	case err == nil:
		// The service account already exists.
		klog.V(2).InfoS("Service account already exists", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		if err := r.setInternalAuthTokenRequestStatusSvcAccountFoundOrCreatedCondition(ctx, iatr, metav1.ConditionTrue, "ServiceAccountFound", "Service account already exists"); err != nil {
			klog.ErrorS(err, "Failed to set service account found or created condition (already exists) in status", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		// Continue to retrieve the service account token.
	case errors.IsNotFound(err):
		// The service account does not exist; one should be created.
		klog.V(2).InfoS("Service account does not exist; will create a new one", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		svcAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcAccountName,
				Namespace: svcAccountNS,
				Labels: map[string]string{
					clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
					ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
				},
			},
		}
		if err := r.MemberClient.Create(ctx, svcAccount); err != nil {
			klog.ErrorS(err, "Failed to create service account", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}

		klog.V(2).InfoS("Service account created", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		if err := r.setInternalAuthTokenRequestStatusSvcAccountFoundOrCreatedCondition(ctx, iatr, metav1.ConditionTrue, "ServiceAccountCreated", "Service account is created"); err != nil {
			klog.ErrorS(err, "Failed to set service account found or created condition (created) in status", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		// Continue to retrieve the service account token.
	default:
		// An unexpected error occurred.
		klog.ErrorS(err, "Failed to get service account", "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		return ctrl.Result{}, err
	}

	// Verify if the roles exist; will create them instead if the roles are not found.
	//
	// Run the setup in parallel.
	var roleSetupWg sync.WaitGroup
	var roleSetupCounter atomic.Int32
	var roleBindingSetupCounter atomic.Int32
	totalRoleCount := len(iatr.Spec.Roles)
	for i := 0; i < totalRoleCount; i++ {
		roleSetupWg.Add(1)
		go func(i int) {
			defer roleSetupWg.Done()

			desiredRole := iatr.Spec.Roles[i]
			desiredRoleNS := desiredRole.Namespace
			desiredRoleName := desiredRole.Name
			// Check if the namespace exists; will create it instead if the namespace is not found.
			ns := &corev1.Namespace{}
			nsTypedName := types.NamespacedName{Name: desiredRoleNS}
			err := r.MemberClient.Get(ctx, nsTypedName, ns)
			switch {
			case err == nil && !ns.DeletionTimestamp.IsZero():
				// The namespace already exists but has been marked for deletion.
				klog.V(2).InfoS("Namespace for a role is marked for deletion", "Role", klog.KRef(desiredRoleNS, desiredRoleName), "Namespace", klog.KObj(ns), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the namespace has been deleted successfully.
				return
			case err == nil:
				// The namespace already exists; do nothing.
			case errors.IsNotFound(err):
				// The namespace does not exist; one should be created.
				ns = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: desiredRoleNS,
						Labels: map[string]string{
							clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
							ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
						},
					},
				}
				if err := r.MemberClient.Create(ctx, ns); err != nil {
					klog.ErrorS(err, "Failed to create namespace for a role", "Namespace", klog.KObj(ns), "Role", klog.KRef(desiredRoleNS, desiredRoleName), "InternalAuthTokenRequest", iatrRef)
					return
				}
			default:
				// An unexpected error occurred.
				klog.ErrorS(err, "Failed to get namespace for a role", "Namespace", klog.KRef(desiredRoleNS, desiredRoleName), "Role", klog.KRef(desiredRoleNS, desiredRoleName), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the situation will self-resolve.
				return
			}

			role := &rbacv1.Role{}
			roleTypedName := types.NamespacedName{Namespace: desiredRole.Namespace, Name: desiredRole.Name}
			err = r.MemberClient.Get(ctx, roleTypedName, role)
			switch {
			case err == nil && !ns.DeletionTimestamp.IsZero():
				// The role already exists but has been marked for deletion.
				klog.V(2).InfoS("Role is marked for deletion", "Role", klog.KObj(role), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the role has been deleted successfully.
				return
			case err == nil:
				// The role already exists; increment the counter.
				roleSetupCounter.Add(1)
			case errors.IsNotFound(err):
				// The role does not exist; one should be created.
				role = &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: desiredRoleNS,
						Name:      desiredRoleName,
						Labels: map[string]string{
							clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
							ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
						},
					},
					Rules: desiredRole.Rules,
				}
				if err := r.MemberClient.Create(ctx, role); err != nil {
					klog.ErrorS(err, "Failed to create role", "Role", klog.KObj(role), "InternalAuthTokenRequest", iatrRef)
					return
				}
				// The role has been created; increment the counter.
				roleSetupCounter.Add(1)
			default:
				// An unexpected error occurred.
				klog.ErrorS(err, "Failed to get role", "Role", klog.KRef(desiredRoleNS, desiredRoleName), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the situation will self-resolve.
				return
			}

			roleBinding := &rbacv1.RoleBinding{}
			roleBindingNameFormat := fmt.Sprintf(roleBindingNameFormat, svcAccountName, desiredRoleName)
			roleBindingTypedName := types.NamespacedName{Namespace: desiredRoleNS, Name: roleBindingNameFormat}
			err = r.MemberClient.Get(ctx, roleBindingTypedName, roleBinding)
			switch {
			case err == nil && !roleBinding.DeletionTimestamp.IsZero():
				// The role binding already exists but has been marked for deletion.
				klog.V(2).InfoS("Role binding is marked for deletion", "RoleBinding", klog.KObj(roleBinding), "Role", klog.KObj(role), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the role binding has been deleted successfully.
				return
			case err == nil:
				// The role binding already exists; increment the counter.
				roleBindingSetupCounter.Add(1)
			case errors.IsNotFound(err):
				// The role binding does not exist; one should be created.
				roleBinding = &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: desiredRoleNS,
						Name:      roleBindingNameFormat,
						Labels: map[string]string{
							clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
							ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
						},
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      svcAccountName,
							Namespace: svcAccountNS,
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind:     "Role",
						Name:     desiredRoleName,
						APIGroup: rbacv1.GroupName,
					},
				}
				if err := r.MemberClient.Create(ctx, roleBinding); err != nil {
					klog.ErrorS(err, "Failed to create role binding", "RoleBinding", klog.KObj(roleBinding), "Role", klog.KObj(role), "InternalAuthTokenRequest", iatrRef)
					return
				}
				// The role binding has been created; increment the counter.
				roleBindingSetupCounter.Add(1)
			default:
				// An unexpected error occurred.
				klog.ErrorS(err, "Failed to get role binding", "RoleBinding", klog.KRef(desiredRoleNS, roleBindingNameFormat), "Role", klog.KRef(desiredRoleNS, desiredRoleName), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the situation will self-resolve.
				return
			}
		}(i)
	}
	roleSetupWg.Wait()
	successRoleCount := roleSetupCounter.Load()
	successRoleBindingCount := roleBindingSetupCounter.Load()
	if successRoleCount < int32(totalRoleCount) || successRoleBindingCount < int32(totalRoleCount) {
		// Some roles cannot be set up successfully; retry later.
		klog.V(2).InfoS("Some roles or role bindings cannot be set up successfully",
			"InternalAuthTokenRequest", iatrRef,
			"SuccessRoleCount", successRoleCount, "TotalRoleCount", totalRoleCount,
			"SuccessRoleBindingCount", successRoleBindingCount, "TotalRoleBindingCount", totalRoleCount)
		condMsg := fmt.Sprintf("Some roles or role bindings cannot be set up successfully (roles: %d/%d, role bindings: %d/%d)", successRoleCount, totalRoleCount, successRoleBindingCount, totalRoleCount)
		if err := r.setInternalAuthTokenRequestStatusRoleSetupConditon(ctx, iatr, metav1.ConditionFalse, "RoleSetupFailed", condMsg); err != nil {
			klog.ErrorS(err, "Failed to set role setup condition (failed) in status", "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, fmt.Errorf("some roles cannot be set up successfully")
	}

	// Report that all roles have been set up successfully as a status condition.
	if err := r.setInternalAuthTokenRequestStatusRoleSetupConditon(ctx, iatr, metav1.ConditionTrue, "RoleSetupSucceeded", "All roles have been set up successfully"); err != nil {
		klog.ErrorS(err, "Failed to set role setup condition (succeeded) in status", "InternalAuthTokenRequest", iatrRef)
		return ctrl.Result{}, err
	}

	// Verify if the cluster roles exist; will create them instead if the cluster roles are not found.
	//
	// Run the setup in parallel.
	var clusterRoleSetupWg sync.WaitGroup
	var clusterRoleSetupCounter atomic.Int32
	var clusterRoleBindingSetupCounter atomic.Int32
	totalClusterRoleCount := len(iatr.Spec.ClusterRoles)
	for i := 0; i < totalClusterRoleCount; i++ {
		clusterRoleSetupWg.Add(1)
		go func(i int) {
			defer clusterRoleSetupWg.Done()

			desiredClusterRole := iatr.Spec.ClusterRoles[i]
			desiredClusterRoleName := desiredClusterRole.Name
			clusterRole := &rbacv1.ClusterRole{}
			clusterRoleTypedName := types.NamespacedName{Name: desiredClusterRoleName}
			err := r.MemberClient.Get(ctx, clusterRoleTypedName, clusterRole)
			switch {
			case err == nil && !clusterRole.DeletionTimestamp.IsZero():
				// The cluster role already exists but has been marked for deletion.
				klog.V(2).InfoS("Cluster role is marked for deletion", "ClusterRole", klog.KObj(clusterRole), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the cluster role has been deleted successfully.
				return
			case err == nil:
				// The cluster role already exists; increment the counter.
				clusterRoleSetupCounter.Add(1)
			case errors.IsNotFound(err):
				// The cluster role does not exist; one should be created.
				clusterRole := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: desiredClusterRoleName,
						Labels: map[string]string{
							clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
							ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
						},
					},
					Rules: desiredClusterRole.Rules,
				}
				if err := r.MemberClient.Create(ctx, clusterRole); err != nil {
					klog.ErrorS(err, "Failed to create cluster role", "ClusterRole", klog.KObj(clusterRole), "InternalAuthTokenRequest", iatrRef)
					return
				}
				// The cluster role has been created; increment the counter.
				clusterRoleSetupCounter.Add(1)
			default:
				// An unexpected error occurred.
				klog.ErrorS(err, "Failed to get cluster role", "ClusterRole", klog.KRef("", desiredClusterRoleName), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the situation will self-resolve.
				return
			}

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			clusterRoleBindingNameFormat := fmt.Sprintf(clusterRoleBindingNameFormat, svcAccountName, desiredClusterRoleName)
			clusterRoleBindingTypedName := types.NamespacedName{Name: clusterRoleBindingNameFormat}
			err = r.MemberClient.Get(ctx, clusterRoleBindingTypedName, clusterRoleBinding)
			switch {
			case err == nil && !clusterRoleBinding.DeletionTimestamp.IsZero():
				// The cluster role binding already exists but has been marked for deletion.
				klog.V(2).InfoS("Cluster role binding is marked for deletion", "ClusterRoleBinding", klog.KObj(clusterRoleBinding), "ClusterRole", klog.KObj(clusterRole), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the cluster role binding has been deleted successfully.
				return
			case err == nil:
				// The cluster role binding already exists; increment the counter.
				clusterRoleBindingSetupCounter.Add(1)
			case errors.IsNotFound(err):
				// The cluster role binding does not exist; one should be created.
				clusterRoleBinding = &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterRoleBindingNameFormat,
						Labels: map[string]string{
							clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
							ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
						},
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      svcAccountName,
							Namespace: svcAccountNS,
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind:     "ClusterRole",
						Name:     desiredClusterRoleName,
						APIGroup: rbacv1.GroupName,
					},
				}
				if err := r.MemberClient.Create(ctx, clusterRoleBinding); err != nil {
					klog.ErrorS(err, "Failed to create cluster role binding", "ClusterRoleBinding", klog.KObj(clusterRoleBinding), "ClusterRole", klog.KObj(clusterRole), "InternalAuthTokenRequest", iatrRef)
					return
				}
				// The cluster role binding has been created; increment the counter.
				clusterRoleBindingSetupCounter.Add(1)
			default:
				// An unexpected error occurred.
				klog.ErrorS(err, "Failed to get cluster role binding", "ClusterRoleBinding", klog.KRef("", clusterRoleBindingNameFormat), "ClusterRole", klog.KRef("", desiredClusterRoleName), "InternalAuthTokenRequest", iatrRef)
				// Retry later to see if the situation will self-resolve.
				return
			}
		}(i)
	}
	clusterRoleSetupWg.Wait()
	successClusterRoleCount := clusterRoleSetupCounter.Load()
	successClusterRoleBindingCount := clusterRoleBindingSetupCounter.Load()
	if successClusterRoleCount < int32(totalClusterRoleCount) || successClusterRoleBindingCount < int32(totalClusterRoleCount) {
		// Some cluster roles cannot be set up successfully; retry later.
		klog.V(2).InfoS("Some cluster roles or cluster role bindings cannot be set up successfully",
			"InternalAuthTokenRequest", iatrRef,
			"SuccessClusterRoleCount", successClusterRoleCount, "TotalClusterRoleCount", totalClusterRoleCount,
			"SuccessClusterRoleBindingCount", successClusterRoleBindingCount, "TotalClusterRoleBindingCount", totalClusterRoleCount)
		condMsg := fmt.Sprintf("Some cluster roles or cluster role bindings cannot be set up successfully (cluster roles: %d/%d, cluster role bindings: %d/%d)",
			successClusterRoleCount, totalClusterRoleCount,
			successClusterRoleBindingCount, totalClusterRoleCount)
		if err := r.setInternalAuthTokenRequestStatusClusterRoleSetupConditon(ctx, iatr, metav1.ConditionFalse, "ClusterRoleSetupFailed", condMsg); err != nil {
			klog.ErrorS(err, "Failed to set cluster role setup condition (failed) in status", "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, fmt.Errorf("some cluster roles cannot be set up successfully")
	}

	// Report that all cluster roles have been set up successfully as a status condition.
	if err := r.setInternalAuthTokenRequestStatusClusterRoleSetupConditon(ctx, iatr, metav1.ConditionTrue, "ClusterRoleSetupSucceeded", "All cluster roles have been set up successfully"); err != nil {
		klog.ErrorS(err, "Failed to set cluster role setup condition (succeeded) in status", "InternalAuthTokenRequest", iatrRef)
		return ctrl.Result{}, err
	}

	// Retrieve the service account token.
	//
	// The token is always stored in the same namespace as the service account.
	secret := &corev1.Secret{}
	secretTypedName := types.NamespacedName{
		Name:      fmt.Sprintf(svcAccountTokenSecretNameFormat, svcAccountName, iatr.Name),
		Namespace: svcAccountNS,
	}
	secretRef := klog.KRef(svcAccountNS, secretTypedName.Name)
	err = r.MemberClient.Get(ctx, secretTypedName, secret)
	switch {
	case err == nil && !secret.DeletionTimestamp.IsZero():
		// The secret already exists but has been marked for deletion.
		klog.V(2).InfoS("Token secret is marked for deletion", "Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		if err := r.setInternalAuthTokenRequestStatusSvcAccountTokenRetrievedCondition(ctx, iatr, metav1.ConditionFalse, "TokenSecretMarkedForDeletion", "Token secret is marked for deletion"); err != nil {
			klog.ErrorS(err, "Failed to set service account token retrieved condition (marked for deletion) in status", "Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}

		// Retry later to see if the secret has been deleted successfully.
		return ctrl.Result{RequeueAfter: defaultRetryRequeueAfterPeriod}, nil
	case err == nil && shouldRecreateTokenSecret(secret, svcAccount):
		// The secret exists but is not valid.
		//
		// It could be that the secret is not of the service account token type or is not linked
		// with the target service account.
		//
		// Normally this should not occur; the controller will attempt to re-create the token
		// secret.
		klog.V(2).InfoS("Token secret is not valid; will re-create",
			"Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		if err := r.MemberClient.Delete(ctx, secret); err != nil {
			klog.ErrorS(err, "Failed to delete token secret",
				"Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}

		// Retry later to see if the secret has been deleted successfully.
		return ctrl.Result{RequeueAfter: defaultRetryRequeueAfterPeriod}, nil
	case err == nil && !isTokenSecretReadyToUse(secret, svcAccount):
		// The secret exists but is not ready to use.
		//
		// This might occur if the controller catches an intermediate state; it will retry later.
		klog.V(2).InfoS("Token secret is not ready to use", "Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
		if err := r.setInternalAuthTokenRequestStatusSvcAccountTokenRetrievedCondition(ctx, iatr, metav1.ConditionFalse, "TokenSecretNotReady", "Token secret is not ready to use"); err != nil {
			klog.ErrorS(err, "Failed to set service account token retrieved condition (not ready to use) in status", "Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: defaultRetryRequeueAfterPeriod}, nil
	case err == nil:
		// The secret exists and is valid.

		// Prepare a config map to store the configuration for accessing the member cluster.
		kubeConfig, buildErr := buildKubeConfigYAML(r.MemberClusterName, r.MemberCLusterAPIServerHost, secret)
		if buildErr != nil {
			// Failed to build the kubeconfig YAML; normally this should never occur.
			klog.ErrorS(buildErr, "Failed to build kubeconfig YAML", "Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, buildErr
		}

		// Create a config map in the member cluster reserved namespace on the hub cluster.
		// This object will have the same name as the InternalAuthTokenRequest object, which
		// guarantees uniqueness.
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: r.MemberClusterReservedNS,
				Name:      iatr.Name,
			},
		}
		createOrUpdateRes, err := controllerutil.CreateOrUpdate(ctx, r.HubClient, cm, func() error {
			cm.Data = map[string]string{
				"kubeconfig": string(kubeConfig),
			}
			return nil
		})
		if err != nil {
			klog.ErrorS(err, "Failed to create or update config map",
				"ConfigMap", klog.KObj(cm),
				"InternalAuthTokenRequest", iatrRef,
				"Operation", createOrUpdateRes)
			return ctrl.Result{}, err
		}

		// Update the InternalAuthTokenRequest object to include the reference to the config map.
		tokenResRef := corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  r.MemberClusterReservedNS,
			Name:       iatr.Name,
		}
		if !equality.Semantic.DeepEqual(&iatr.Status.TokenResponse, &tokenResRef) {
			iatr.Status.TokenResponse = tokenResRef
			if err := r.HubClient.Status().Update(ctx, iatr); err != nil {
				klog.ErrorS(err, "Failed to update token response in internal auth token request status", "InternalAuthTokenRequest", iatrRef)
				return ctrl.Result{}, err
			}
		}

		if err := r.setInternalAuthTokenRequestStatusSvcAccountTokenRetrievedCondition(ctx, iatr, metav1.ConditionTrue, "TokenRetrieved", "Token is retrieved"); err != nil {
			klog.ErrorS(err, "Failed to set service account token retrieved condition (retrieved) in status", "Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
	case errors.IsNotFound(err):
		// The secret does not exist; one should be created.
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretTypedName.Name,
				Namespace: svcAccountNS,
				Labels: map[string]string{
					clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
					ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
				},
				Annotations: map[string]string{
					corev1.ServiceAccountNameKey: svcAccountName,
				},
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}
		if err := r.MemberClient.Create(ctx, secret); err != nil {
			klog.ErrorS(err, "Failed to create token secret", "Secret", secretRef, "ServiceAccount", svcAccountRef, "InternalAuthTokenRequest", iatrRef)
			return ctrl.Result{}, err
		}
		// Retry later to see if the secret has been created successfully.
		return ctrl.Result{RequeueAfter: defaultRetryRequeueAfterPeriod}, nil
	default:
		// An unexpected error occurred.
		klog.ErrorS(err, "Failed to get secret",
			"Secret", klog.KRef(svcAccountNS, secretTypedName.Name),
			"ServiceAccount", klog.KRef(svcAccountNS, svcAccountName),
			"InternalAuthTokenRequest", iatrRef)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultWatchRequeueAfterPeriod}, nil
}

// SetupWithManager sets up the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterinventoryv1alpha1.InternalAuthTokenRequest{}).
		Complete(r)
}

// cleanupRBACSetup cleans up the RBAC setup (service account and token secret, if applicable) in the member cluster.
func (r *Reconciler) cleanupRBACSetup(ctx context.Context, iatr *clusterinventoryv1alpha1.InternalAuthTokenRequest) error {
	// Delete the service account token.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: iatr.Spec.ServiceAccountNamespace,
			Name:      fmt.Sprintf(svcAccountTokenSecretNameFormat, iatr.Spec.ServiceAccountName, iatr.Name),
		},
	}
	if err := r.MemberClient.Delete(ctx, secret); err != nil && !errors.IsNotFound(err) {
		klog.ErrorS(err, "Failed to delete token secret", "Secret", klog.KObj(secret), "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}

	// Delete the service account (if it is provisioned by the agent).
	svcAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: iatr.Spec.ServiceAccountNamespace,
			Name:      iatr.Spec.ServiceAccountName,
		},
	}
	if err := r.MemberClient.Get(ctx, types.NamespacedName{Name: svcAccount.Name, Namespace: svcAccount.Namespace}, svcAccount); err != nil {
		if errors.IsNotFound(err) {
			// The service account does not exist.
			return nil
		}
		klog.ErrorS(err, "Failed to get service account", "ServiceAccount", klog.KObj(svcAccount), "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}

	isManagedByFleet := (svcAccount.Labels[clusterinventoryv1alpha1.LabelRBACSetupManagedBy] == clusterinventoryv1alpha1.ClusterManagerName)
	if isManagedByFleet {
		if err := r.MemberClient.Delete(ctx, svcAccount); err != nil && !errors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete service account", "ServiceAccount", klog.KObj(svcAccount), "InternalAuthTokenRequest", klog.KObj(iatr))
			return err
		}
	}

	// matchLabelsOpt is an option the helps list RBAC objects managed by Fleet and are owned by
	// the given InternalAuthTokenRequest object.
	matchLabelsOpt := client.MatchingLabels{
		clusterinventoryv1alpha1.LabelRBACSetupManagedBy: clusterinventoryv1alpha1.ClusterManagerName,
		ownerInternalAuthTokenRequestNameLabel:           iatr.Name,
	}

	// Delete the roles and role bindings.
	roleList := &rbacv1.RoleList{}
	if err := r.MemberClient.List(ctx, roleList, matchLabelsOpt); err != nil {
		klog.ErrorS(err, "Failed to list roles", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	for i := range roleList.Items {
		role := &roleList.Items[i]
		if err := r.MemberClient.Delete(ctx, role); err != nil && !errors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete role", "Role", klog.KObj(role), "InternalAuthTokenRequest", klog.KObj(iatr))
			return err
		}
	}

	roleBindingList := &rbacv1.RoleBindingList{}
	if err := r.MemberClient.List(ctx, roleBindingList, matchLabelsOpt); err != nil {
		klog.ErrorS(err, "Failed to list role bindings", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	for i := range roleBindingList.Items {
		roleBinding := &roleBindingList.Items[i]
		if err := r.MemberClient.Delete(ctx, roleBinding); err != nil && !errors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete role binding", "RoleBinding", klog.KObj(roleBinding), "InternalAuthTokenRequest", klog.KObj(iatr))
			return err
		}
	}

	// Delete the cluster roles and cluster role bindings.
	clusterRoleList := &rbacv1.ClusterRoleList{}
	if err := r.MemberClient.List(ctx, clusterRoleList, matchLabelsOpt); err != nil {
		klog.ErrorS(err, "Failed to list cluster roles", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	for i := range clusterRoleList.Items {
		clusterRole := &clusterRoleList.Items[i]
		if err := r.MemberClient.Delete(ctx, clusterRole); err != nil && !errors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete cluster role", "ClusterRole", klog.KObj(clusterRole), "InternalAuthTokenRequest", klog.KObj(iatr))
			return err
		}
	}

	clusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	if err := r.MemberClient.List(ctx, clusterRoleBindingList, matchLabelsOpt); err != nil {
		klog.ErrorS(err, "Failed to list cluster role bindings", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	for i := range clusterRoleBindingList.Items {
		clusterRoleBinding := &clusterRoleBindingList.Items[i]
		if err := r.MemberClient.Delete(ctx, clusterRoleBinding); err != nil && !errors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete cluster role binding", "ClusterRoleBinding", klog.KObj(clusterRoleBinding), "InternalAuthTokenRequest", klog.KObj(iatr))
			return err
		}
	}

	// Delete the namespaces created for specified roles and/or service accounts (if any).
	nsList := &corev1.NamespaceList{}
	if err := r.MemberClient.List(ctx, nsList, matchLabelsOpt); err != nil {
		klog.ErrorS(err, "Failed to list namespaces", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	for i := range nsList.Items {
		ns := &nsList.Items[i]
		if err := r.MemberClient.Delete(ctx, ns); err != nil && !errors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to delete namespace", "Namespace", klog.KObj(ns), "InternalAuthTokenRequest", klog.KObj(iatr))
			return err
		}
	}

	return nil
}

func (r *Reconciler) setInternalAuthTokenRequestStatusSvcAccountFoundOrCreatedCondition(
	ctx context.Context,
	iatr *clusterinventoryv1alpha1.InternalAuthTokenRequest,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	currentCond := meta.FindStatusCondition(iatr.Status.Conditions, clusterinventoryv1alpha1.SvcAccountFoundOrCreatedCondType)
	newCond := &metav1.Condition{
		Type:               clusterinventoryv1alpha1.SvcAccountFoundOrCreatedCondType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: iatr.Generation,
	}
	if conditionutil.EqualCondition(currentCond, newCond) {
		return nil
	}

	meta.SetStatusCondition(&iatr.Status.Conditions, *newCond)
	if err := r.HubClient.Status().Update(ctx, iatr); err != nil {
		klog.ErrorS(err, "Failed to update status", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	return nil
}

func (r *Reconciler) setInternalAuthTokenRequestStatusSvcAccountTokenRetrievedCondition(
	ctx context.Context,
	iatr *clusterinventoryv1alpha1.InternalAuthTokenRequest,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	currentCond := meta.FindStatusCondition(iatr.Status.Conditions, clusterinventoryv1alpha1.SvcAccountTokenRetrievedCondType)
	newCond := &metav1.Condition{
		Type:               clusterinventoryv1alpha1.SvcAccountTokenRetrievedCondType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: iatr.Generation,
	}
	if conditionutil.EqualCondition(currentCond, newCond) {
		return nil
	}

	meta.SetStatusCondition(&iatr.Status.Conditions, *newCond)
	if err := r.HubClient.Status().Update(ctx, iatr); err != nil {
		klog.ErrorS(err, "Failed to update status", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}

	return nil
}

func (r *Reconciler) setInternalAuthTokenRequestStatusRoleSetupConditon(
	ctx context.Context,
	iatr *clusterinventoryv1alpha1.InternalAuthTokenRequest,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	currentCond := meta.FindStatusCondition(iatr.Status.Conditions, clusterinventoryv1alpha1.RoleFoundOrCreatedCondType)
	newCond := &metav1.Condition{
		Type:               clusterinventoryv1alpha1.RoleFoundOrCreatedCondType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: iatr.Generation,
	}
	if conditionutil.EqualCondition(currentCond, newCond) {
		return nil
	}

	meta.SetStatusCondition(&iatr.Status.Conditions, *newCond)
	if err := r.HubClient.Status().Update(ctx, iatr); err != nil {
		klog.ErrorS(err, "Failed to update status", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	return nil
}

func (r *Reconciler) setInternalAuthTokenRequestStatusClusterRoleSetupConditon(
	ctx context.Context,
	iatr *clusterinventoryv1alpha1.InternalAuthTokenRequest,
	status metav1.ConditionStatus,
	reason, message string,
) error {
	currentCond := meta.FindStatusCondition(iatr.Status.Conditions, clusterinventoryv1alpha1.ClusterRoleFoundOrCreatedCondType)
	newCond := &metav1.Condition{
		Type:               clusterinventoryv1alpha1.ClusterRoleFoundOrCreatedCondType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: iatr.Generation,
	}
	if conditionutil.EqualCondition(currentCond, newCond) {
		return nil
	}

	meta.SetStatusCondition(&iatr.Status.Conditions, *newCond)
	if err := r.HubClient.Status().Update(ctx, iatr); err != nil {
		klog.ErrorS(err, "Failed to update status", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	return nil
}
