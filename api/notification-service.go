package api

import (
	"errors"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"notification-service/common/common"
	commonmodel "notification-service/common/common-model"
	sqs "notification-service/common/sqs"
	"notification-service/database"
	"notification-service/factory"
	"strconv"
	"time"
)

type NotificationService struct {
	F                 factory.FactoryInterface
	zLog              zap.Logger
	d                 database.DatabaseInterface
	sqs               sqs.SqsServiceInterface
	sessions          map[string]chan string
	maxTimeoutSeconds int
	serviceInstanceId string
	queueUrl          *string
	receiveMessage    chan awssqs.Message
	doneChannel       chan interface{}
	operationMode     commonmodel.OperationMode
	userQueueBaseUrl  *string
}

func NewNotificationService(factory factory.FactoryInterface) *NotificationService {
	var uuidProvided = uuid.UUID{}
	if uuidProvidedStr := common.GetEnvWithDefault("NOTIFICATION_SERVICE_CLIENT_ID", ""); uuidProvidedStr != "" {
		uuidProvided, _ = uuid.Parse(uuidProvidedStr)
	} else {
		uuidProvided = uuid.New()
	}
	log := factory.Logger()
	timeout, err := strconv.Atoi(common.GetEnvWithDefault("MAX_TIMEOUT_SECONDS", "600"))
	if err != nil {
		log.Fatal("Error while parsing MAX_TIMEOUT_SECONDS", zap.Any("error", err))
	}
	//Create the queue if the uuid was not provided, if it was then expect that the url is provided too and do not create the queue again
	var queueUrl *string
	var userQueueBaseUrl *string
	if factory.Mode() == commonmodel.ServiceInstanceQueue {
		if uuidProvidedStr := common.GetEnvWithDefault("NOTIFICATION_SERVICE_CLIENT_ID", ""); uuidProvidedStr != "" {
			uuidProvided, _ = uuid.Parse(uuidProvidedStr)
			queueUrl = common.GetStringPointer(common.GetEnvRequired("SQS_QUEUE_URL"))
		} else {
			queueName := common.GetEnvRequired("SQS_QUEUE_NAME_PREFIX") + "-" + uuidProvided.String()
			queueUrl, err = factory.Sqs().CreateMessageQueue(queueName, nil, nil, nil, nil)
			if err != nil {
				log.Fatal("Error while creating queue", zap.Any("error", err))
			}
		}
	} else if factory.Mode() == commonmodel.UserQueue {
		userQueueBaseUrl = common.GetStringPointer(common.GetEnvRequired("SQS_USER_QUEUE_BASE_URL"))
	}
	service := NotificationService{
		F:                 factory,
		zLog:              factory.Logger(),
		d:                 factory.Db(),
		sqs:               factory.Sqs(),
		sessions:          make(map[string]chan string),
		maxTimeoutSeconds: timeout,
		serviceInstanceId: uuidProvided.String(),
		queueUrl:          queueUrl,
		receiveMessage:    make(chan awssqs.Message),
		doneChannel:       make(chan interface{}),
		operationMode:     factory.Mode(),
		userQueueBaseUrl:  userQueueBaseUrl,
	}

	//Subscribe to the service instance queue
	if factory.Mode() == commonmodel.ServiceInstanceQueue {
		errSubscribe := service.sqs.ReceiveMessage(&service.doneChannel, &service.receiveMessage, service.queueUrl, 15)
		if errSubscribe != nil {
			service.zLog.Fatal("Error while subscribing to the queue", zap.Any("error", errSubscribe))
		}
	}

	//Start handler for incoming messages
	go func() {
		for {
			select {
			case <-service.doneChannel:
				service.zLog.Debug("Done message caught... Stopping handler function receiving messages")
			case message := <-service.receiveMessage:
				err := service.HandleIncomingNotification(message)
				needToBeDeleted := false
				if err == nil {
					needToBeDeleted = true
				} else {
					service.zLog.Error("Error while handling incoming message", zap.String("message_id", *message.MessageId), zap.Any("error", err))
					if errors.Is(err, commonmodel.ErrSqsInvalidMessage) {
						needToBeDeleted = true
					}
				}
				if needToBeDeleted {
					// Delete the message from the queue
					err = service.sqs.DeleteMessage(*service.queueUrl, *message.ReceiptHandle)
					if err != nil {
						service.zLog.Error("Error while deleting message from the queue", zap.String("message_id", *message.MessageId), zap.Any("error", err))
					}
				}
			}
		}
	}()

	return &service
}

func (s NotificationService) HandleIncomingNotification(message awssqs.Message) (err error) {
	userMessage := *message.Body
	if message.MessageAttributes == nil || len(message.MessageAttributes) == 0 {
		s.zLog.Error("No message attributes provided, cannot be delivered", zap.String("message_id", *message.MessageId), zap.Any("message", message))
		return commonmodel.ErrSqsInvalidMessage
	}
	addressee := message.MessageAttributes["addressee"].StringValue
	if addressee == nil {
		s.zLog.Error("No addressee provided, cannot be delivered", zap.String("message_id", *message.MessageId), zap.Any("message", message))
		return commonmodel.ErrSqsInvalidMessage
	}

	// Request checked, performing delivery if the addressee is connected
	if channel, ok := s.sessions[*addressee]; ok {
		channel <- userMessage
	} else {
		s.zLog.Debug("User not connected, message not delivered", zap.String("message_id", *message.MessageId))
		//Message is valid and addressee is provided, however we could not deliver it, possible addressee disconnected, thus
		//we do not delete the message from the queue here, we let it reach the dead-letter-queue, so an alternative way of delivery
		//will be possible (e.g.: push notifications
		return commonmodel.ErrLongPollingCouldNotDeliver
	}
	return nil
}

func (s NotificationService) GetNotificationSubscribe(c *gin.Context) {
	//Set up a listener to detect when the client closes the connection, or losing the connection by whatever reason
	s.zLog.Debug("Setting up event listening", zap.String("trace-id:", c.GetHeader("trace-id")))
	closeNotify := c.Writer.CloseNotify()
	tokenParsed, errToken := s.F.Auth().ParseJWTPayloadGin(c)
	if errToken != nil {
		s.zLog.Debug("Error while parsing token", zap.Any("error", errToken), zap.String("trace-id:", c.GetHeader("trace-id")))
		c.Status(400)
		return
	}
	client := tokenParsed["sub"].(string)
	sessionClosed := make(chan interface{})

	//Create dedicated channel for this client
	sessionChannel := make(chan string)
	s.sessions[client] = sessionChannel

	if s.operationMode == commonmodel.ServiceInstanceQueue {
		//Assign the service ID to the client in the database
		err := s.d.UpdateClientServiceId(client, s.serviceInstanceId)
		if err != nil {
			s.zLog.Error("Error while assigning service ID to client", zap.String("trace-id:", c.GetHeader("trace-id")), zap.Any("error", err))
			s.sessions[client] = nil
			c.Status(500)
			return
		}
	} else if s.operationMode == commonmodel.UserQueue {
		//Subscribe to the user queue
		errSubscribe := s.sqs.ReceiveMessage(&sessionClosed, &s.receiveMessage, getUserQueueUrl(*s.userQueueBaseUrl, client), 15)
		if errSubscribe != nil {
			s.zLog.Error("Error while subscribing to the queue", zap.Any("error", errSubscribe))
			s.sessions[client] = nil
			c.Status(500)
			return
		}
	}

	//Set up headers and flush it immediately to let client know that connection is established
	//and the server will stream data through the established connection
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Flush()

	//Set up timeout and terminate connection in case it happens
	timeoutChannel := make(chan interface{})
	go func() {
		time.Sleep(time.Duration(s.maxTimeoutSeconds) * time.Second)
		timeoutChannel <- true
	}()

	//Start listening events and also monitor the closeNotify channel
	for {
	selectLoop:
		select {
		case <-closeNotify:
			s.zLog.Debug("HTTP connection just closed", zap.String("trace-id:", c.GetHeader("trace-id")))
			break selectLoop
		case <-timeoutChannel:
			s.zLog.Debug("Connection timed out", zap.String("trace-id:", c.GetHeader("trace-id")))
			break selectLoop
		case sessionMessage := <-sessionChannel:
			s.zLog.Debug("Sending message to client", zap.String("trace-id:", c.GetHeader("trace-id")), zap.Any("message", sessionMessage))
			_, err := c.Writer.WriteString(sessionMessage)
			if err != nil {
				s.zLog.Debug("Error while notifying client", zap.String("trace-id:", c.GetHeader("trace-id")), zap.Any("error", err))
			}
			c.Writer.Flush()
		}
	}
	if s.operationMode == commonmodel.ServiceInstanceQueue {
		err := s.d.UpdateClientServiceIdToNull(client)
		if err != nil {
			s.zLog.Error("Error while removing service ID from client", zap.String("trace-id:", c.GetHeader("trace-id")), zap.Any("error", err))
		}
	} else if s.operationMode == commonmodel.UserQueue {
		sessionClosed <- true
	}
	close(sessionChannel)
	delete(s.sessions, client)
}

func getUserQueueUrl(baseUrl, userId string) (queueUrl *string) {
	return common.GetStringPointer(baseUrl + "-" + userId)
}
