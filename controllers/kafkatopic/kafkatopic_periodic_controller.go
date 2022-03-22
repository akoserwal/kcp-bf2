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
	"encoding/base64"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
	kafkamgmtclient "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/client"
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

	var kafkaInstances kafkav1.KafkaInstanceList

	if err := r.List(ctx, &kafkaInstances, client.InNamespace(req.Namespace)); err != nil {
		return errors.WithStack(err)
	}

	for _, kafkaInstance := range kafkaInstances.Items {
		// Make sure we have a service account - this is a huge hack
		serviceAccountName := client.ObjectKey{
			Namespace: req.Namespace,
			Name:      kafkaInstance.Name,
		}

		var serviceAccountSecret corev1.Secret
		if err := r.Get(ctx, serviceAccountName, &serviceAccountSecret); err != nil {
			// means a service account for the reconciler doesn't exist so try to create one
			apiName := fmt.Sprintf("kcp-%s", serviceAccountName.Name)
			serviceAccountReq, _, err := c.SecurityApi.CreateServiceAccount(ctx).ServiceAccountRequest(kafkamgmtclient.ServiceAccountRequest{Name: apiName}).Execute()
			if err != nil {
				apiErr := utils.GetAPIError(err)
				return errors.Wrapf(err, fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason()))
			}
			var clientId []byte
			var clientSecret []byte
			base64.StdEncoding.Encode([]byte(serviceAccountReq.GetClientId()), clientId)
			base64.StdEncoding.Encode([]byte(serviceAccountReq.GetClientSecret()), clientSecret)

			serviceAccountSecret = corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      serviceAccountName.Name,
					Namespace: serviceAccountName.Namespace,
				},
				Data: map[string][]byte{
					"ClientId":     clientId,
					"ClientSecret": clientSecret,
				},
			}
			err = r.Create(ctx, &serviceAccountSecret)
			if err != nil {
				return errors.WithStack(err)
			}
			// Big problem, service accounts by default don't have full access to the data plane by default, so the user has to go in manually and grant it
			log.Info("service Account created", "name", apiName)
			log.Info("you must grant this service account admin access to the Kafka instance for full management")
		}

		kafkaAdmin, err := r.createKafkaAdmin(kafkaInstance.Status.BootstrapServerHost, string(serviceAccountSecret.Data["clientId"]), string(serviceAccountSecret.Data["clientSecret"]))
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (r *KafkaTopicPeriodicReconciler) createKafkaAdmin(bootstrapServerHost string, clientId string, clientSecret string) (sarama.ClusterAdmin, error) {
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	config.Net.SASL.User = clientId
	config.Net.SASL.Password = clientSecret
	admin, err := sarama.NewClusterAdmin([]string{bootstrapServerHost}, config)
	if err != nil {
		return nil, errors.Wrapf(err, "Error creating the Sarama cluster admin")
	}
	return admin, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaTopicPeriodicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
