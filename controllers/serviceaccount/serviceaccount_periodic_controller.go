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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kafkav1 "pmuir/kcp-bf2/api/v1"
	"pmuir/kcp-bf2/controllers/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceAccountPeriodicReconciler reconciles a CloudServiceAccount object
type ServiceAccountPeriodicReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kafka.pmuir,resources=kafkainstances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CloudServiceAccount object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ServiceAccountPeriodicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

func (r *ServiceAccountPeriodicReconciler) periodicReconcile(ctx context.Context, req ctrl.Request, offlineToken string) error {

	log := log.FromContext(ctx)
	c := utils.BuildKasAPIClient(offlineToken, utils.DefaultClientID, utils.DefaultAuthURL, utils.DefaultAPIURL)
	serviceAccountList, _, err := c.SecurityApi.GetServiceAccounts(ctx).Execute()
	if err != nil {
		apiErr := utils.GetAPIError(err)
		return errors.Wrapf(err, fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason()))
	}
	var serviceAcounts kafkav1.CloudServiceAccountList
	if err := r.List(ctx, &serviceAcounts, client.InNamespace(req.Namespace)); err != nil {
		return errors.WithStack(err)
	}
	seenServiceAccounts := make(map[string]bool, 0)
	serviceAccountsById := make(map[string]kafkav1.CloudServiceAccount)
	for _, serviceAccount := range serviceAcounts.Items {
		seenServiceAccounts[serviceAccount.Status.Id] = false
		if serviceAccount.Status.Id == "" && serviceAccount.Status.Phase == kafkav1.ServiceAccountReady {
			serviceAccount.Status.Message = "Unable to update as no ID available"
			serviceAccount.Status.Phase = kafkav1.ServiceAccountUnknown
			log.Info("unable to update", "CloudServiceAccount", serviceAccount)
			err := r.Status().Update(ctx, &serviceAccount)
			if err != nil {
				return errors.WithStack(err)
			}
		} else {
			serviceAccountsById[serviceAccount.Status.Id] = serviceAccount
		}
	}
	for _, serviceAccount := range serviceAccountList.GetItems() {
		seenServiceAccounts[serviceAccount.GetId()] = true
		serviceAccountById, ok := serviceAccountsById[serviceAccount.GetId()]
		if !ok {
			// This means it's a new kafka instance that's appeared on the API so we need to create an entry in KCP
			utils.ConvertToServiceAccount(serviceAccount, req.Namespace, &serviceAccountById)
			// annotate the object as externally created
			annotations := make(map[string]string)
			annotations["kafka.pmuir/created-externally"] = "true"
			serviceAccountById.ObjectMeta.SetAnnotations(annotations)
			err := r.Create(ctx, &serviceAccountById)
			if err != nil {
				return errors.WithStack(err)
			}
			log.Info("adding service account resource", "CloudServiceAccount", serviceAccountById)
			err = utils.UpdateServiceAccountListItemStatus(r.Client, ctx, serviceAccount, &serviceAccountById)
			if err != nil {
				return errors.WithStack(err)
			}
		} else {
			// Existing service account instance
			err := utils.UpdateServiceAccountListItemStatus(r.Client, ctx, serviceAccount, &serviceAccountById)
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}
	for id, seen := range seenServiceAccounts {
		if id != "" && !seen {
			// try to find the service account by id
			serviceAccount, ok := serviceAccountsById[id]
			if !ok {
				return errors.New(fmt.Sprintf("service account with id %s not found", id))
			} else {
				// Existing kafka instance that is no longer in the api
				log.Info("deleting service account CR", "id", serviceAccount.Status.Id)
				err := r.Delete(ctx, &serviceAccount)
				if err != nil {
					return errors.WithStack(err)
				}
			}
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountPeriodicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
