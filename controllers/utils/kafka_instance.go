package utils

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	kafkamgmtclient "github.com/redhat-developer/app-services-sdk-go/kafkamgmt/apiv1/client"
	kafkav1 "pmuir/kcp-bf2/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

func ConvertToKafkaInstance(kafkaRequest kafkamgmtclient.KafkaRequest, namespace string, kafkaInstance *kafkav1.KafkaInstance) {
	ConvertToKafkaInstanceSpec(kafkaRequest, kafkaInstance)
	kafkaInstance.ObjectMeta.Name = fmt.Sprintf("%s-%s", EncodeKubernetesName(kafkaRequest.GetName(), 63-21), kafkaRequest.GetId())
	kafkaInstance.ObjectMeta.Namespace = namespace
}

func ConvertToKafkaInstanceSpec(kafkaRequest kafkamgmtclient.KafkaRequest, kafkaInstance *kafkav1.KafkaInstance) {
	multiAz := kafkaRequest.GetMultiAz()
	reauthenticationEnabled := kafkaRequest.GetReauthenticationEnabled()
	kafkaInstance.Spec.MultiAz = &multiAz
	kafkaInstance.Spec.Region = kafkaRequest.GetRegion()
	kafkaInstance.Spec.Name = kafkaRequest.GetName()
	kafkaInstance.Spec.ReauthenticationEnabled = &reauthenticationEnabled
	kafkaInstance.Spec.CloudProvider = kafkaRequest.GetCloudProvider()
}

func ConvertToKafkaInstanceStatus(kafkaRequest kafkamgmtclient.KafkaRequest, kafkaInstance *kafkav1.KafkaInstance) {
	// Otherwise, we need to update the Kafka status
	kafkaInstance.Status.InstanceType = kafkaRequest.GetInstanceType()
	kafkaInstance.Status.BootstrapServerHost = kafkaRequest.GetBootstrapServerHost()
	kafkaInstance.Status.CreatedAt.Time = kafkaRequest.GetCreatedAt()
	kafkaInstance.Status.Href = kafkaRequest.GetHref()
	kafkaInstance.Status.Owner = kafkaRequest.GetOwner()
	kafkaInstance.Status.Kind = kafkaRequest.GetKind()
	kafkaInstance.Status.UpdatedAt.Time = kafkaRequest.GetUpdatedAt()
	kafkaInstance.Status.Version = kafkaRequest.GetVersion()
	kafkaInstance.Status.Id = kafkaRequest.GetId()
	kafkaInstance.Status.Phase = kafkav1.KafkaPhase(strings.Title(kafkaRequest.GetStatus()))
}

func UpdateKafkaInstanceStatus(c client.Client, ctx context.Context, kafkaRequest kafkamgmtclient.KafkaRequest, kafkaInstance *kafkav1.KafkaInstance) error {
	ConvertToKafkaInstanceStatus(kafkaRequest, kafkaInstance)
	if err := c.Status().Update(ctx, kafkaInstance); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
