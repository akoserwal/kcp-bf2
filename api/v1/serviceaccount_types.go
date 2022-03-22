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

// CloudServiceAccountSpec defines the desired state of CloudServiceAccount `
type CloudServiceAccountSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Name string `json:"name,omitempty"`
}

// CloudServiceAccountStatus defines the observed state of CloudServiceAccount
type CloudServiceAccountStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	CreatedAt             metav1.Time              `json:"region,omitempty"`
	Href                  string                   `json:"href,omitempty"`
	Id                    string                   `json:"id,omitempty"`
	Kind                  string                   `json:"kind,omitempty"`
	Owner                 string                   `json:"owner,omitempty"`
	Phase                 CloudServiceAccountPhase `json:"phase,omitempty"`
	Message               string                   `json:"message,omitempty"`
	CredentialsSecretName string                   `json:"credentialsSecretName,omitempty"`
}

// CloudServiceAccountPhase is a valid value for CloudServiceAccountStatus.Phase
type CloudServiceAccountPhase string

// These are valid phases of a Pod
const (
	ServiceAccountUnknown CloudServiceAccountPhase = "Unknown"
	ServiceAccountReady   CloudServiceAccountPhase = "Ready"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// CloudServiceAccount is the Schema for the CloudServiceAccount API
type CloudServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudServiceAccountSpec   `json:"spec,omitempty"`
	Status CloudServiceAccountStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CloudServiceAccountList contains a list of CloudServiceAccount
type CloudServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudServiceAccount `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudServiceAccount{}, &CloudServiceAccountList{})
}
