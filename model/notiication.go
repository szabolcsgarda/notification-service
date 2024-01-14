package model

import (
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	commonmodel "notification-service/common/common-model"
)

type Notification struct {
	Id        string `json:"id"`
	Addressee string `json:"addressee"`
	Body      string `json:"body"`
	Subject   string `json:"subject"`
}

func CreateNotification(message awssqs.Message) (Notification, error) {
	if message.Body == nil || len(*message.Body) == 0 || message.MessageAttributes == nil || len(message.MessageAttributes) == 0 {
		return Notification{}, commonmodel.ErrSqsInvalidMessage
	}
	id := message.MessageId
	addressee := message.MessageAttributes["addressee"]
	subject := message.MessageAttributes["subject"]
	body := *message.Body

	if addressee == nil || id == nil || subject == nil || len(body) == 0 {
		return Notification{}, commonmodel.ErrSqsInvalidMessage
	}
	return Notification{
		Id:        *id,
		Addressee: *addressee.StringValue,
		Subject:   *subject.StringValue,
		Body:      body,
	}, nil
}
