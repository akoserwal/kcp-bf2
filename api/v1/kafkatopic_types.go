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

// KafkaTopicSpec defines the desired state of KafkaInstance `
type KafkaTopicSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Partitions int32 `json:"partitions,omitempty"`

	// +kubebuilder:validation:Required
	TopicName string `json:"topicName"`

	Config map[string]string `json:"config,omitempty"`
}

// KafkaTopicPhase is a valid value for KafkaTopicStatus.Phase
type KafkaTopicPhase string

// KafkaTopicStatus defines the observed state of a topic
type KafkaTopicStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Message string          `json:"message,omitempty"`
	Phase   KafkaTopicPhase `json:"phase,omitempty"`
}

// These are valid phases of a Pod
const (
	KafkaTopicUnknown KafkaTopicPhase = "Unknown"
	KafkaTopicReady   KafkaTopicPhase = "Ready"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="KafkaInstance",type=string,JSONPath=`.metadata.labels['kafka\.pmuir\/kafka-instance']`
// +kubebuilder:printcolumn:name="Partitions",type=string,JSONPath=`.spec.partitions`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// KafkaTopic is the Schema for the kafkatopic API
type KafkaTopic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaTopicSpec   `json:"spec,omitempty"`
	Status KafkaTopicStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KafkaTopicList contains a list of KafkaTopic
type KafkaTopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaTopic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaTopic{}, &KafkaTopicList{})
}
