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

package serviceaccount

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	kafkamgmtclient "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/client"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pmuir/kcp-bf2/controllers/utils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kafkav1 "pmuir/kcp-bf2/api/v1"
)

type ServiceAccountCredentials struct {
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// ServiceAccountReconciler reconciles a KafkaInstance object
type ServiceAccountReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances/finalizers,verbs=update
func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var serviceAccount kafkav1.CloudServiceAccount

	log := log.FromContext(ctx)

	offlineToken, err := utils.LoadOfflineToken(r.Client, ctx, req)
	if err != nil {
		apiErr := utils.GetAPIError(err)
		serviceAccount.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
		serviceAccount.Status.Phase = kafkav1.ServiceAccountUnknown
		if err := r.Status().Update(ctx, &serviceAccount); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		return ctrl.Result{}, nil
	}

	if err := r.Get(ctx, req.NamespacedName, &serviceAccount); err != nil {
		// Ignore not-found errors, they have to be dealt with by the finalizer
		return ctrl.Result{}, nil
	}

	// If the instance isn't properly created, we can just allow it to be removed
	if (serviceAccount.Status.Phase != kafkav1.ServiceAccountUnknown && serviceAccount.Status.Phase != "") && serviceAccount.Status.Id != "" {

		finalizerName := "service-account.kafka.pmuir/finalizer"

		if serviceAccount.ObjectMeta.DeletionTimestamp.IsZero() {
			if !utils.ContainsString(serviceAccount.GetFinalizers(), finalizerName) {
				log.Info("adding finalizer")
				controllerutil.AddFinalizer(&serviceAccount, finalizerName)

				if err := r.Update(ctx, &serviceAccount); err != nil {
					return ctrl.Result{}, errors.WithStack(err)
				}
			}
		} else {
			// The object is being deleted
			if utils.ContainsString(serviceAccount.GetFinalizers(), finalizerName) {
				// our finalizer is present, so lets handle any external dependency

				// Check state from the API
				c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)
				if _, res, err := c.SecurityApi.GetServiceAccountById(ctx, serviceAccount.Status.Id).Execute(); err != nil {
					if res.StatusCode == 404 {
						// The remote resource is now deleted so we can remove the resource
						// remove our finalizer from the list and update it.
						log.Info("deletion complete, removing resource")
						controllerutil.RemoveFinalizer(&serviceAccount, finalizerName)
						if err := r.Update(ctx, &serviceAccount); err != nil {
							return ctrl.Result{}, errors.WithStack(err)
						}
						// Stop reconcilation
						return ctrl.Result{}, nil
					}
					return ctrl.Result{}, err
				} else {
					if err := r.deleteServiceAccount(ctx, &serviceAccount, offlineToken); err != nil {
						// if fail to delete the external dependency here, return with error
						// so that it can be retried
						return ctrl.Result{}, errors.WithStack(err)
					}
				}
			}
			log.Info("deletion pending")
			return ctrl.Result{
				RequeueAfter: utils.WatchInterval,
			}, nil
		}
	}

	answer, err := r.createServiceAccount(ctx, &serviceAccount, offlineToken)
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

func (r *ServiceAccountReconciler) createServiceAccount(ctx context.Context, serviceAccount *kafkav1.CloudServiceAccount, offlineToken string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// try to find the Kafka, only consider those not created externally (as there is a gap whilst they have their Id set)
	annotations := serviceAccount.ObjectMeta.GetAnnotations()
	if serviceAccount.Status.Id != "" || serviceAccount.Status.Phase != "" {
		return ctrl.Result{}, nil
	}
	if annotations != nil {
		if annotations["kafka.pmuir/created-externally"] == "true" {
			return ctrl.Result{}, nil
		}
	}
	c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)
	// This means it's a new CloudServiceAccount
	payload := kafkamgmtclient.ServiceAccountRequest{
		Name: serviceAccount.Spec.Name,
	}
	log.Info("creating CloudServiceAccount using kas-fleet-manager", "RequestPayload", payload, "CloudServiceAccount", serviceAccount)
	serviceAccountRequest, _, err := c.SecurityApi.CreateServiceAccount(ctx).ServiceAccountRequest(payload).Execute()
	if err != nil {
		apiErr := utils.GetAPIError(err)
		serviceAccount.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
		serviceAccount.Status.Phase = kafkav1.ServiceAccountUnknown
		log.Info("Error creating ServiceAcount using kas-fleet-manager", "Message", serviceAccount.Status.Message, "CloudServiceAccount", serviceAccount)
		if err := r.Status().Update(ctx, serviceAccount); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		return ctrl.Result{}, nil
	}
	// Create a secret containing the credentials
	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      serviceAccount.ObjectMeta.Name,
			Namespace: serviceAccount.ObjectMeta.Namespace,
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: serviceAccount.TypeMeta.APIVersion,
					Kind:       serviceAccount.TypeMeta.Kind,
					Name:       serviceAccount.ObjectMeta.Name,
					UID:        serviceAccount.ObjectMeta.UID,
				},
			},
		},
		Data: map[string][]byte{
			"clientId":     []byte(serviceAccountRequest.GetClientId()),
			"clientSecret": []byte(serviceAccountRequest.GetClientSecret()),
		},
	}
	if err := r.Create(ctx, &secret); err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}
	serviceAccount.Status.CredentialsSecretName = secret.ObjectMeta.Name
	return ctrl.Result{}, utils.UpdateServiceAccountStatus(r.Client, ctx, serviceAccountRequest, serviceAccount)
}

func (r *ServiceAccountReconciler) deleteServiceAccount(ctx context.Context, serviceAccount *kafkav1.CloudServiceAccount, offlineToken string) error {
	log := log.FromContext(ctx)
	// try to find the Service Account
	if serviceAccount.Status.Id != "" {
		c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)
		log.Info("Deleting CloudServiceAccount using kas-fleet-manager", "CloudServiceAccount", serviceAccount)
		_, _, err := c.SecurityApi.DeleteServiceAccountById(ctx, serviceAccount.Status.Id).Execute()
		if err != nil {
			apiErr := utils.GetAPIError(err)
			serviceAccount.Status.Message = fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason())
			serviceAccount.Status.Phase = kafkav1.ServiceAccountUnknown
			log.Info("Error deleting CloudServiceAccount using kas-fleet-manager", "Message", serviceAccount.Status.Message, "CloudServiceAccount", serviceAccount)
			if err := r.Status().Update(ctx, serviceAccount); err != nil {
				return errors.WithStack(err)
			}
			return nil
		}
		return nil
	}
	return errors.New(fmt.Sprintf("service account instance has no Id. Service Aount instance status: %v", serviceAccount.Status))
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1.CloudServiceAccount{}).
		Complete(r)
}
