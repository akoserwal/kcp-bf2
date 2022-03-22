/*
Copyright 2021.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KafkaInstanceSpec defines the desired state of KafkaInstance `
type KafkaInstanceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	CloudProvider           string `json:"cloudProvider,omitempty"`
	Name                    string `json:"name,omitempty"`
	ReauthenticationEnabled *bool  `json:"reauthenticationEnabled,omitempty"`
	Region                  string `json:"region,omitempty"`
	MultiAz                 *bool  `json:"multiAz,omitempty"`
}

// KafkaInstanceStatus defines the observed state of KafkaInstance
type KafkaInstanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	CreatedAt           metav1.Time `json:"region,omitempty"`
	Href                string      `json:"href,omitempty"`
	Id                  string      `json:"id,omitempty"`
	InstanceType        string      `json:"instanceType,omitempty"`
	Kind                string      `json:"kind,omitempty"`
	Owner               string      `json:"owner,omitempty"`
	Message             string      `json:"message,omitempty"`
	UpdatedAt           metav1.Time `json:"updatedAt,omitempty"`
	Version             string      `json:"version,omitempty"`
	BootstrapServerHost string      `json:"bootstrapServerHost,omitempty"`
	Phase               KafkaPhase  `json:"phase,omitempty"`
}

// KafkaPhase is a valid value for KafkaInstanceStatus.Phase
type KafkaPhase string

// These are valid phases of a Pod
const (
	KafkaUnknown      KafkaPhase = "Unknown"
	KafkaReady        KafkaPhase = "Ready"
	KafkaFailed       KafkaPhase = "Failed"
	KafkaAccepted     KafkaPhase = "Accepted"
	KafkaPreparing    KafkaPhase = "Preparing"
	KafkaProvisioning KafkaPhase = "Provisioning"
	KafkaDeleting     KafkaPhase = "Deleting"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Region",type=string,JSONPath=`.spec.region`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="BootstrapServerHost",type=string,JSONPath=`.status.bootstrapServerHost`

// KafkaInstance is the Schema for the kafkainstances API
type KafkaInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaInstanceSpec   `json:"spec,omitempty"`
	Status KafkaInstanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KafkaInstanceList contains a list of KafkaInstance
type KafkaInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaInstance{}, &KafkaInstanceList{})
}
