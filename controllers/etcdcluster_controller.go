/*
Copyright 2020 mars.

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
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	etcdv1alpha1 "github.com/wenhuwang/etcd-operator/api/v1alpha1"
)

// EtcdClusterReconciler reconciles a EtcdCluster object
type EtcdClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=etcd.mars.io,resources=etcdclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etcd.mars.io,resources=etcdclusters/status,verbs=get;update;patch

func (r *EtcdClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("etcdcluster", req.NamespacedName)

	// Get EtcdCluster instance
	var etcdCluster etcdv1alpha1.EtcdCluster
	if err := r.Client.Get(ctx, req.NamespacedName, &etcdCluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Get", "is nil", req)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// CreateOrUpdate Service
	log = r.Log.WithValues("service", req.NamespacedName)
	var svc corev1.Service
	svc.Name = etcdCluster.Name
	svc.Namespace = etcdCluster.Namespace
	or, err := ctrl.CreateOrUpdate(ctx, r, &svc, func() error {
		MutateHeadlessSvc(&etcdCluster, &svc)
		return controllerutil.SetControllerReference(&etcdCluster, &svc, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("CreateOrUpdate", "Service", or)

	// CreateOrUpdate StatefulSet
	// The first way
	//var desiredSts, sts appsv1.StatefulSet
	//MutateStatefulSet(&etcdCluster, &desiredSts)
	//err = r.Client.Get(ctx, client.ObjectKey{Namespace: etcdCluster.Namespace, Name: etcdCluster.Name}, &sts)
	//if err != nil {
	//	if errors.IsNotFound(err) {
	//		or, err = ctrl.CreateOrUpdate(ctx, r, &desiredSts, func() error {
	//			return controllerutil.SetControllerReference(&etcdCluster, &desiredSts, r.Scheme)
	//		})
	//		log.Info("CreateOrUpdate", "StatefulSet", "Create")
	//		if err != nil {
	//			// TODO: handler this error
	//			log.Error(err, "sts create error")
	//			return ctrl.Result{}, err
	//		}
	//	}
	//	log.Error(err, "sts get error")
	//	return ctrl.Result{}, err
	//}
	//
	//if !reflect.DeepEqual(&desiredSts.Spec, &sts.Spec) {
	//	sts.Spec = desiredSts.Spec
	//	sts.ObjectMeta.ResourceVersion = ""
	//	if err := r.Client.Update(ctx, &sts); err != nil {
	//		log.Error(err, "failed to Update statefulset")
	//		return ctrl.Result{}, err
	//	}
	//	log.Info("CreateOrUpdate", "StatefulSet", "Updated")
	//}
	// The Second way
	log = r.Log.WithValues("statefulset", req.NamespacedName)
	var sts appsv1.StatefulSet
	sts.Name = etcdCluster.Name
	sts.Namespace = etcdCluster.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r, &sts, func() error {
		MutateStatefulSet(&etcdCluster, &sts)
		sts.ObjectMeta.ResourceVersion = ""
		return controllerutil.SetControllerReference(&etcdCluster, &sts, r.Scheme)
	})
	log.Info("CreateOrUpdate", "StatefulSet", or)
	if err != nil {
		// TODO: handler this error
		return ctrl.Result{}, err
	}

	// handle finalizer
	ecFinalizerName := "mars.etcdcluster.io"
	if etcdCluster.DeletionTimestamp.IsZero() {
		if !containsString(etcdCluster.ObjectMeta.Finalizers, ecFinalizerName) {
			etcdCluster.ObjectMeta.Finalizers = append(etcdCluster.ObjectMeta.Finalizers, ecFinalizerName)
			if err := r.Update(context.Background(), &etcdCluster); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(etcdCluster.ObjectMeta.Finalizers, ecFinalizerName) {
			// 如果存在 finalizer 且与上述声明的 finalizer 匹配，那么执行对应 hook 逻辑
			if err := r.deleteExternalResources(&etcdCluster); err != nil {
				// 如果删除失败，则直接返回对应 err，controller 会自动执行重试逻辑
				return ctrl.Result{}, err
			}

			// 如果对应 hook 执行成功，那么清空 finalizers， k8s 删除对应资源
			etcdCluster.ObjectMeta.Finalizers = removeString(etcdCluster.ObjectMeta.Finalizers, ecFinalizerName)
			if err := r.Update(context.Background(), &etcdCluster); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

var (
	ownerKey = ".metadata.controller"
)

func (r *EtcdClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//if err := mgr.GetFieldIndexer().IndexField(&appsv1.StatefulSet{}, ownerKey, func(rawObj runtime.Object) []string {
	//	// grab the Deployment object, extract the owner...
	//	sts := rawObj.(*appsv1.StatefulSet)
	//	owner := metav1.GetControllerOf(sts)
	//	if owner == nil {
	//		return nil
	//	}
	//	// ...make sure it's a etcdCluster...
	//	if owner.APIVersion != etcdv1alpha1.GroupVersion.String() || owner.Kind != "MyKind" {
	//		return nil
	//	}
	//
	//	// ...and if so, return it
	//	return []string{owner.Name}
	//}); err != nil {
	//	return err
	//}

	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.EtcdCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func (r *EtcdClusterReconciler) deleteExternalResources(etcdCluster *etcdv1alpha1.EtcdCluster) error {
	//
	// 删除 etcdCLuster关联的外部资源逻辑
	//
	// 需要确保实现是幂等的
	time.Sleep(10 * time.Second)
	return nil
}
