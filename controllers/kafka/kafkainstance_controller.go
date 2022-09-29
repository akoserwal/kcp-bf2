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

package kafka

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	kafkamgmtclient "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/client"
	"pmuir/kcp-bf2/controllers/utils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kafkav1 "pmuir/kcp-bf2/api/v1"
)

// KafkaInstanceReconciler reconciles a KafkaInstance object
type KafkaInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances/finalizers,verbs=update
func (r *KafkaInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var kafkaInstance kafkav1.KafkaInstance

	log := log.FromContext(ctx)

	offlineToken, err := utils.LoadOfflineToken(r.Client, ctx, req)
	if err != nil {
		apiErr := utils.GetAPIError(err)
		kafkaInstance.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
		kafkaInstance.Status.Phase = kafkav1.KafkaUnknown
		if err := r.Status().Update(ctx, &kafkaInstance); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		return ctrl.Result{}, nil
	}

	if err := r.Get(ctx, req.NamespacedName, &kafkaInstance); err != nil {
		// Ignore not-found errors, they have to be dealt with by the finalizer
		return ctrl.Result{}, nil
	}

	// If the instance isn't properly created, we can just allow it to be removed
	if (kafkaInstance.Status.Phase != kafkav1.KafkaUnknown && kafkaInstance.Status.Phase != "") && kafkaInstance.Status.Id != "" {

		finalizerName := "kafka-instance.kafka.pmuir/finalizer"

		if kafkaInstance.ObjectMeta.DeletionTimestamp.IsZero() {
			if !utils.ContainsString(kafkaInstance.GetFinalizers(), finalizerName) {
				log.Info("adding finalizer")
				controllerutil.AddFinalizer(&kafkaInstance, finalizerName)

				if err := r.Update(ctx, &kafkaInstance); err != nil {
					return ctrl.Result{}, errors.WithStack(err)
				}
			}
		} else {
			// The object is being deleted
			if utils.ContainsString(kafkaInstance.GetFinalizers(), finalizerName) {
				// our finalizer is present, so lets handle any external dependency

				// Check state from the API
				c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)
				if kafkaRequest, res, err := c.DefaultApi.GetKafkaById(ctx, kafkaInstance.Status.Id).Execute(); err != nil {
					if res.StatusCode == 404 {
						// The remote resource is now deleted so we can remove the resource
						// remove our finalizer from the list and update it.
						log.Info("deletion complete, removing resource")
						controllerutil.RemoveFinalizer(&kafkaInstance, finalizerName)
						if err := r.Update(ctx, &kafkaInstance); err != nil {
							return ctrl.Result{}, errors.WithStack(err)
						}
						// Stop reconcilation
						return ctrl.Result{}, nil
					}
					return ctrl.Result{}, err
				} else {
					if kafkaRequest.Status != nil && *kafkaRequest.Status != "deleting" && *kafkaRequest.Status != "deprovision" {
						if err := r.deleteKafkaInstance(ctx, &kafkaInstance, offlineToken); err != nil {
							// if fail to delete the external dependency here, return with error
							// so that it can be retried
							return ctrl.Result{}, errors.WithStack(err)
						}
					}
				}
			}
			log.Info("deletion pending")
			return ctrl.Result{
				RequeueAfter: utils.WatchInterval,
			}, nil
		}
	}

	answer, err := r.createKafkaInstance(ctx, &kafkaInstance, offlineToken)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	return answer, nil
}

func stringToPointer(str string) *string {
	if str == "" {
		return nil
	}
	return &str
}

func (r *KafkaInstanceReconciler) createKafkaInstance(ctx context.Context, kafkaInstance *kafkav1.KafkaInstance, offlineToken string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// try to find the Kafka, only consider those not created externally (as there is a gap whilst they have their Id set)
	annotations := kafkaInstance.ObjectMeta.GetAnnotations()
	if kafkaInstance.Status.Id != "" || kafkaInstance.Status.Phase != "" {
		return ctrl.Result{}, nil
	}
	if annotations != nil {
		if annotations["kafka.pmuir/created-externally"] == "true" {
			return ctrl.Result{}, nil
		}
	}
	c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)
	// This means it's a new Kafka
	payload := kafkamgmtclient.KafkaRequestPayload{
		CloudProvider:           stringToPointer(kafkaInstance.Spec.CloudProvider),
		Name:                    kafkaInstance.Spec.Name,
		Region:                  stringToPointer(kafkaInstance.Spec.Region),
		ReauthenticationEnabled: *kafkamgmtclient.NewNullableBool(kafkaInstance.Spec.ReauthenticationEnabled),
	}
	log.Info("creating Kafka using kas-fleet-manager", "RequestPayload", payload, "KafkaInstance", kafkaInstance)
	kafkaRequest, _, err := c.DefaultApi.CreateKafka(ctx).KafkaRequestPayload(payload).Async(true).Execute()

	if err != nil {
		apiErr := utils.GetAPIError(err)
		kafkaInstance.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
		kafkaInstance.Status.Phase = kafkav1.KafkaUnknown
		log.Info("Error creating Kafka using kas-fleet-manager", "Message", kafkaInstance.Status.Message, "KafkaInstance", kafkaInstance)
		if err := r.Status().Update(ctx, kafkaInstance); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, utils.UpdateKafkaInstanceStatus(r.Client, ctx, kafkaRequest, kafkaInstance)
}

func (r *KafkaInstanceReconciler) deleteKafkaInstance(ctx context.Context, kafkaInstance *kafkav1.KafkaInstance, offlineToken string) error {
	log := log.FromContext(ctx)
	// try to find the Kafka
	if kafkaInstance.Status.Id != "" {
		c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)
		log.Info("Deleting Kafka using kas-fleet-manager", "KafkaInstance", kafkaInstance)
		_, _, err := c.DefaultApi.DeleteKafkaById(ctx, kafkaInstance.Status.Id).Async(true).Execute()
		if err != nil {
			apiErr := utils.GetAPIError(err)
			kafkaInstance.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
			kafkaInstance.Status.Phase = kafkav1.KafkaUnknown
			log.Info("Error deleting Kafka using kas-fleet-manager", "Message", kafkaInstance.Status.Message, "KafkaInstance", kafkaInstance)
			if err := r.Status().Update(ctx, kafkaInstance); err != nil {
				return errors.WithStack(err)
			}
			return nil
		}
		return nil
	}
	return errors.New(fmt.Sprintf("kafka instance has no Id. Kafka instance status: %v", kafkaInstance.Status))
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1.KafkaInstance{}).
		Complete(r)
}
