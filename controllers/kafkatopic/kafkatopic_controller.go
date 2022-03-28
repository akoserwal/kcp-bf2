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

package kafkatopic

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	kafkainstanceclient "github.com/redhat-developer/app-services-sdk-go/kafkainstance/apiv1internal/client"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"pmuir/kcp-bf2/controllers/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kafkav1 "pmuir/kcp-bf2/api/v1"
)

// KafkaTopicReconciler reconciles a KafkaTopic object
type KafkaTopicReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kafka.pmuir,resources=KafkaTopics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.pmuir,resources=KafkaTopics/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kafka.pmuir,resources=KafkaTopics/finalizers,verbs=update
func (r *KafkaTopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var kafkaTopic kafkav1.KafkaTopic

	offlineToken, err := utils.LoadOfflineToken(r.Client, ctx, req)

	log := log.FromContext(ctx)

	if err := r.Get(ctx, req.NamespacedName, &kafkaTopic); err != nil {
		// Ignore not-found errors, they have to be dealt with by the finalizer
		return ctrl.Result{}, nil
	}

	kafkaAdmin, err := r.buildKafkaAdmin(ctx, offlineToken, &kafkaTopic)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	if kafkaAdmin == nil {
		return ctrl.Result{}, nil
	}

	if (kafkaTopic.Status.Phase != kafkav1.KafkaTopicUnknown && kafkaTopic.Status.Phase != "") && kafkaTopic.Name != "" {

		if !kafkaTopic.ObjectMeta.DeletionTimestamp.IsZero() {
			// The object is being deleted
			_, err := kafkaAdmin.TopicsApi.DeleteTopic(ctx, kafkaTopic.Name).Execute()
			if err != nil {
				apiErr := utils.GetAPIError(err)
				return ctrl.Result{}, errors.Wrapf(err, fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason()))
			}
			log.Info("topic deleted")
			return ctrl.Result{}, nil
		}
	}

	answer, err := r.createKafkaTopic(ctx, &kafkaTopic, kafkaAdmin)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return answer, nil
}

func (r *KafkaTopicReconciler) createKafkaTopic(ctx context.Context, kafkaTopic *kafkav1.KafkaTopic, kafkaAdmin *kafkainstanceclient.APIClient) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// try to find the Kafka, only consider those not created externally (as there is a gap whilst they have their Id set)
	annotations := kafkaTopic.ObjectMeta.GetAnnotations()
	log.Info("about to create topic", "name", kafkaTopic.Name, "phase", kafkaTopic.Status.Phase)
	if kafkaTopic.Name == "" || kafkaTopic.Status.Phase != "" {
		return ctrl.Result{}, nil
	}
	if annotations != nil {
		if annotations["kafka.pmuir/created-externally"] == "true" {
			return ctrl.Result{}, nil
		}
	}
	// This means it's a new Kafka
	payload := utils.ConvertToNewTopicInput(kafkaTopic)
	log.Info("creating Kafka Topic using kafka admin api", "RequestPayload", payload, "kafkaTopic", kafkaTopic)
	topic, _, err := kafkaAdmin.TopicsApi.CreateTopic(ctx).NewTopicInput(payload).Execute()
	if err != nil {
		apiErr := utils.GetAPIError(err)
		kafkaTopic.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
		kafkaTopic.Status.Phase = kafkav1.KafkaTopicUnknown
		log.Info("Error creating Kafka Topic using kafka admin api", "Message", kafkaTopic.Status.Message, "kafkaTopic", kafkaTopic)
		if err := r.Status().Update(ctx, kafkaTopic); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, utils.UpdateKafkaTopicStatus(r.Client, ctx, topic, kafkaTopic)
}

func (r *KafkaTopicReconciler) deleteKafkaTopic(ctx context.Context, kafkaTopic *kafkav1.KafkaTopic, offlineToken string) error {
	log := log.FromContext(ctx)
	// try to find the Kafka
	if kafkaTopic.Name != "" {
		kafkaAdmin, err := r.buildKafkaAdmin(ctx, offlineToken, kafkaTopic)
		log.Info("Deleting Kafka Topic using kafka admin api", "kafkaTopic", kafkaTopic)
		_, err = kafkaAdmin.TopicsApi.DeleteTopic(ctx, kafkaTopic.Name).Execute()
		if err != nil {
			apiErr := utils.GetAPIError(err)
			kafkaTopic.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
			kafkaTopic.Status.Phase = kafkav1.KafkaTopicUnknown
			log.Info("Error deleting Kafka using kafka admin api", "Message", kafkaTopic.Status.Message, "kafkaTopic", kafkaTopic)
			if err := r.Status().Update(ctx, kafkaTopic); err != nil {
				return errors.WithStack(err)
			}
			return nil
		}
		return nil
	}
	return errors.New(fmt.Sprintf("kafka instance has no Id. Kafka instance status: %v", kafkaTopic.Status))
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaTopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1.KafkaTopic{}).
		Complete(r)
}

func (r *KafkaTopicReconciler) buildKafkaAdmin(ctx context.Context, offlineToken string, kafkaTopic *kafkav1.KafkaTopic) (*kafkainstanceclient.APIClient, error) {

	log := log.FromContext(ctx)

	if name, ok := kafkaTopic.ObjectMeta.Labels["kafka.pmuir/kafka-instance"]; !ok {
		kafkaTopic.Status.Phase = kafkav1.KafkaTopicReady
		kafkaTopic.Status.Message = "label kafka.pmuir/kafka-instance must be present on the resource"
		err := r.Status().Update(ctx, kafkaTopic)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return nil, nil
	} else {

		var kafkaInstance kafkav1.KafkaInstance

		if err := r.Get(ctx, types.NamespacedName{
			Namespace: kafkaTopic.Namespace,
			Name:      name,
		}, &kafkaInstance); err != nil {
			// Ignore not-found errors, they have to be dealt with by the finalizer
			return nil, nil
		}

		c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)

		clientId, clientSecret, err := utils.GetOrCreateServiceAccount(ctx, r.Client, c, kafkaInstance.Name, kafkaInstance.Namespace)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		kafkaAdmin, err := utils.CreateKafkaAdmin(ctx, kafkaInstance.Status.BootstrapServerHost, clientId, clientSecret)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		log.Info("created kafka admin")

		if len(kafkaTopic.OwnerReferences) != 1 {
			controller := true
			kafkaTopic.ObjectMeta.OwnerReferences = []v1.OwnerReference{
				{
					APIVersion: kafkaInstance.APIVersion,
					Kind:       kafkaInstance.Kind,
					Name:       kafkaInstance.Name,
					UID:        kafkaInstance.UID,
					Controller: &controller,
				},
			}
			err = r.Client.Update(ctx, kafkaTopic)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}

		return kafkaAdmin, nil
	}
}
