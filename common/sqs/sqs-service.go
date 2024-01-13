package service

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/zap"
	commonmodel "notification-service/common/common-model"
	"reflect"
	"strconv"
)

const MaxMessageBodySize = 262144

type SqsService struct {
	session *session.Session
	sqs     *sqs.SQS
	log     *zap.Logger
	//Todo: add some kind of circuit breaker logic, to avoid infinite high frequency retrying in case of network issues
}

type SqsServiceInterface interface {
	SendMessageToQueue(queueUrl string, message string, messageAttributes *map[string]interface{}) (messageId *string, err error)
	ReceiveMessage(done *chan interface{}, c *chan sqs.Message, queueUrl *string, visibilityTimeout int64) (err error)
	CreateMessageQueue(queueName string, delaySeconds, retentionPeriodSeconds, maxReceiveCount *int, deadLetterQueueArn *string) (queueUrl *string, err error)
	DeleteMessage(queueUrl string, receiptHandle string) (err error)
}

func NewSqsService(logger *zap.Logger) (sqsService *SqsService) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable}))
	sqsBuff := sqs.New(sess)
	return &SqsService{
		session: sess,
		sqs:     sqsBuff,
		log:     logger,
	}
}

func (s *SqsService) SendMessageToQueue(queueUrl string, message string, messageAttributes *map[string]interface{}) (messageId *string, err error) {
	if len(queueUrl) < 5 || len(message) < 5 {
		s.log.Error("Queue name or message is too short", zap.String("queueUrl", queueUrl), zap.String("message", message))
		return nil, commonmodel.ErrInvalidArgument
	}
	if messageValidation := validateSqsMessage(message); messageValidation != nil {
		if errors.Is(messageValidation, commonmodel.ErrContentTooLong) {
			s.log.Error("Message too long", zap.String("queueUrl", queueUrl), zap.String("message", message))
		}
		return nil, commonmodel.ErrContentTooLong
	}
	input := sqs.SendMessageInput{
		MessageBody: &message,
		QueueUrl:    &queueUrl,
	}
	if messageAttributes != nil {
		inputAttributes := make(map[string]*sqs.MessageAttributeValue)
		for key, value := range *messageAttributes {
			valueType := reflect.TypeOf(value)
			switch valueType.Kind() {
			case reflect.String:
				inputAttributes[key] = &sqs.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String(value.(string)),
				}
				break
			case reflect.Int, reflect.Float64, reflect.Int64:
				inputAttributes[key] = &sqs.MessageAttributeValue{
					DataType:    aws.String("Number"),
					StringValue: aws.String(value.(string)),
				}
				break
			default:
				s.log.Error("Invalid message attribute type", zap.String("queueUrl", queueUrl), zap.String("message", message), zap.Any("map_entry", value), zap.Any("data_type", valueType))
				return nil, commonmodel.ErrInvalidArgument
			}
		}
	}
	result, err := s.sqs.SendMessage(&input)
	if err != nil {
		s.log.Error("Error while sending message", zap.String("queueUrl", queueUrl), zap.String("message", message), zap.Any("error", err))
		return nil, commonmodel.ErrSqsUnexpected
	}
	return result.MessageId, nil
}

func (s *SqsService) ReceiveMessage(done *chan interface{}, c *chan sqs.Message, queueUrl *string, visibilityTimeout int64) (err error) {
	if visibilityTimeout < 0 || visibilityTimeout > 43200 {
		s.log.Error("Invalid visibility timeout", zap.Int64("visibilityTimeout", visibilityTimeout))
		return commonmodel.ErrInvalidArgument
	}
	if queueUrl == nil || len(*queueUrl) < 5 {
		s.log.Error("Invalid queue url", zap.String("queueUrl", *queueUrl))
		return commonmodel.ErrInvalidArgument
	}
	terminated := false
	go func(done *chan interface{}) {
		<-*done
		s.log.Debug("Stopping receiving messages", zap.String("queueUrl", *queueUrl))
		terminated = true
	}(done)
	go func(c *chan sqs.Message, queueUrl *string, visibilityTimeout int64, terminated *bool) {
		for !*terminated {
			msgResult, err := s.sqs.ReceiveMessage(&sqs.ReceiveMessageInput{
				AttributeNames: []*string{
					aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
				},
				MessageAttributeNames: []*string{
					aws.String(sqs.QueueAttributeNameAll),
				},
				QueueUrl:            queueUrl,
				MaxNumberOfMessages: aws.Int64(1),
				VisibilityTimeout:   aws.Int64(visibilityTimeout),
			})
			if err != nil {
				s.log.Error("Error while receiving message", zap.String("queueUrl", *queueUrl), zap.Any("error", err))
			} else {
				s.log.Debug("Received message", zap.String("queueUrl", *queueUrl), zap.Any("quantity", len(msgResult.Messages)))
				messages := msgResult.Messages
				for _, message := range messages {
					*c <- *message
				}
			}
		}
		s.log.Debug("Stopped receiving messages", zap.String("queueUrl", *queueUrl))
	}(c, queueUrl, visibilityTimeout, &terminated)
	s.log.Debug("Started receiving messages", zap.String("queueUrl", *queueUrl))
	return nil
}

func (s *SqsService) CreateMessageQueue(queueName string, delaySeconds, retentionPeriodSeconds, maxReceiveCount *int, deadLetterQueueArn *string) (queueUrl *string, err error) {
	if len(queueName) < 5 {
		s.log.Error("Queue name is too short", zap.String("queueName", queueName))
		return nil, commonmodel.ErrInvalidArgument
	}
	attributes := make(map[string]*string)
	if delaySeconds != nil {
		attributes["DelaySeconds"] = aws.String(strconv.Itoa(*delaySeconds))
	}
	if retentionPeriodSeconds != nil {
		attributes["MessageRetentionPeriod"] = aws.String(strconv.Itoa(*retentionPeriodSeconds))
	}
	if deadLetterQueueArn != nil {
		attributes["deadLetterTargetArn"] = deadLetterQueueArn
	}
	if maxReceiveCount != nil {
		attributes["maxReceiveCount"] = aws.String(strconv.Itoa(*maxReceiveCount))
	}
	attributes["ReceiveMessageWaitTimeSeconds"] = aws.String("20")

	s.log.Debug("Creating queue", zap.String("queueName", queueName), zap.Any("attributes", attributes))
	result, err := s.sqs.CreateQueue(&sqs.CreateQueueInput{
		QueueName:  &queueName,
		Attributes: attributes,
	})
	if err != nil {
		s.log.Error("Error while creating queue", zap.String("queueName", queueName), zap.Any("error", err))
		return nil, commonmodel.ErrSqsUnexpected
	}
	return result.QueueUrl, nil
}

func (s *SqsService) DeleteMessage(queueUrl string, receiptHandle string) (err error) {
	_, err = s.sqs.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &queueUrl,
		ReceiptHandle: &receiptHandle,
	})
	if err != nil {
		s.log.Error("Error while deleting message", zap.String("queueUrl", queueUrl), zap.String("receiptHandle", receiptHandle), zap.Any("error", err))
		return commonmodel.ErrSqsUnexpected
	}
	return nil
}
