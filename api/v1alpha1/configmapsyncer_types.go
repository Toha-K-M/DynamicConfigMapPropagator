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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConfigMapSyncerStateProgressing = "Progressing"
	ConfigMapSyncerStateSucceeded   = "Succeeded"
	ConfigMapSyncerStateFailed      = "Failed"

	ConfigMapSyncStrategyOverwrite       = "Overwrite"
	ConfigMapSyncStrategyAppend          = "Append"
	ConfigMapSyncStrategySelectiveUpdate = "Selective"

	SyncerDataActionCreate = "Create"
	SyncerDataActionUpdate = "Update"

	SyncerDataStatusSuccess = "Success"
	SyncerDataStatusFailed  = "Failed"
)

type Metadata struct {
	//+kubebuilder:validation:Required
	Name string `json:"name"`
	//+kubebuilder:default=default
	Namespace string `json:"namespace"`
	//+kubebuilder:validation:Optional
	Labels map[string]string `json:"labels"`
}

// ConfigMapSyncerSpec defines the desired state of ConfigMapSyncer
type ConfigMapSyncerSpec struct {
	MasterConfigMap Metadata `json:"masterConfigMap,omitempty"`
	//+kubebuilder:validation:Required
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`
	//+kubebuilder:default=Overwrite
	SyncStrategy string `json:"syncStrategy,omitempty"`
}

type SyncerData struct {
	Namespace string  `json:"namespace,omitempty"`
	Action    string  `json:"action,omitempty"`
	Status    string  `json:"status,omitempty"`
	Error     *string `json:"error,omitempty"`
}

// ConfigMapSyncerStatus defines the observed state of ConfigMapSyncer
type ConfigMapSyncerStatus struct {
	LastUpdateTime string       `json:"lastUpdateTime,omitempty"`
	Data           []SyncerData `json:"data,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ConfigMapSyncer is the Schema for the configmapsyncers API
type ConfigMapSyncer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigMapSyncerSpec   `json:"spec,omitempty"`
	Status ConfigMapSyncerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigMapSyncerList contains a list of ConfigMapSyncer
type ConfigMapSyncerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigMapSyncer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigMapSyncer{}, &ConfigMapSyncerList{})
}
