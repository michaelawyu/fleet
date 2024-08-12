/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package authtokenrequest

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterinventoryv1alpha1 "go.goms.io/fleet/apis/clusterinventory/v1alpha1"
	"go.goms.io/fleet/pkg/utils"
)

const (
	// internalAuthTokenRequestCleanupFinalizer is the finalizer added to the AuthTokenRequest object if
	// an InternalAuthTokenRequest object has been created.
	internalAuthTokenRequestCleanupFinalizer = "kubernetes-fleet.io/internal-auth-token-request-cleanup"

	// clusterManagerName is the name of the cluster manager that owns ClusterProfile objects.
	clusterManagerName = "fleet"

	// internalAuthTokenRequestNameFormat is the format of the name of an InternalAuthTokenRequest object.
	// TO-DO (chenyu1): this format guarantees uniqueness, but no length check is performed.
	internalAuthTokenRequestNameFormat = "%s-%s"

	// ownerAuthTokenRequestNameAnnotation is the annotation key used to store the name of
	// the AuthTokenRequest object that owns an InternalAuthTokenRequest object.
	ownerAuthTokenRequestNameAnnotation = "kubernetes-fleet.io/auth-token-request.name"

	// ownerAuthTokenRequestNamespaceAnnotation is the annotation key used to store the namespace of
	// the AuthTokenRequest object that owns an InternalAuthTokenRequest object.
	ownerAuthTokenRequestNamespaceAnnotation = "kubernetes-fleet.io/auth-token-request.namespace"
)

const (
	validAuthTokenRequestCondType = "AuthTokenRequestValid"
)

// Reconciler reconciles an AuthTokenRequest object.
type Reconciler struct {
	HubClient               client.Client
	ClusterProfileNamespace string
}

// Reconcile reconciles an AuthTokenRequest object.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	atrRef := klog.KRef(req.Namespace, req.Name)
	startTime := time.Now()
	klog.V(2).InfoS("Reconciliation starts (auth token request controller)", "AuthTokenRequest", atrRef)
	defer func() {
		latency := time.Since(startTime).Milliseconds()
		klog.V(2).InfoS("Reconciliation ends (auth token request controller)", "AuthTokenRequest", atrRef, "latency", latency)
	}()

	// Retrieve the AuthTokenRequest object.
	atr := &clusterinventoryv1alpha1.AuthTokenRequest{}
	if err := r.HubClient.Get(ctx, req.NamespacedName, atr); err != nil {
		if errors.IsNotFound(err) {
			// AuthTokenRequest object is not found.

			// TO-DO (chenyu1): address a corner case and clean up the corresponding
			// InternalAuthTokenRequest object if it exists.
			return ctrl.Result{}, nil
		}
		klog.ErrorS(err, "Failed to get auth token request", "AuthTokenRequest", atrRef)
	}

	// Validate the target cluster profile.
	isTargetCPValid, err := r.validateTargetClusterProfileAndReportInStatus(ctx, atr)
	if err != nil {
		klog.ErrorS(err, "Failed to validate target cluster profile", "AuthTokenRequest", atrRef)
		return ctrl.Result{}, err
	}
	if !isTargetCPValid {
		// Report that the target cluster profile is invalid.
		klog.V(2).InfoS("Target cluster profile is not valid", "AuthTokenRequest", atrRef)
		if err := r.HubClient.Status().Update(ctx, atr); err != nil {
			klog.ErrorS(err, "Failed to update auth token request status", "AuthTokenRequest", atrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Note that we only process an AuthTokenRequest object if it links to a valid cluster profile,
	// and the AuthTokenRequest API is designed to be immutable after creation.
	targetCPName := atr.Spec.TargetClusterProfile.Name
	targetMCNamespace := fmt.Sprintf(utils.NamespaceNameFormat, targetCPName)
	iatrName := fmt.Sprintf(internalAuthTokenRequestNameFormat, atr.Namespace, atr.Name)

	// Clean up the InternalAuthTokenRequest object if the AuthTokenRequest object is being deleted.
	//
	// If a mirrored config map credential has been created, it will be cleaned up by the
	// garbage collector via owner reference.
	if !atr.DeletionTimestamp.IsZero() {
		if err := r.cleanupInternalAuthTokenRequest(ctx, targetMCNamespace, iatrName); err != nil {
			klog.ErrorS(err, "Failed to clean up internal auth token request when auth token request is deleted", "AuthTokenRequest", atrRef)
			return ctrl.Result{}, err
		}

		// Remove the cleanup finalizer from the AuthTokenRequest object.
		controllerutil.RemoveFinalizer(atr, internalAuthTokenRequestCleanupFinalizer)
		if err := r.HubClient.Update(ctx, atr); err != nil {
			klog.ErrorS(err, "Failed to remove cleanup finalizer", "AuthTokenRequest", atrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if the AuthTokenRequest object has the cleanup finalizer.
	if !controllerutil.ContainsFinalizer(atr, internalAuthTokenRequestCleanupFinalizer) {
		// Add the cleanup finalizer to the AuthTokenRequest object.
		controllerutil.AddFinalizer(atr, internalAuthTokenRequestCleanupFinalizer)
		if err := r.HubClient.Update(ctx, atr); err != nil {
			klog.ErrorS(err, "Failed to add cleanup finalizer", "AuthTokenRequest", atrRef)
			return ctrl.Result{}, err
		}
	}

	// Check if an InternalAuthTokenRequest object already exists.
	iatr := &clusterinventoryv1alpha1.InternalAuthTokenRequest{}
	err = r.HubClient.Get(ctx, client.ObjectKey{Namespace: targetMCNamespace, Name: iatrName}, iatr)
	switch {
	case err == nil:
		// Copy back the status to the AuthTokenRequest object.
		atr.Status = *iatr.Status.DeepCopy()

		// Copy back the credential (config map).
		//
		// For simplicity reasons, for now we will assume that if the reference includes
		// a name, a response has been ready.
		tokenResRef := iatr.Status.TokenResponse
		if len(tokenResRef.Name) > 0 {
			configMap := &corev1.ConfigMap{}
			configMapTypedName := client.ObjectKey{Namespace: targetMCNamespace, Name: tokenResRef.Name}
			if err := r.HubClient.Get(ctx, configMapTypedName, configMap); err != nil {
				klog.ErrorS(err, "Failed to get config map", "ConfigMap", configMapTypedName, "AuthTokenRequest", atrRef)
				return ctrl.Result{}, err
			}

			mirroredConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: atr.Namespace,
					Name:      atr.Name,
				},
			}
			// Create/update the mirrored config map in the AuthTokenRequest's owner namespace.
			createOrUpdateRes, err := controllerutil.CreateOrUpdate(ctx, r.HubClient, mirroredConfigMap, func() error {
				mirroredConfigMap.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: atr.APIVersion,
						Kind:       atr.Kind,
						Name:       atr.Name,
						UID:        atr.UID,
					},
				}

				mirroredConfigMap.Data = configMap.Data
				return nil
			})
			if err != nil {
				klog.ErrorS(err, "Failed to create/update mirrored config map",
					"ConfigMap", klog.KObj(configMap),
					"MirroredConfigMap", klog.KObj(mirroredConfigMap),
					"AuthTokenRequest", atrRef,
					"Operation", createOrUpdateRes)
				return ctrl.Result{}, err
			}
		}

		if err := r.HubClient.Status().Update(ctx, atr); err != nil {
			klog.ErrorS(err, "Failed to update auth token request status", "AuthTokenRequest", atrRef)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	case errors.IsNotFound(err):
		// Create an InternalAuthTokenRequest object.
		iatr = &clusterinventoryv1alpha1.InternalAuthTokenRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: targetMCNamespace,
				Name:      iatrName,
				Annotations: map[string]string{
					ownerAuthTokenRequestNameAnnotation:      atr.Name,
					ownerAuthTokenRequestNamespaceAnnotation: atr.Namespace,
				},
			},
			Spec: *atr.Spec.DeepCopy(),
		}
		if err := r.HubClient.Create(ctx, iatr); err != nil {
			klog.ErrorS(err, "Failed to create internal auth token request", "AuthTokenRequest", atrRef, "InternalAuthTokenRequest", klog.KObj(iatr))
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	default:
		// An unexpected error occurred.
		klog.ErrorS(err, "Failed to get internal auth token request", "AuthTokenRequest", atrRef, "InternalAuthTokenRequest", klog.KRef(targetMCNamespace, iatrName))
		return ctrl.Result{}, err
	}
}

// SetupWithManager sets up the controller with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	iatrEventHandlers := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
		iatr, ok := a.(*clusterinventoryv1alpha1.InternalAuthTokenRequest)
		if !ok {
			return []reconcile.Request{}
		}

		// Extract the owner AuthTokenRequest's name and namespace.
		ownerName, ownerNamespace := iatr.Annotations[ownerAuthTokenRequestNameAnnotation], iatr.Annotations[ownerAuthTokenRequestNamespaceAnnotation]
		if len(ownerName) == 0 || len(ownerNamespace) == 0 {
			klog.InfoS("Owner AuthTokenRequest's name or namespace is missing",
				"InternalAuthTokenRequest", klog.KObj(iatr),
				"OwnerAuthTokenRequestName", ownerName,
				"OwnerAuthTokenRequestNamespace", ownerNamespace)
			return []reconcile.Request{}
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: ownerNamespace,
					Name:      ownerName,
				},
			},
		}
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterinventoryv1alpha1.AuthTokenRequest{}).
		Watches(&clusterinventoryv1alpha1.InternalAuthTokenRequest{}, iatrEventHandlers).
		Complete(r)
}

// cleanupInternalAuthTokenRequest cleans up the InternalAuthTokenRequest object associated with
// an AuthTokenRequest object.
func (r *Reconciler) cleanupInternalAuthTokenRequest(ctx context.Context, mcNamespace, iatrName string) error {
	iatr := &clusterinventoryv1alpha1.InternalAuthTokenRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: mcNamespace,
			Name:      iatrName,
		},
	}
	if err := r.HubClient.Delete(ctx, iatr); err != nil && !errors.IsNotFound(err) {
		klog.ErrorS(err, "Failed to delete InternalAuthTokenRequest", "InternalAuthTokenRequest", klog.KObj(iatr))
		return err
	}
	return nil
}

// validateTargetClusterProfileAndReportInStatus validates the target cluster profile associated
// with an AuthTokenRequest object and reports the result as a condition in its status.
func (r *Reconciler) validateTargetClusterProfileAndReportInStatus(ctx context.Context, atr *clusterinventoryv1alpha1.AuthTokenRequest) (bool, error) {
	// Check if a target cluster profile name is specified. Note that no namespace check is
	// performed here as this controller is configured to watch only AuthTokenRequest objects
	// from a specific namespace.
	targetCPName := atr.Spec.TargetClusterProfile.Name
	if len(targetCPName) == 0 {
		meta.SetStatusCondition(&atr.Status.Conditions, metav1.Condition{
			Type:    validAuthTokenRequestCondType,
			Status:  metav1.ConditionFalse,
			Reason:  "InvalidTargetClusterProfile",
			Message: "No cluster profile name is specified",
		})
		return false, nil
	}

	// Retrieve the cluster profile.
	cp := &clusterinventoryv1alpha1.ClusterProfile{}
	if err := r.HubClient.Get(ctx, client.ObjectKey{Namespace: r.ClusterProfileNamespace, Name: targetCPName}, cp); err != nil {
		if errors.IsNotFound(err) {
			meta.SetStatusCondition(&atr.Status.Conditions, metav1.Condition{
				Type:    validAuthTokenRequestCondType,
				Status:  metav1.ConditionFalse,
				Reason:  "InvalidTargetClusterProfile",
				Message: "The target cluster profile is not found",
			})
			return false, nil
		}
		klog.ErrorS(err, "Failed to get cluster profile", "ClusterProfile", klog.KObj(cp), "AuthTokenRequest", klog.KObj(atr))
		return false, err
	}

	// Check if the cluster profile has been marked for deletion.
	if !cp.DeletionTimestamp.IsZero() {
		meta.SetStatusCondition(&atr.Status.Conditions, metav1.Condition{
			Type:    validAuthTokenRequestCondType,
			Status:  metav1.ConditionFalse,
			Reason:  "InvalidTargetClusterProfile",
			Message: "The target cluster profile has been marked for deletion",
		})
		return false, nil
	}

	// Check if the cluster profile is managed by the expected manager.
	if cp.Spec.ClusterManager.Name != clusterManagerName {
		meta.SetStatusCondition(&atr.Status.Conditions, metav1.Condition{
			Type:    validAuthTokenRequestCondType,
			Status:  metav1.ConditionFalse,
			Reason:  "InvalidTargetClusterProfile",
			Message: "The target cluster profile is not managed by Fleet",
		})
		return false, nil
	}

	return true, nil
}
