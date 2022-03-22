package utils

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	kafkamgmtclient "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/client"
	kafkav1 "pmuir/kcp-bf2/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ConvertToServiceAccount(serviceAccountRequest kafkamgmtclient.ServiceAccountListItem, namespace string, serviceAccount *kafkav1.CloudServiceAccount) {
	ConvertToServiceAccountSpec(serviceAccountRequest, serviceAccount)
	serviceAccount.ObjectMeta.Name = fmt.Sprintf("%s-%s", EncodeKubernetesName(serviceAccountRequest.GetName(), 63-21), serviceAccountRequest.GetId())
	serviceAccount.ObjectMeta.Namespace = namespace
}

func ConvertToServiceAccountSpec(serviceAccountRequest kafkamgmtclient.ServiceAccountListItem, serviceAccount *kafkav1.CloudServiceAccount) {
	serviceAccount.Spec.Name = serviceAccountRequest.GetName()
}

func ConvertToServiceAccountStatus(serviceAccountRequest kafkamgmtclient.ServiceAccount, serviceAccount *kafkav1.CloudServiceAccount) {
	// Otherwise, we need to update the Kafka status
	serviceAccount.Status.CreatedAt.Time = serviceAccountRequest.GetCreatedAt()
	serviceAccount.Status.Href = serviceAccountRequest.GetHref()
	serviceAccount.Status.Owner = serviceAccountRequest.GetOwner()
	serviceAccount.Status.Kind = serviceAccountRequest.GetKind()
	serviceAccount.Status.Id = serviceAccountRequest.GetId()
	serviceAccount.Status.Phase = kafkav1.ServiceAccountReady
}

func UpdateServiceAccountStatus(c client.Client, ctx context.Context, serviceAccountRequest kafkamgmtclient.ServiceAccount, serviceAccount *kafkav1.CloudServiceAccount) error {
	ConvertToServiceAccountStatus(serviceAccountRequest, serviceAccount)
	if err := c.Status().Update(ctx, serviceAccount); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func ConvertToServiceAccountListItemStatus(serviceAccountRequest kafkamgmtclient.ServiceAccountListItem, serviceAccount *kafkav1.CloudServiceAccount) {
	// Otherwise, we need to update the Kafka status
	serviceAccount.Status.CreatedAt.Time = serviceAccountRequest.GetCreatedAt()
	serviceAccount.Status.Href = serviceAccountRequest.GetHref()
	serviceAccount.Status.Owner = serviceAccountRequest.GetOwner()
	serviceAccount.Status.Kind = serviceAccountRequest.GetKind()
	serviceAccount.Status.Id = serviceAccountRequest.GetId()
	serviceAccount.Status.Phase = kafkav1.ServiceAccountReady
}

func UpdateServiceAccountListItemStatus(c client.Client, ctx context.Context, serviceAccountRequest kafkamgmtclient.ServiceAccountListItem, serviceAccount *kafkav1.CloudServiceAccount) error {
	ConvertToServiceAccountListItemStatus(serviceAccountRequest, serviceAccount)
	if err := c.Status().Update(ctx, serviceAccount); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
