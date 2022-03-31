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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	containerv1 "github.com/huisebug/registry-operator/api/v1"
)

// RegistryCleanReconciler reconciles a RegistryClean object
type RegistryCleanReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=container.huisebug.org,resources=registrycleans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=container.huisebug.org,resources=registrycleans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=container.huisebug.org,resources=registrycleans/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

//授予registry-operator可以往其他namespace写入event
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RegistryClean object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *RegistryCleanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	defer utilruntime.HandleCrash()
	registriycleanlog := ctrl.Log.WithValues("func", req.NamespacedName)

	// your logic here
	registriycleanlog.Info("1.start registryclean reconcile")

	// 实例化数据结构
	registryclean := &containerv1.RegistryClean{}
	// 查询自定义资源Registry是否存在req.NamespacedName的值，如果存在，err的值为nil
	if err := r.Get(ctx, req.NamespacedName, registryclean); err != nil {
		if errors.IsNotFound(err) {
			registriycleanlog.Info("2.1 instance not found, 可能已移除", "func", "Reconcile")
			// 包reconcile的结构体
			return reconcile.Result{}, nil
		}
		registriycleanlog.Error(err, "2.2 error")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 如果处在删除中直接跳过
	if registryclean.DeletionTimestamp != nil {
		registriycleanlog.Info("registryclean in deleting", "name", req.String())
		return ctrl.Result{}, nil
	}

	if err := r.SyncRegistry(ctx, registryclean); err != nil {
		registriycleanlog.Error(err, "failed to sync registryclean", "name", req.String())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

const (
	registrycleanport = 8080
)

func (r *RegistryCleanReconciler) SyncRegistry(ctx context.Context, registryclean *containerv1.RegistryClean) error {
	registrieslog := ctrl.Log.WithValues("func", "SyncRegistryClean")

	registryclean = registryclean.DeepCopy()
	registryname := types.NamespacedName{
		Namespace: registryclean.Namespace,
		Name:      registryclean.Name,
	}

	owner := []metav1.OwnerReference{
		{
			APIVersion:         registryclean.APIVersion,
			Kind:               registryclean.Kind,
			Name:               registryclean.Name,
			Controller:         pointer.BoolPtr(true),
			BlockOwnerDeletion: pointer.BoolPtr(true),
			UID:                registryclean.UID,
		},
	}

	labels := map[string]string{
		"app": registryclean.Name,
	}

	meta := metav1.ObjectMeta{
		Name:            registryclean.Name,
		Namespace:       registryclean.Namespace,
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
			Spec:       r.GetDeploymentSpec(registryclean, labels),
		}
		// 级联删除deployment
		registrieslog.Info("set reference")
		if err := controllerutil.SetControllerReference(registryclean, deployment, r.Scheme); err != nil {
			registrieslog.Error(err, "SetControllerReference error")
			return err
		}
		// 新建deployment
		if err := r.Create(ctx, deployment); err != nil {
			return err
		}
		// 自定义事件Events内容，kubectl describe registry可以看到
		r.Recorder.Event(registryclean, corev1.EventTypeNormal, "SyncRegistry", "create registryclean deployment success")
		registrieslog.Info("create registryclean deployment success", "name", registryname.String())
	} else {

		// 不是新建那么就是更新deployment
		now := r.GetDeploymentSpec(registryclean, labels)
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
			r.Recorder.Event(registryclean, corev1.EventTypeNormal, "SyncRegistry", "update registryclean deployment success")
			registrieslog.Info("update registryclean deployment success", "name", registryname.String())
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
			Spec:       r.GetServiceSpec(registryclean, labels),
		}
		//级联删除service
		registrieslog.Info("set reference")
		if err := controllerutil.SetControllerReference(registryclean, service, r.Scheme); err != nil {
			registrieslog.Error(err, "SetControllerReference error")
			return err
		}
		if err := r.Create(ctx, service); err != nil {
			return err
		}
		// 自定义事件Events内容，kubectl describe registry可以看到
		r.Recorder.Event(registryclean, corev1.EventTypeNormal, "SyncRegistry", "create registryclean service success")
		registrieslog.Info("create registryclean service success", "name", registryname.String())

	} else {
		// 不是新建那么就是更新service
		now := r.GetServiceSpec(registryclean, labels)
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
			r.Recorder.Event(registryclean, corev1.EventTypeNormal, "SyncRegistry", "update service success")
			registrieslog.Info("update service success", "name", registryname.String())
		}

	}
	if registryclean.Spec.Keepimage != nil {

		cronjob := &batchv1.CronJob{}
		if err := r.Get(ctx, registryname, cronjob); err != nil {
			if !errors.IsNotFound(err) {
				registrieslog.Error(err, "IsNotFound err")
				return err
			}

			// 新建cronjob
			cronjob = &batchv1.CronJob{
				ObjectMeta: meta,
				Spec:       r.GetCronJobSpec(registryclean, labels),
			}
			//级联删除service
			registrieslog.Info("set reference")
			if err := controllerutil.SetControllerReference(registryclean, cronjob, r.Scheme); err != nil {
				registrieslog.Error(err, "SetControllerReference error")
				return err
			}
			if err := r.Create(ctx, cronjob); err != nil {
				return err
			}
			// 自定义事件Events内容，kubectl describe registry可以看到
			r.Recorder.Event(registryclean, corev1.EventTypeNormal, "SyncRegistry", "create registryclean cronjobkeep success")
			registrieslog.Info("create registryclean cronjobkeep success", "name", registryname.String())

		} else {
			// 不是新建那么就是更新service
			now := r.GetCronJobSpec(registryclean, labels)
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
				r.Recorder.Event(registryclean, corev1.EventTypeNormal, "SyncRegistry", "update cronjobkeep success")
				registrieslog.Info("update cronjobkeep success", "name", registryname.String())
			}

		}
	}

	// 更新状态
	nowStatus := containerv1.RegistryCleanStatus{
		Status: "yes",
	}
	if !reflect.DeepEqual(registryclean.Status, nowStatus) {
		registryclean.Status = nowStatus
		registrieslog.Info("update registryclean status", "name", registryname.String())
		return r.Client.Status().Update(ctx, registryclean)
	}

	return nil

}

func (r *RegistryCleanReconciler) GetDeploymentSpec(registryclean *containerv1.RegistryClean, labels map[string]string) appsv1.DeploymentSpec {

	containers := r.Makecontainers(registryclean)
	volumes := r.Makevolumes(registryclean)
	return appsv1.DeploymentSpec{
		//elasticsearch必须先删除pod，才可以新建
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Replicas: pointer.Int32Ptr(1),
		Selector: metav1.SetAsLabelSelector(labels),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers: containers,
				Volumes:    volumes,
			},
		},
	}
}

func (r *RegistryCleanReconciler) GetDeploymentSpecFrom(deployment *appsv1.Deployment) appsv1.DeploymentSpec {
	spec := deployment.Spec.Template.Spec
	return appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
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

func (r *RegistryCleanReconciler) GetServiceSpec(registryclean *containerv1.RegistryClean, labels map[string]string) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       int32(registrycleanport),
				TargetPort: intstr.FromInt(registrycleanport),
				NodePort:   *registryclean.Spec.NodePort,
				Protocol:   corev1.ProtocolTCP,
			},
		},
		Selector: labels,
		Type:     corev1.ServiceTypeNodePort,
	}
}

func (r *RegistryCleanReconciler) GetServiceSpecFrom(service *corev1.Service) corev1.ServiceSpec {
	return service.Spec
}

func (r *RegistryCleanReconciler) GetCronJobSpec(registryclean *containerv1.RegistryClean, labels map[string]string) batchv1.CronJobSpec {
	var volumes []corev1.Volume
	if registryclean.Spec.Pvc != nil {
		volumes = append(volumes,
			corev1.Volume{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: *registryclean.Spec.Pvc,
					},
				},
			},
		)
	}
	var volumeMounts []corev1.VolumeMount
	if registryclean.Spec.Pvc != nil {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "data",
				MountPath: "/data/projects/dockerregistryclean/data/",
				SubPath:   registryclean.Name,
			},
		)
	}
	var env []corev1.EnvVar
	env = []corev1.EnvVar{
		{
			Name:  "keepimagenum",
			Value: string(*registryclean.Spec.Keepimage.Keepimagenum),
		},
	}

	return batchv1.CronJobSpec{
		Schedule: registryclean.Spec.Keepimage.Schedule,
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
								Name:            registryclean.Name,
								Image:           *registryclean.Spec.Image,
								ImagePullPolicy: "IfNotPresent",
								Env:             env,
								Command:         []string{"/data/keepimagenum"},

								VolumeMounts: volumeMounts,
							},
						},
						Volumes: volumes,
					},
				},
			},
		},
	}
}

func (r *RegistryCleanReconciler) GetCronJobSpecFrom(cronjob *batchv1.CronJob) batchv1.CronJobSpec {
	return cronjob.Spec
}

func (r *RegistryCleanReconciler) Makecontainers(registryclean *containerv1.RegistryClean) []corev1.Container {

	var containers []corev1.Container
	var volumeMounts []corev1.VolumeMount
	if registryclean.Spec.Pvc != nil {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "data",
				MountPath: "/usr/share/elasticsearch/data",
				SubPath:   registryclean.Name + "elasticsearch",
			},
		)
	}
	volumeMounts = append(volumeMounts, registryclean.Spec.VolumeMounts...)

	containers = append(containers,
		corev1.Container{
			Name:            "elasticsearch",
			Image:           "elasticsearch:7.8.1",
			ImagePullPolicy: "IfNotPresent",
			Env: []corev1.EnvVar{
				{
					Name:  "ES_JAVA_OPTS",
					Value: "-Xms512m -Xmx512m",
				},
				{
					Name:  "bootstrap.memory_lock",
					Value: "true",
				},
				{
					Name:  "discovery.type",
					Value: "single-node",
				},
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "elasticsearch",
					Protocol:      corev1.Protocol("TCP"),
					ContainerPort: int32(9200),
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{
							Type:   1,
							IntVal: 0,
							StrVal: "elasticsearch",
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
							StrVal: "elasticsearch",
						},
					},
				},
				InitialDelaySeconds: int32(10),
				TimeoutSeconds:      int32(5),
			},
			VolumeMounts: volumeMounts,
		},
	)
	containers = append(containers,
		corev1.Container{
			Name:            registryclean.Name,
			Image:           *registryclean.Spec.Image,
			ImagePullPolicy: "IfNotPresent",
			Env: []corev1.EnvVar{
				{
					Name:  "esurl",
					Value: "http://127.0.0.1:9200",
				},
				{
					Name:  "dockerrepourl",
					Value: "http://" + *registryclean.Spec.Registryurl,
				},
				{
					Name:  "authbase64",
					Value: *registryclean.Spec.Registryauth,
				},
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "http",
					Protocol:      corev1.Protocol("TCP"),
					ContainerPort: int32(registrycleanport),
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("300m"),
					"memory": resource.MustParse("380Mi"),
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
				TimeoutSeconds:      int32(10),
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
		},
	)
	return containers
}

func (r *RegistryCleanReconciler) Makevolumes(registryclean *containerv1.RegistryClean) []corev1.Volume {
	var operatorVolume []corev1.Volume
	if registryclean.Spec.Pvc != nil {
		operatorVolume = append(operatorVolume,
			corev1.Volume{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: *registryclean.Spec.Pvc,
					},
				},
			},
		)
	}

	return operatorVolume
}

// SetupWithManager sets up the controller with the Manager.
func (r *RegistryCleanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&containerv1.RegistryClean{}).
		Complete(r)
}
