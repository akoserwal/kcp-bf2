package utils

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	kafkainstance "github.com/redhat-developer/app-services-sdk-go/kafkainstance/apiv1"
	kafkainstanceclient "github.com/redhat-developer/app-services-sdk-go/kafkainstance/apiv1/client"
	kafkamgmtclient "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/client"
	"golang.org/x/oauth2/clientcredentials"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func CreateKafkaAdmin(ctx context.Context, bootstrapServerHost string, clientId string, clientSecret string) (*kafkainstanceclient.APIClient, error) {
	ts := clientcredentials.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		TokenURL:     "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token",
	}
	fmt.Println(bootstrapServerHost)
	client := kafkainstance.NewAPIClient(&kafkainstance.Config{
		BaseURL:    bootstrapServerHost,
		HTTPClient: ts.Client(ctx),
	})

	return client, nil
}

func GetOrCreateServiceAccount(ctx context.Context, kubernetesClient client.Client, cloudClient *kafkamgmtclient.APIClient, name string, namespace string) (string, string, error) {

	log := log.FromContext(ctx)
	// Make sure we have a service account - this is a huge hack
	serviceAccountName := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	var serviceAccountSecret corev1.Secret
	if err := kubernetesClient.Get(ctx, serviceAccountName, &serviceAccountSecret); err != nil {
		// means a service account for the reconciler doesn't exist so try to create one
		apiName := fmt.Sprintf("kcp-%s", serviceAccountName.Name)
		serviceAccountReq, _, err := cloudClient.SecurityApi.CreateServiceAccount(ctx).ServiceAccountRequest(kafkamgmtclient.ServiceAccountRequest{Name: apiName}).Execute()
		if err != nil {
			apiErr := GetAPIError(err)
			return "", "", errors.Wrapf(err, fmt.Sprintf("%s %s", apiErr.GetCode(), apiErr.GetReason()))
		}

		serviceAccountSecret = corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      serviceAccountName.Name,
				Namespace: serviceAccountName.Namespace,
			},
			Data: map[string][]byte{
				"clientId":     []byte(serviceAccountReq.GetClientId()),
				"clientSecret": []byte(serviceAccountReq.GetClientSecret()),
			},
		}
		err = kubernetesClient.Create(ctx, &serviceAccountSecret)
		if err != nil {
			return "", "", errors.WithStack(err)
		}
		// Big problem, service accounts by default don't have full access to the data plane by default, so the user has to go in manually and grant it
		log.Info("service Account created", "name", apiName)
		log.Info("you must grant this service account admin access to the Kafka instance for full management")
		return serviceAccountReq.GetClientId(), serviceAccountReq.GetClientSecret(), nil
	}

	return string(serviceAccountSecret.Data["clientId"]), string(serviceAccountSecret.Data["clientSecret"]), nil
}
