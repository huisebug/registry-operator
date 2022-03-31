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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RegistrySpec defines the desired state of Registry
type RegistrySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// 业务服务对应的镜像，包括名称:tag
	Image *string `json:"image,omitempty"`
	// service端口
	NodePort *int32 `json:"nodePort,omitempty"`

	// 额外的卷挂载
	Volumes      []v1.Volume      `json:"volumes,omitempty"`
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`
	// 数据目录挂载
	Pvc *string `json:"pvc,omitempty"`
	// 是否密码认证
	Auth *Auth `json:"auth,omitempty"`
	Gc   *Gc   `json:"gc,omitempty"`
}

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type Gc struct {
	Schedule string `json:"schedule"`
}

// RegistryStatus defines the observed state of Registry
type RegistryStatus struct {
	Status string `json:"status"`
}

// 自定义kubectl get registry显示的字段
//+kubebuilder:printcolumn:JSONPath=".status.status",name=Status,type=string
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Registry is the Schema for the registries API
type Registry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistrySpec   `json:"spec,omitempty"`
	Status RegistryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RegistryList contains a list of Registry
type RegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Registry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Registry{}, &RegistryList{})
}
