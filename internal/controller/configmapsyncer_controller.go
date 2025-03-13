/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	syncerv1alpha1 "DynamicConfigMapPropagator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

// ConfigMapSyncerReconciler reconciles a ConfigMapSyncer object
type ConfigMapSyncerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	logger logr.Logger
	ctx    context.Context
}

//+kubebuilder:rbac:groups=syncer.custom,resources=configmapsyncers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncer.custom,resources=configmapsyncers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncer.custom,resources=configmapsyncers/finalizers,verbs=update

func (r *ConfigMapSyncerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.FromContext(ctx)
	r.ctx = ctx

	syncer := syncerv1alpha1.ConfigMapSyncer{}
	err := r.Get(r.ctx, req.NamespacedName, &syncer)
	if err == nil {
		r.logger.Info("Reconciling ConfigMapSyncer", "name", req.Name, "namespace", req.Namespace)
		return r.handleSyncerReconciliation(&syncer)
	}

	configmap := corev1.ConfigMap{}
	err = r.Get(r.ctx, req.NamespacedName, &configmap)
	if err == nil {
		r.logger.Info("Reconciling ConfigMap", "name", req.Name, "namespace", req.Namespace)
		return r.handleConfigMapReconciliation(&configmap)

	} else if req.Namespace == "" && req.Name != "" {
		ns := corev1.Namespace{}
		err = r.Get(r.ctx, req.NamespacedName, &ns)
		if err == nil {
			r.logger.Info("Reconciling Namespace", "name", req.Name)
			return r.handleNamespaceReconciliation(&ns)

		}
	} else if errors.IsNotFound(err) {
		r.logger.Info("Reconciling Missing ConfigMap", "name", req.Name, "namespace", req.Namespace)
		return r.handleMissingConfigMapReconciliation(req.Name, req.Namespace)
	}

	r.logger.Info("Resource not found, ignoring", "name", req.Name, "namespace", req.Namespace)
	return reconcile.Result{}, client.IgnoreNotFound(err)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapSyncerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncerv1alpha1.ConfigMapSyncer{}).
		Watches(
			&corev1.ConfigMap{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					cm, ok := e.Object.(*corev1.ConfigMap)
					if !ok {
						return false
					}
					return r.doWatchConfigMap(cm)
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					cm, ok := e.ObjectNew.(*corev1.ConfigMap)
					if !ok {
						return false
					}
					return r.doWatchConfigMap(cm)
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					cm, ok := e.Object.(*corev1.ConfigMap)
					if !ok {
						return false
					}
					return r.doWatchConfigMap(cm)
				},
			}),
		).
		Watches(
			&corev1.Namespace{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					ns, ok := e.Object.(*corev1.Namespace)
					if !ok {
						return false
					}
					return r.doWatchNamespace(ns)
				},
			}),
		).
		Complete(r)
}

func (r *ConfigMapSyncerReconciler) doWatchConfigMap(cm *corev1.ConfigMap) bool {
	crList := &syncerv1alpha1.ConfigMapSyncerList{}
	err := r.List(context.Background(), crList)
	if err != nil {
		return false
	}

	for _, cr := range crList.Items {
		if cr.Spec.MasterConfigMap.Name == cm.Name && cr.Spec.MasterConfigMap.Namespace == cm.Namespace {
			return true
		} else {
			for _, namespace := range cr.Spec.TargetNamespaces {
				if cm.Namespace == namespace && cm.Name == cr.Spec.MasterConfigMap.Name && (cm.Immutable == nil || *cm.Immutable == false) {
					return true
				}
			}
		}
	}
	return false
}

func (r *ConfigMapSyncerReconciler) doWatchNamespace(ns *corev1.Namespace) bool {
	crList := &syncerv1alpha1.ConfigMapSyncerList{}
	err := r.List(context.Background(), crList)
	if err != nil {
		return false
	}

	for _, cr := range crList.Items {
		if cr.Spec.MasterConfigMap.Namespace == ns.Name {
			return true
		} else {
			for _, namespace := range cr.Spec.TargetNamespaces {
				if ns.Name == namespace {
					return true
				}
			}
		}
	}
	return false
}

func (r *ConfigMapSyncerReconciler) handleSyncerReconciliation(syncer *syncerv1alpha1.ConfigMapSyncer) (reconcile.Result, error) {
	masterConfigmap := &corev1.ConfigMap{}
	err := r.Get(r.ctx, client.ObjectKey{Name: syncer.Spec.MasterConfigMap.Name, Namespace: syncer.Spec.MasterConfigMap.Namespace}, masterConfigmap)
	if err != nil {
		if errors.IsNotFound(err) {
			r.logger.Error(err, "Master ConfigMap Not Found!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace)
			return reconcile.Result{}, client.IgnoreNotFound(err)
		} else {
			r.logger.Error(err, "Failed to Get Master ConfigMap!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace)
			return reconcile.Result{Requeue: true}, err
		}
	}
	syncer.Status.Data = []syncerv1alpha1.SyncerData{}
	for _, targetNamespace := range syncer.Spec.TargetNamespaces {
		syncerData, err := r.syncConfigMap(masterConfigmap, targetNamespace, syncer)
		if err != nil {
			r.logger.Error(err, "Failed to sync ConfigMap to namespace!", "namespace", targetNamespace)
		}
		if syncerData != nil {
			syncer.Status.Data = append(syncer.Status.Data, *syncerData)
		}
	}
	if len(syncer.Status.Data) > 0 {
		syncer.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
		err = r.Status().Update(r.ctx, syncer)
		if err != nil {
			r.logger.Error(err, "Failed to update status")
		}
	}
	return reconcile.Result{}, nil
}

func (r *ConfigMapSyncerReconciler) handleConfigMapReconciliation(configmap *corev1.ConfigMap) (reconcile.Result, error) {
	syncerList := syncerv1alpha1.ConfigMapSyncerList{}
	if err := r.List(r.ctx, &syncerList); err != nil {
		r.logger.Error(err, "Failed to list ConfigMapSyncer resources")
		return reconcile.Result{}, err
	}

	for _, syncer := range syncerList.Items {
		masterConfigmap := &corev1.ConfigMap{}

		if syncer.Spec.MasterConfigMap.Name == configmap.Name && syncer.Spec.MasterConfigMap.Namespace == configmap.Namespace {
			// Master ConfigMap Has Been Reconciled
			masterConfigmap = configmap
			syncer.Status.Data = []syncerv1alpha1.SyncerData{}
			for _, targetNamespace := range syncer.Spec.TargetNamespaces {
				syncerData, err := r.syncConfigMap(masterConfigmap, targetNamespace, &syncer)
				if err != nil {
					r.logger.Error(err, "Failed to sync ConfigMap to namespace!", "namespace", targetNamespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
				}
				if syncerData != nil {
					syncer.Status.Data = append(syncer.Status.Data, *syncerData)
				}
			}
			if len(syncer.Status.Data) > 0 {
				syncer.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
				err := r.Status().Update(r.ctx, &syncer)
				if err != nil {
					r.logger.Error(err, "Failed to update status")
				}
			}
		} else {
			// Slave ConfigMap Has Been Reconciled
			err := r.Get(r.ctx, client.ObjectKey{Name: syncer.Spec.MasterConfigMap.Name, Namespace: syncer.Spec.MasterConfigMap.Namespace}, masterConfigmap)
			if err != nil {
				if errors.IsNotFound(err) {
					r.logger.Error(err, "Master ConfigMap Not Found!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
					return reconcile.Result{}, client.IgnoreNotFound(err)
				} else {
					r.logger.Error(err, "Failed to Get Master ConfigMap!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
					return reconcile.Result{Requeue: true}, err
				}
			}
			syncer.Status.Data = []syncerv1alpha1.SyncerData{}
			syncerData, err := r.syncConfigMap(masterConfigmap, configmap.Namespace, &syncer)
			if err != nil {
				r.logger.Error(err, "Failed to sync ConfigMap to namespace!", "namespace", configmap.Namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
			}
			if syncerData != nil {
				syncer.Status.Data = append(syncer.Status.Data, *syncerData)
				syncer.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
				err = r.Status().Update(r.ctx, &syncer)
				if err != nil {
					r.logger.Error(err, "Failed to update status")
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *ConfigMapSyncerReconciler) handleNamespaceReconciliation(ns *corev1.Namespace) (reconcile.Result, error) {
	syncerList := syncerv1alpha1.ConfigMapSyncerList{}
	if err := r.List(r.ctx, &syncerList); err != nil {
		r.logger.Error(err, "Failed to list ConfigMapSyncer resources")
		return reconcile.Result{}, err
	}

	for _, syncer := range syncerList.Items {
		masterConfigmap := &corev1.ConfigMap{}
		err := r.Get(r.ctx, client.ObjectKey{Name: syncer.Spec.MasterConfigMap.Name, Namespace: syncer.Spec.MasterConfigMap.Namespace}, masterConfigmap)
		if err != nil {
			if errors.IsNotFound(err) {
				r.logger.Error(err, "Master ConfigMap Not Found!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
				return reconcile.Result{}, client.IgnoreNotFound(err)
			} else {
				r.logger.Error(err, "Failed to Get Master ConfigMap!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
				return reconcile.Result{Requeue: true}, err
			}
		}

		for _, targetNamespace := range syncer.Spec.TargetNamespaces {
			if ns.Name == targetNamespace {
				syncer.Status.Data = []syncerv1alpha1.SyncerData{}
				syncerData, err := r.syncConfigMap(masterConfigmap, ns.Name, &syncer)
				if err != nil {
					r.logger.Error(err, "Failed to sync ConfigMap to namespace!", "namespace", ns.Name, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
				}
				if syncerData != nil {
					syncer.Status.Data = append(syncer.Status.Data, *syncerData)
					syncer.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
					err = r.Status().Update(r.ctx, &syncer)
					if err != nil {
						r.logger.Error(err, "Failed to update status")
					}
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *ConfigMapSyncerReconciler) handleMissingConfigMapReconciliation(configMapName string, namespace string) (reconcile.Result, error) {
	syncerList := syncerv1alpha1.ConfigMapSyncerList{}
	if err := r.List(r.ctx, &syncerList); err != nil {
		r.logger.Error(err, "Failed to list ConfigMapSyncer resources")
		return reconcile.Result{}, err
	}

	for _, syncer := range syncerList.Items {
		masterConfigmap := &corev1.ConfigMap{}

		if syncer.Spec.MasterConfigMap.Name == configMapName && syncer.Spec.MasterConfigMap.Namespace == namespace {
			// Master ConfigMap Removed Reconciled
			return reconcile.Result{}, nil
		} else {
			// Slave ConfigMap Has Been Reconciled
			err := r.Get(r.ctx, client.ObjectKey{Name: syncer.Spec.MasterConfigMap.Name, Namespace: syncer.Spec.MasterConfigMap.Namespace}, masterConfigmap)
			if err != nil {
				if errors.IsNotFound(err) {
					r.logger.Error(err, "Master ConfigMap Not Found!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
					return reconcile.Result{}, client.IgnoreNotFound(err)
				} else {
					r.logger.Error(err, "Failed to Get Master ConfigMap!", "name", syncer.Spec.MasterConfigMap.Name, "namespace", syncer.Spec.MasterConfigMap.Namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
					return reconcile.Result{Requeue: true}, err
				}
			}
			syncer.Status.Data = []syncerv1alpha1.SyncerData{}
			syncerData, err := r.syncConfigMap(masterConfigmap, namespace, &syncer)
			if err != nil {
				r.logger.Error(err, "Failed to sync ConfigMap to namespace!", "namespace", namespace, "syncer", syncer.Name, "syncerNamespace", syncer.Namespace)
			}
			if syncerData != nil {
				syncer.Status.Data = append(syncer.Status.Data, *syncerData)
				syncer.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
				err = r.Status().Update(r.ctx, &syncer)
				if err != nil {
					r.logger.Error(err, "Failed to update status")
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *ConfigMapSyncerReconciler) syncConfigMap(masterConfigmap *corev1.ConfigMap, targetNamespace string, syncer *syncerv1alpha1.ConfigMapSyncer) (*syncerv1alpha1.SyncerData, error) {
	data := &syncerv1alpha1.SyncerData{}
	data.Namespace = targetNamespace
	if !r.isNamespaceExists(targetNamespace) {
		r.logger.Info("namespace missing", "namespace", targetNamespace)
		data.Action = syncerv1alpha1.SyncerDataActionCreate
		data.Status = syncerv1alpha1.SyncerDataStatusFailed
		errorMessage := fmt.Sprintf("namespace %s does not exists", targetNamespace)
		data.Error = &errorMessage
		return data, errors.NewBadRequest(fmt.Sprintf("namespace %s does not exists", targetNamespace))
	}

	targetConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      masterConfigmap.Name,
			Namespace: targetNamespace,
		},
	}
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(targetConfigmap), targetConfigmap); err != nil && errors.IsNotFound(err) {
		// Create Configmap
		data.Action = syncerv1alpha1.SyncerDataActionCreate
		targetConfigmap.Data = masterConfigmap.Data
		targetConfigmap.BinaryData = masterConfigmap.BinaryData
		if err = r.Create(r.ctx, targetConfigmap); err != nil {
			r.logger.Error(err, "Failed to create target ConfigMap!", "namespace", targetNamespace)
			data.Status = syncerv1alpha1.SyncerDataStatusFailed
			errorMessage := err.Error()
			data.Error = &errorMessage
			return data, err
		} else {
			r.logger.Info("ConfigMap created!", "namespace", targetNamespace)
			data.Status = syncerv1alpha1.SyncerDataStatusSuccess
			return data, nil
		}
	} else {
		// Sync Existing ConfigMap
		updatedData, updatedBinaryData := r.syncConfigMapData(targetConfigmap, masterConfigmap, syncer.Spec.SyncStrategy)
		if !reflect.DeepEqual(targetConfigmap.Data, updatedData) || !reflect.DeepEqual(targetConfigmap.BinaryData, updatedBinaryData) {
			targetConfigmap.Data = updatedData
			targetConfigmap.BinaryData = updatedBinaryData
			data.Action = syncerv1alpha1.SyncerDataActionUpdate
			if err = r.Update(r.ctx, targetConfigmap); err != nil {
				r.logger.Error(err, "Failed to update target ConfigMap!", "name", targetConfigmap.Name, "namespace", targetNamespace)
				data.Status = syncerv1alpha1.SyncerDataStatusFailed
				errorMessage := err.Error()
				data.Error = &errorMessage
				return data, err
			} else {
				r.logger.Info("ConfigMap updated!", "namespace", targetNamespace)
				data.Status = syncerv1alpha1.SyncerDataStatusSuccess
				return data, nil
			}
		}
	}
	r.logger.Info("no changes", "namespace", targetNamespace)
	data.Status = syncerv1alpha1.SyncerDataStatusSuccess
	data.Action = syncerv1alpha1.SyncerDataActionNoChange
	return data, nil
}

func (r *ConfigMapSyncerReconciler) syncConfigMapData(targetConfigMap *corev1.ConfigMap, masterConfigmap *corev1.ConfigMap, syncStrategy string) (map[string]string, map[string][]byte) {
	updatedData := make(map[string]string)
	updatedBinaryData := make(map[string][]byte)

	switch syncStrategy {
	case syncerv1alpha1.ConfigMapSyncStrategyOverwrite:
		return masterConfigmap.Data, masterConfigmap.BinaryData

	case syncerv1alpha1.ConfigMapSyncStrategyAppend:
		if targetConfigMap.Data != nil {
			for key, val := range targetConfigMap.Data {
				updatedData[key] = val
			}
		}

		if masterConfigmap.Data != nil {
			for key, val := range masterConfigmap.Data {
				updatedData[key] = val
			}
		}

		if targetConfigMap.BinaryData != nil {
			for key, val := range targetConfigMap.BinaryData {
				updatedBinaryData[key] = val
			}
		}

		if masterConfigmap.BinaryData != nil {
			for key, val := range masterConfigmap.BinaryData {
				updatedBinaryData[key] = val
			}
		}

		return updatedData, updatedBinaryData

	case syncerv1alpha1.ConfigMapSyncStrategySelectiveUpdate:
		updatedData = targetConfigMap.Data
		updatedBinaryData = targetConfigMap.BinaryData

		if masterConfigmap.Data != nil {
			for key, val := range masterConfigmap.Data {
				if _, exists := targetConfigMap.Data[key]; exists {
					updatedData[key] = val
				}
			}
		}

		if masterConfigmap.BinaryData != nil {
			for key, val := range masterConfigmap.BinaryData {
				if _, exists := targetConfigMap.BinaryData[key]; exists {
					updatedBinaryData[key] = val
				}
			}
		}

		return updatedData, updatedBinaryData

	default:
		return masterConfigmap.Data, masterConfigmap.BinaryData
	}
}

func (r *ConfigMapSyncerReconciler) isNamespaceExists(namespace string) bool {
	ns := &corev1.Namespace{}
	err := r.Get(r.ctx, types.NamespacedName{Name: namespace}, ns)
	if err == nil {
		return true
	}
	return false
}

//func (r *ConfigMapSyncerReconciler) isNamespaceExists(namespace string) bool {
//	counter := 0
//	for {
//		if counter > 3 {
//			break
//		}
//		ns := &corev1.Namespace{}
//		err := r.Get(r.ctx, types.NamespacedName{Name: namespace}, ns)
//		if err == nil {
//			return true
//		}
//		counter++
//		time.Sleep(2 * time.Second)
//	}
//	return false
//}
