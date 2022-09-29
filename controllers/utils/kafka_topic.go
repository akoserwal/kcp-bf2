package utils

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	kafkainstanceclient "github.com/redhat-developer/app-services-sdk-go/kafkainstance/apiv1/client"
	kafkav1 "pmuir/kcp-bf2/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ConvertToKafkaTopic(topic kafkainstanceclient.Topic, namespace string, kafkaTopic *kafkav1.KafkaTopic) {
	ConvertToKafkaTopicSpec(topic, kafkaTopic)
	kafkaTopic.ObjectMeta.Name = fmt.Sprintf("%s", EncodeKubernetesName(topic.GetName(), 63-21))
	kafkaTopic.ObjectMeta.Namespace = namespace
}

func ConvertToNewTopicInput(kafkaTopic *kafkav1.KafkaTopic) kafkainstanceclient.NewTopicInput {
	config := make([]kafkainstanceclient.ConfigEntry, 0)
	for key, value := range kafkaTopic.Spec.Config {
		config = append(config, kafkainstanceclient.ConfigEntry{
			Key:   key,
			Value: value,
		})
	}
	return kafkainstanceclient.NewTopicInput{
		Name: kafkaTopic.GetName(),
		Settings: kafkainstanceclient.TopicSettings{
			NumPartitions: &kafkaTopic.Spec.Partitions,
			Config:        &config,
		},
	}
}

func ConvertToKafkaTopicSpec(detail kafkainstanceclient.Topic, kafkaTopic *kafkav1.KafkaTopic) {
	kafkaTopic.Spec.TopicName = detail.GetName()
	kafkaTopic.Spec.Partitions = int32(len(detail.GetPartitions()))
	kafkaTopic.Spec.Config = make(map[string]string)
	for _, entry := range detail.GetConfig() {
		kafkaTopic.Spec.Config[entry.Key] = entry.Value
	}
}

func ConvertToKafkaTopicStatus(topic kafkainstanceclient.Topic, kafkaTopic *kafkav1.KafkaTopic) {
	// Otherwise, we need to update the Kafka status
	kafkaTopic.Status.Phase = kafkav1.KafkaTopicReady
}

func UpdateKafkaTopicStatus(c client.Client, ctx context.Context, topic kafkainstanceclient.Topic, kafkaTopic *kafkav1.KafkaTopic) error {
	ConvertToKafkaTopicStatus(topic, kafkaTopic)
	if err := c.Status().Update(ctx, kafkaTopic); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
