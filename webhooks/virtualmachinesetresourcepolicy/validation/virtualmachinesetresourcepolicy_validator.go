// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net/http"
	"reflect"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha4"

	"github.com/vmware-tanzu/vm-operator/pkg/builder"
	pkgctx "github.com/vmware-tanzu/vm-operator/pkg/context"
	"github.com/vmware-tanzu/vm-operator/webhooks/common"
)

const (
	webHookName = "default"
)

// +kubebuilder:webhook:verbs=create;update,path=/default-validate-vmoperator-vmware-com-v1alpha4-virtualmachinesetresourcepolicy,mutating=false,failurePolicy=fail,groups=vmoperator.vmware.com,resources=virtualmachinesetresourcepolicies,versions=v1alpha4,name=default.validating.virtualmachinesetresourcepolicy.v1alpha4.vmoperator.vmware.com,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinesetresourcepolicies,verbs=get;list
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinesetresourcepolicies/status,verbs=get

// AddToManager adds the webhook to the provided manager.
func AddToManager(ctx *pkgctx.ControllerManagerContext, mgr ctrlmgr.Manager) error {
	hook, err := builder.NewValidatingWebhook(ctx, mgr, webHookName, NewValidator(mgr.GetClient()))
	if err != nil {
		return fmt.Errorf("failed to create VirtualMachineSetResourcePolicy validation webhook: %w", err)
	}
	mgr.GetWebhookServer().Register(hook.Path, hook)

	return nil
}

// NewValidator returns the package's Validator.
func NewValidator(_ client.Client) builder.Validator {
	return validator{
		converter: runtime.DefaultUnstructuredConverter,
	}
}

type validator struct {
	converter runtime.UnstructuredConverter
}

func (v validator) For() schema.GroupVersionKind {
	return vmopv1.GroupVersion.WithKind(reflect.TypeOf(vmopv1.VirtualMachineSetResourcePolicy{}).Name())
}

func (v validator) ValidateCreate(ctx *pkgctx.WebhookRequestContext) admission.Response {
	vmRP, err := v.vmRPFromUnstructured(ctx.Obj)
	if err != nil {
		return webhook.Errored(http.StatusBadRequest, err)
	}

	var fieldErrs field.ErrorList
	fieldErrs = append(fieldErrs, v.validateSpec(ctx, vmRP)...)

	validationErrs := make([]string, 0, len(fieldErrs))
	for _, fieldErr := range fieldErrs {
		validationErrs = append(validationErrs, fieldErr.Error())
	}

	return common.BuildValidationResponse(ctx, nil, validationErrs, nil)
}

func (v validator) ValidateDelete(*pkgctx.WebhookRequestContext) admission.Response {
	return admission.Allowed("")
}

func (v validator) ValidateUpdate(ctx *pkgctx.WebhookRequestContext) admission.Response {
	vmRP, err := v.vmRPFromUnstructured(ctx.Obj)
	if err != nil {
		return webhook.Errored(http.StatusBadRequest, err)
	}

	oldVMRP, err := v.vmRPFromUnstructured(ctx.OldObj)
	if err != nil {
		return webhook.Errored(http.StatusBadRequest, err)
	}

	var fieldErrs field.ErrorList
	fieldErrs = append(fieldErrs, v.validateAllowedChanges(ctx, vmRP, oldVMRP)...)

	validationErrs := make([]string, 0, len(fieldErrs))
	for _, fieldErr := range fieldErrs {
		validationErrs = append(validationErrs, fieldErr.Error())
	}

	return common.BuildValidationResponse(ctx, nil, validationErrs, nil)
}

func (v validator) validateSpec(ctx *pkgctx.WebhookRequestContext, vmRP *vmopv1.VirtualMachineSetResourcePolicy) field.ErrorList {
	var fieldErrs field.ErrorList
	specPath := field.NewPath("spec")

	fieldErrs = append(fieldErrs, v.validateResourcePool(ctx, specPath.Child("resourcePool"), vmRP.Spec.ResourcePool)...)
	fieldErrs = append(fieldErrs, v.validateFolder(ctx, specPath.Child("folder"), vmRP.Spec.Folder)...)
	fieldErrs = append(fieldErrs, v.validateClusterModules(ctx, specPath.Child("clusterModuleGroups"), vmRP.Spec.ClusterModuleGroups)...)

	return fieldErrs
}

func (v validator) validateResourcePool(ctx *pkgctx.WebhookRequestContext, fldPath *field.Path, rp vmopv1.ResourcePoolSpec) field.ErrorList {
	var fieldErrs field.ErrorList

	reservation, limits := rp.Reservations, rp.Limits
	reservationsPath := fldPath.Child("reservations")

	fieldErrs = append(fieldErrs, validateReservationAndLimit(reservationsPath.Child("cpu"), reservation.Cpu, limits.Cpu)...)
	fieldErrs = append(fieldErrs, validateReservationAndLimit(reservationsPath.Child("memory"), reservation.Memory, limits.Memory)...)

	return fieldErrs
}

func (v validator) validateFolder(ctx *pkgctx.WebhookRequestContext, specPath *field.Path, folder string) field.ErrorList {
	var fieldErrs field.ErrorList
	return fieldErrs
}

func (v validator) validateClusterModules(ctx *pkgctx.WebhookRequestContext, fldPath *field.Path, clusterModuleGroups []string) field.ErrorList {
	var fieldErrs field.ErrorList

	groupNames := map[string]struct{}{}
	for i, name := range clusterModuleGroups {
		if _, ok := groupNames[name]; ok {
			fieldErrs = append(fieldErrs, field.Duplicate(fldPath.Index(i).Child("clusterModuleGroups"), name))
			continue
		}
		groupNames[name] = struct{}{}
	}

	return fieldErrs
}

// validateAllowedChanges returns true only if immutable fields have not been modified.
func (v validator) validateAllowedChanges(ctx *pkgctx.WebhookRequestContext, vmRP, oldVMRP *vmopv1.VirtualMachineSetResourcePolicy) field.ErrorList {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	// Validate all fields under spec which are not allowed to change.
	allErrs = append(allErrs, validation.ValidateImmutableField(vmRP.Spec.ResourcePool, oldVMRP.Spec.ResourcePool, specPath.Child("resourcePool"))...)
	allErrs = append(allErrs, validation.ValidateImmutableField(vmRP.Spec.Folder, oldVMRP.Spec.Folder, specPath.Child("folder"))...)
	allErrs = append(allErrs, validation.ValidateImmutableField(vmRP.Spec.ClusterModuleGroups, oldVMRP.Spec.ClusterModuleGroups, specPath.Child("clusterModuleGroups"))...)

	return allErrs
}

// vmRPFromUnstructured returns the VirtualMachineSetResourcePolicy from the unstructured object.
func (v validator) vmRPFromUnstructured(obj runtime.Unstructured) (*vmopv1.VirtualMachineSetResourcePolicy, error) {
	vmRP := &vmopv1.VirtualMachineSetResourcePolicy{}
	if err := v.converter.FromUnstructured(obj.UnstructuredContent(), vmRP); err != nil {
		return nil, err
	}
	return vmRP, nil
}

func validateReservationAndLimit(reservationPath *field.Path, reservation, limit resource.Quantity) field.ErrorList {
	if reservation.IsZero() || limit.IsZero() || reservation.Value() <= limit.Value() {
		return nil
	}

	return field.ErrorList{
		field.Invalid(reservationPath, reservation.String(), "reservation value cannot exceed the limit value"),
	}
}
