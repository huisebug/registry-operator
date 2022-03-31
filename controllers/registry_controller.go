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

package controllers

import (
	"context"
	"fmt"
	"reflect"

	containerv1 "github.com/huisebug/registry-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RegistryReconciler reconciles a Registry object
type RegistryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=container.huisebug.org,resources=registries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=container.huisebug.org,resources=registries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=container.huisebug.org,resources=registries/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//授予registry-operator可以往其他namespace写入event
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Registry object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile

// 主干代码
func (r *RegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	defer utilruntime.HandleCrash()
	registrieslog := ctrl.Log.WithValues("func", req.NamespacedName)

	// your logic here
	registrieslog.Info("1.start registry reconcile")

	// 实例化数据结构
	registry := &containerv1.Registry{}

	// 查询自定义资源Registry是否存在req.NamespacedName的值，如果存在，err的值为nil
	if err := r.Get(ctx, req.NamespacedName, registry); err != nil {
		if errors.IsNotFound(err) {
			registrieslog.Info("2.1 instance not found, 可能已移除", "func", "Reconcile")
			// 包reconcile的结构体
			return reconcile.Result{}, nil
		}
		registrieslog.Error(err, "2.2 error")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 如果处在删除中直接跳过
	if registry.DeletionTimestamp != nil {
		registrieslog.Info("registry in deleting", "name", req.String())
		return ctrl.Result{}, nil
	}

	if err := r.SyncRegistry(ctx, registry); err != nil {
		registrieslog.Error(err, "failed to sync registry", "name", req.String())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

const (
	registryport = 5000
)

func (r *RegistryReconciler) SyncRegistry(ctx context.Context, registry *containerv1.Registry) error {
	registrieslog := ctrl.Log.WithValues("func", "SyncRegistry")

	registry = registry.DeepCopy()
	registryname := types.NamespacedName{
		Namespace: registry.Namespace,
		Name:      registry.Name,
	}

	owner := []metav1.OwnerReference{
		{
			APIVersion:         registry.APIVersion,
			Kind:               registry.Kind,
			Name:               registry.Name,
			Controller:         pointer.BoolPtr(true),
			BlockOwnerDeletion: pointer.BoolPtr(true),
			UID:                registry.UID,
		},
	}

	labels := map[string]string{
		"app": registry.Name,
	}

	meta := metav1.ObjectMeta{
		Name:            registry.Name,
		Namespace:       registry.Namespace,
		Labels:          labels,
		OwnerReferences: owner,
	}

	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, registryname, deployment); err != nil {
		// 如果不是不存在错误，就返回错误信息
		if !errors.IsNotFound(err) {
			return err
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: meta,
			Spec:       r.GetDeploymentSpec(registry, labels),
		}
		// 级联删除deployment
		registrieslog.Info("set reference")
		if err := controllerutil.SetControllerReference(registry, deployment, r.Scheme); err != nil {
			registrieslog.Error(err, "SetControllerReference error")
			return err
		}
		// 新建deployment
		if err := r.Create(ctx, deployment); err != nil {
			return err
		}
		// 自定义事件Events内容，kubectl describe registry可以看到
		r.Recorder.Event(registry, corev1.EventTypeNormal, "SyncRegistry", "create registry deployment success")
		registrieslog.Info("create registry deployment success", "name", registryname.String())
	} else {

		// 不是新建那么就是更新deployment
		now := r.GetDeploymentSpec(registry, labels)
		old := r.GetDeploymentSpecFrom(deployment)
		// fmt.Printf("%+v\n", now)
		// fmt.Printf("%+v\n", old)
		if !reflect.DeepEqual(now, old) {
			update := deployment.DeepCopy()
			update.Spec = now
			if err := r.Update(ctx, update); err != nil {
				return err
			}
			// 自定义事件Events内容，kubectl describe registry可以看到
			r.Recorder.Event(registry, corev1.EventTypeNormal, "SyncRegistry", "update registry deployment success")
			registrieslog.Info("update registry deployment success", "name", registryname.String())
		}

	}

	service := &corev1.Service{}
	if err := r.Get(ctx, registryname, service); err != nil {
		if !errors.IsNotFound(err) {
			registrieslog.Error(err, "IsNotFound err")
			return err
		}

		// 新建service
		service = &corev1.Service{
			ObjectMeta: meta,
			Spec:       r.GetServiceSpec(registry, labels),
		}
		//级联删除service
		registrieslog.Info("set reference")
		if err := controllerutil.SetControllerReference(registry, service, r.Scheme); err != nil {
			registrieslog.Error(err, "SetControllerReference error")
			return err
		}
		if err := r.Create(ctx, service); err != nil {
			return err
		}
		// 自定义事件Events内容，kubectl describe registry可以看到
		r.Recorder.Event(registry, corev1.EventTypeNormal, "SyncRegistry", "create registry service success")
		registrieslog.Info("create registry service success", "name", registryname.String())

	} else {
		// 不是新建那么就是更新service
		now := r.GetServiceSpec(registry, labels)
		old := r.GetServiceSpecFrom(service)

		// fmt.Printf("%+v\n", now)
		// fmt.Printf("%+v\n", old)
		if !reflect.DeepEqual(now.Ports, old.Ports) {
			update := service.DeepCopy()
			update.Spec = old
			update.Spec.Ports = now.Ports
			if err := r.Update(ctx, update); err != nil {
				return err
			}
			// 自定义事件Events内容，kubectl describe registry可以看到
			r.Recorder.Event(registry, corev1.EventTypeNormal, "SyncRegistry", "update registry service success")
			registrieslog.Info("update registry service success", "name", registryname.String())
		}

	}
	if registry.Spec.Gc != nil {
		cronjob := &batchv1.CronJob{}
		if err := r.Get(ctx, registryname, cronjob); err != nil {
			if !errors.IsNotFound(err) {
				registrieslog.Error(err, "IsNotFound err")
				return err
			}

			// 新建cronjob
			cronjob = &batchv1.CronJob{
				ObjectMeta: meta,
				Spec:       r.GetCronJobSpec(registry, labels),
			}
			//级联删除service
			registrieslog.Info("set reference")
			if err := controllerutil.SetControllerReference(registry, cronjob, r.Scheme); err != nil {
				registrieslog.Error(err, "SetControllerReference error")
				return err
			}
			if err := r.Create(ctx, cronjob); err != nil {
				return err
			}
			// 自定义事件Events内容，kubectl describe registry可以看到
			r.Recorder.Event(registry, corev1.EventTypeNormal, "SyncRegistry", "create registry cronjobgc success")
			registrieslog.Info("create registry cronjobgc success", "name", registryname.String())

		} else {
			// 不是新建那么就是更新service
			now := r.GetCronJobSpec(registry, labels)
			old := r.GetCronJobSpecFrom(cronjob)

			// fmt.Printf("%+v\n", now)
			// fmt.Printf("%+v\n", old)
			if !reflect.DeepEqual(now, old) {
				update := cronjob.DeepCopy()
				update.Spec = old
				if err := r.Update(ctx, update); err != nil {
					return err
				}
				// 自定义事件Events内容，kubectl describe registry可以看到
				r.Recorder.Event(registry, corev1.EventTypeNormal, "SyncRegistry", "update cronjobgc success")
				registrieslog.Info("update cronjobgc success", "name", registryname.String())
			}

		}
	}
	// 更新状态
	nowStatus := containerv1.RegistryStatus{
		Status: "yes",
	}
	if !reflect.DeepEqual(registry.Status, nowStatus) {
		registry.Status = nowStatus
		registrieslog.Info("update registry status", "name", registryname.String())
		return r.Client.Status().Update(ctx, registry)
	}

	return nil
}

func (r *RegistryReconciler) GetDeploymentSpec(registry *containerv1.Registry, labels map[string]string) appsv1.DeploymentSpec {
	initContainers := r.MakeinitContainers(registry)
	containers := r.Makecontainers(registry)
	volumes := r.Makevolumes(registry)
	return appsv1.DeploymentSpec{
		Replicas: pointer.Int32Ptr(1),
		Selector: metav1.SetAsLabelSelector(labels),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers:     containers,
				InitContainers: initContainers,
				Volumes:        volumes,
			},
		},
	}
}

func (r *RegistryReconciler) GetDeploymentSpecFrom(deployment *appsv1.Deployment) appsv1.DeploymentSpec {
	spec := deployment.Spec.Template.Spec
	return appsv1.DeploymentSpec{
		Replicas: deployment.Spec.Replicas,
		Selector: deployment.Spec.Selector,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: deployment.Spec.Template.Labels,
			},
			Spec: corev1.PodSpec{
				Containers:     spec.Containers,
				InitContainers: spec.InitContainers,
				Volumes:        spec.Volumes,
			},
		},
	}
}

func (r *RegistryReconciler) GetServiceSpec(registry *containerv1.Registry, labels map[string]string) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       int32(registryport),
				TargetPort: intstr.FromInt(registryport),
				NodePort:   *registry.Spec.NodePort,
				Protocol:   corev1.ProtocolTCP,
			},
		},
		Selector: labels,
		Type:     corev1.ServiceTypeNodePort,
	}
}

func (r *RegistryReconciler) GetServiceSpecFrom(service *corev1.Service) corev1.ServiceSpec {
	return service.Spec
}

func (r *RegistryReconciler) GetCronJobSpec(registry *containerv1.Registry, labels map[string]string) batchv1.CronJobSpec {
	var volumes []corev1.Volume
	if registry.Spec.Pvc != nil {
		volumes = append(volumes,
			corev1.Volume{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: *registry.Spec.Pvc,
					},
				},
			},
		)
	}
	var volumeMounts []corev1.VolumeMount
	if registry.Spec.Pvc != nil {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "data",
				MountPath: "/var/lib/registry",
				SubPath:   registry.Name,
			},
		)
	}
	var env []corev1.EnvVar
	env = []corev1.EnvVar{
		{
			Name:  "REGISTRY_STORAGE_DELETE_ENABLED",
			Value: "true",
		},
	}

	return batchv1.CronJobSpec{
		Schedule: registry.Spec.Gc.Schedule,
		JobTemplate: batchv1.JobTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:            registry.Name,
								Image:           *registry.Spec.Image,
								ImagePullPolicy: "IfNotPresent",
								Env:             env,
								Command:         []string{"registry"},
								Args:            []string{"garbage-collect", "/etc/docker/registry/config.yml"},
								VolumeMounts:    volumeMounts,
							},
						},
						Volumes: volumes,
					},
				},
			},
		},
	}
}

func (r *RegistryReconciler) GetCronJobSpecFrom(cronjob *batchv1.CronJob) batchv1.CronJobSpec {
	return cronjob.Spec
}

func (r *RegistryReconciler) MakeinitContainers(registry *containerv1.Registry) []corev1.Container {

	var operatorInitContainers []corev1.Container
	if registry.Spec.Auth != nil {

		command := fmt.Sprintf("htpasswd -Bbn %s %s  > /auth/htpasswd", registry.Spec.Auth.Username, registry.Spec.Auth.Password)
		operatorInitContainers = []corev1.Container{
			{
				Name:            "htpasswd",
				Image:           "xmartlabs/htpasswd:latest",
				ImagePullPolicy: "IfNotPresent",

				Command: []string{
					"sh",
					"-c",
					command,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						MountPath: "/auth",
						Name:      "htpasswd",
					},
				},
			},
		}
	}

	return operatorInitContainers
}

func (r *RegistryReconciler) Makecontainers(registry *containerv1.Registry) []corev1.Container {

	var containers []corev1.Container
	var volumeMounts []corev1.VolumeMount
	var env []corev1.EnvVar
	if registry.Spec.Auth != nil {
		env = []corev1.EnvVar{
			{
				Name:  "REGISTRY_AUTH",
				Value: "htpasswd",
			},
			{
				Name:  "REGISTRY_AUTH_HTPASSWD_REALM",
				Value: "Registry Realm",
			},
			{
				Name:  "REGISTRY_AUTH_HTPASSWD_PATH",
				Value: "/auth/htpasswd",
			},
			{
				Name:  "REGISTRY_STORAGE_DELETE_ENABLED",
				Value: "true",
			},
		}
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				MountPath: "/auth",
				Name:      "htpasswd",
				ReadOnly:  true,
			},
		)
	}
	if registry.Spec.Pvc != nil {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "data",
				MountPath: "/var/lib/registry",
				SubPath:   registry.Name,
			},
		)
	}
	// volumeMounts = append(volumeMounts,
	// 	corev1.VolumeMount{
	// 		Name:      "conf",
	// 		MountPath: "/etc/docker/registry",
	// 		ReadOnly:  true,
	// 	},
	// )

	volumeMounts = append(volumeMounts, registry.Spec.VolumeMounts...)

	containers = append(containers,
		corev1.Container{
			Name:            registry.Name,
			Image:           *registry.Spec.Image,
			ImagePullPolicy: "IfNotPresent",
			Env:             env,
			Ports: []corev1.ContainerPort{
				{
					Name:          "http",
					Protocol:      corev1.Protocol("TCP"),
					ContainerPort: int32(registryport),
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("500m"),
					"memory": resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("1000m"),
					"memory": resource.MustParse("1Gi"),
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{
							Type:   1,
							IntVal: 0,
							StrVal: "http",
						},
					},
				},
				InitialDelaySeconds: int32(20),
				TimeoutSeconds:      int32(60),
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{
							Type:   1,
							IntVal: 0,
							StrVal: "http",
						},
					},
				},
				InitialDelaySeconds: int32(10),
				TimeoutSeconds:      int32(5),
			},
			VolumeMounts: volumeMounts,
		},
	)
	return containers
}

func (r *RegistryReconciler) Makevolumes(registry *containerv1.Registry) []corev1.Volume {
	var operatorVolume []corev1.Volume
	// operatorVolume = append(operatorVolume,
	// 	corev1.Volume{
	// 		Name: "conf",
	// 		VolumeSource: corev1.VolumeSource{
	// 			ConfigMap: &corev1.ConfigMapVolumeSource{
	// 				LocalObjectReference: corev1.LocalObjectReference{
	// 					Name: registry.Name,
	// 				},
	// 			},
	// 		},
	// 	},
	// )

	if registry.Spec.Pvc != nil {
		operatorVolume = append(operatorVolume,
			corev1.Volume{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: *registry.Spec.Pvc,
					},
				},
			},
		)
	}
	if registry.Spec.Auth != nil {
		operatorVolume = append(operatorVolume,
			corev1.Volume{
				Name: "htpasswd",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		)
	}
	return operatorVolume
}

// SetupWithManager sets up the controller with the Manager.
// 使用的是 Builder 模式，NewControllerManagerBy 和 For 方法都是给 Builder 传参，最重要的是最后一个方法 Complete
func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 3,
		}).
		For(&containerv1.Registry{}).
		//下面的单独监听会导致多次触发Reconcile的执行
		// Owns(&appsv1.Deployment{}).
		// Owns(&corev1.Service{}).
		Complete(r)
}
