package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonapi "github.com/openmcp-project/openmcp-operator/api/common"

	ctrlutils "github.com/openmcp-project/controller-utils/pkg/controller"

	v1alpha1 "github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

// Condition types and reasons for the Crossplane resource status.
const (
	ConditionTypeReconciled = "Reconciled"

	ReasonReconciled                    = "Reconciled"
	ReasonProviderConfigNotFound        = "ProviderConfigNotFound"
	ReasonClusterAccessPending          = "ClusterAccessPending"
	ReasonClusterAccessFailed           = "ClusterAccessFailed"
	ReasonFluxKubeconfigFailed          = "FluxKubeconfigFailed"
	ReasonReconciliationContextFailed   = "ReconciliationContextFailed"
	ReasonComponentBuildFailed          = "ComponentBuildFailed"
	ReasonComponentReconcileFailed      = "ComponentReconcileFailed"
	ReasonDeletionInProgress            = "DeletionInProgress"
	ReasonDeletionComponentCleanupError = "DeletionComponentCleanupError"
	ReasonFinalizerFailed               = "FinalizerFailed"
)

func computePhase(obj *v1alpha1.Crossplane, _ ctrlutils.ReconcileResult[*v1alpha1.Crossplane]) (string, error) {
	if !obj.GetDeletionTimestamp().IsZero() {
		return commonapi.StatusPhaseTerminating, nil
	}
	if len(obj.Status.Conditions) == 0 {
		return commonapi.StatusPhaseProgressing, nil
	}
	for _, c := range obj.Status.Conditions {
		if c.Status != metav1.ConditionTrue {
			return commonapi.StatusPhaseProgressing, nil
		}
	}
	return commonapi.StatusPhaseReady, nil
}

func smartRequeueConditional(rr ctrlutils.ReconcileResult[*v1alpha1.Crossplane]) ctrlutils.SmartRequeueAction {
	if rr.SmartRequeue != "" {
		return rr.SmartRequeue
	}
	if rr.Object == nil {
		return ctrlutils.SR_NO_REQUEUE
	}
	for _, c := range rr.Object.Status.Conditions {
		if c.Status != metav1.ConditionTrue {
			return ctrlutils.SR_RESET
		}
	}
	return ctrlutils.SR_BACKOFF
}
