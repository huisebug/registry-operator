/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var registrylog = logf.Log.WithName("registry-resource")

func (r *Registry) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// Registry资源create与update时，调用这个接口
//+kubebuilder:webhook:path=/mutate-container-huisebug-org-v1-registry,mutating=true,failurePolicy=fail,sideEffects=None,groups=container.huisebug.org,resources=registries,verbs=create;update,versions=v1,name=mregistry.kb.io,admissionReviewVersions={v1,v1beta1}

//开启填写默认值的逻辑
var _ webhook.Defaulter = &Registry{}

var defaultImage string = "registry:2"
var defaultRegistryNodePort = int32(30000)

// Default implements webhook.Defaulter so a webhook will be registered for the type
// 此处设置CR中的默认值
func (r *Registry) Default() {
	registrylog.Info("default", "name", r.Name)
	// 设置默认镜像
	if r.Spec.Image == nil {
		r.Spec.Image = &defaultImage
	}
	// 设置默认nodePort
	if r.Spec.NodePort == nil {
		r.Spec.NodePort = &defaultRegistryNodePort
	}
	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// Registry资源create与update时，调用这个接口
//+kubebuilder:webhook:path=/validate-container-huisebug-org-v1-registry,mutating=false,failurePolicy=fail,sideEffects=None,groups=container.huisebug.org,resources=registries,verbs=create;update,versions=v1,name=vregistry.kb.io,admissionReviewVersions={v1,v1beta1}

//开启校验字段的逻辑
var _ webhook.Validator = &Registry{}

// 同样的通过ValidateCreate、ValidateUpdate、ValidateDelete可实现validating webhook
// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Registry) ValidateCreate() error {
	registrylog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return r.validateRegistry()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Registry) ValidateUpdate(old runtime.Object) error {
	registrylog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return r.validateRegistry()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Registry) ValidateDelete() error {
	registrylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *Registry) validateRegistry() error {
	var allErrs field.ErrorList

	// Image tag不能使用latest
	if strings.Contains(*r.Spec.Image, "latest") {
		registrylog.Info("Image Invalid tag")
		err := field.Invalid(field.NewPath("spec").Child("ImageTag"),
			*r.Spec.Image,
			"tag should not contain latest")

		allErrs = append(allErrs, err)

		return apierrors.NewInvalid(
			schema.GroupKind{Group: "container.huisebug.org", Kind: "Registry"},
			r.Name,
			allErrs)
	} else {
		registrylog.Info("Image is valid")
		return nil
	}
}
