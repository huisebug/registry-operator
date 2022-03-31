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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var registrycleanlog = logf.Log.WithName("registryclean-resource")

func (r *RegistryClean) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-container-huisebug-org-v1-registryclean,mutating=true,failurePolicy=fail,sideEffects=None,groups=container.huisebug.org,resources=registrycleans,verbs=create;update,versions=v1,name=mregistryclean.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &RegistryClean{}

var defaultRegistryCleanNodePort = int32(30000)
var defaultRegistryCleanKeepimagenum = int32(100)

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *RegistryClean) Default() {
	registrycleanlog.Info("default", "name", r.Name)
	// 设置默认nodePort
	if r.Spec.NodePort == nil {
		r.Spec.NodePort = &defaultRegistryCleanNodePort
	}
	if r.Spec.Keepimage.Keepimagenum == nil {
		r.Spec.Keepimage.Keepimagenum = &defaultRegistryCleanKeepimagenum
	}
	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-container-huisebug-org-v1-registryclean,mutating=false,failurePolicy=fail,sideEffects=None,groups=container.huisebug.org,resources=registrycleans,verbs=create;update,versions=v1,name=vregistryclean.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &RegistryClean{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *RegistryClean) ValidateCreate() error {
	registrycleanlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *RegistryClean) ValidateUpdate(old runtime.Object) error {
	registrycleanlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *RegistryClean) ValidateDelete() error {
	registrycleanlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
