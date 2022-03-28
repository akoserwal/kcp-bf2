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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kafkav1 "pmuir/kcp-bf2/api/v1"
	"pmuir/kcp-bf2/controllers/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// KafkaTopicPeriodicReconciler reconciles a KafkaTopic object
type KafkaTopicPeriodicReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkaTopics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkaTopics/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkaTopics/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KafkaTopic object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *KafkaTopicPeriodicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// This is a bit of a hack to set up a periodic reconcile
	if req.Name == "sso.redhat.com" {
		offlineToken, err := utils.LoadOfflineToken(r.Client, ctx, req)
		if err != nil {
			apiErr := utils.GetAPIError(err)
			return ctrl.Result{}, errors.New(fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason()))
		}
		err = r.periodicReconcile(ctx, req, offlineToken)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		return ctrl.Result{
			RequeueAfter: utils.WatchInterval,
		}, nil
	}
	return ctrl.Result{}, nil
}

func (r *KafkaTopicPeriodicReconciler) periodicReconcile(ctx context.Context, req ctrl.Request, offlineToken string) error {

	log := log.FromContext(ctx)
	c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)

	var kafkaTopics kafkav1.KafkaTopicList

	if err := r.List(ctx, &kafkaTopics, client.InNamespace(req.Namespace)); err != nil {
		return errors.WithStack(err)
	}

	kafkaTopicsByName := make(map[string]kafkav1.KafkaTopic)
	seenKafkaTopicNames := make(map[string]bool, 0)
	for _, kafkaTopic := range kafkaTopics.Items {
		seenKafkaTopicNames[kafkaTopic.Name] = false
		kafkaTopicsByName[kafkaTopic.Name] = kafkaTopic
	}

	var kafkaInstances kafkav1.KafkaInstanceList

	if err := r.List(ctx, &kafkaInstances, client.InNamespace(req.Namespace)); err != nil {
		return errors.WithStack(err)
	}

	for _, kafkaInstance := range kafkaInstances.Items {

		clientId, clientSecret, err := utils.GetOrCreateServiceAccount(ctx, r.Client, c, kafkaInstance.Name, kafkaInstance.Namespace)
		if err != nil {
			return errors.WithStack(err)
		}
		kafkaAdmin, err := utils.CreateKafkaAdmin(ctx, kafkaInstance.Status.BootstrapServerHost, clientId, clientSecret)
		if err != nil {
			return errors.WithStack(err)
		}
		kafkaTopicRequests, _, err := kafkaAdmin.TopicsApi.GetTopics(ctx).Execute()
		if err != nil {
			return errors.WithStack(err)
		}
		for _, detail := range kafkaTopicRequests.Items {
			seenKafkaTopicNames[detail.GetName()] = true
			kafkaTopic, ok := kafkaTopicsByName[detail.GetName()]
			if !ok {
				// This means its a new topic that has appeared on the API so we need to create an entry in KCP
				// annotate the object as externally created
				utils.ConvertToKafkaTopic(detail, req.Namespace, &kafkaTopic)
				annotations := make(map[string]string)
				annotations["kafka.pmuir/created-externally"] = "true"
				kafkaTopic.ObjectMeta.SetAnnotations(annotations)
				if kafkaTopic.ObjectMeta.Labels == nil {
					kafkaTopic.ObjectMeta.Labels = make(map[string]string)
				}
				kafkaTopic.ObjectMeta.Labels["kafka.pmuir/kafka-instance"] = kafkaInstance.Name
				kafkaTopic.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: kafkaInstance.APIVersion,
						Kind:       kafkaInstance.Kind,
						Name:       kafkaInstance.Name,
						UID:        kafkaInstance.UID,
					},
				}
				err := r.Create(ctx, &kafkaTopic)
				if err != nil {
					return errors.WithStack(err)
				}
				log.Info("adding kafka topic resource", "KafkaTopic", kafkaTopic)
				if err := r.Get(ctx, client.ObjectKeyFromObject(&kafkaTopic), &kafkaTopic); err != nil {
					return errors.WithStack(err)
				}
				err = utils.UpdateKafkaTopicStatus(r.Client, ctx, detail, &kafkaTopic)
				if err != nil {
					return errors.WithStack(err)
				}
			} else {
				// Existing kafka topic
				err := utils.UpdateKafkaTopicStatus(r.Client, ctx, detail, &kafkaTopic)
				if err != nil {
					return errors.WithStack(err)
				}
			}
		}

		for name, seen := range seenKafkaTopicNames {
			if name != "" && !seen {
				// try to find the kafka topic by name
				kafkaTopic, ok := kafkaTopicsByName[name]
				if !ok {
					return errors.New(fmt.Sprintf("Kafka topic with name %s not found", name))
				} else if kafkaTopic.Status.Phase != kafkav1.KafkaTopicUnknown && kafkaTopic.Status.Phase != "" {
					// Existing kafka topic that is no longer in the api
					log.Info("deleting kafka topic CR", "name", kafkaTopic.Name, "phase", kafkaTopic.Status.Phase)
					err := r.Delete(ctx, &kafkaTopic)
					if err != nil {
						return errors.WithStack(err)
					}
				}
			}
		}
		return nil

	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaTopicPeriodicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
