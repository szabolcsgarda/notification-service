package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"notification-service/common/common"
	commonmodel "notification-service/common/common-model"
	sqs "notification-service/common/sqs"
	"notification-service/database"
	"notification-service/factory"
	"notification-service/model"
	"strconv"
	"time"
)

const ErrorInvalidJwt = "ERROR_INVALID_JWT"
const ErrorInternalServerError = "ERROR_INTERNAL_SERVER_ERROR"

// NotificationService is an object type that implements all the functionalities to handle incoming messages and deliver them to the addressee
type NotificationService struct {
	F                 factory.FactoryInterface
	zLog              zap.Logger
	d                 database.DatabaseInterface
	sqs               sqs.SqsServiceInterface
	sessions          map[string]chan string
	maxTimeoutSeconds int
	serviceInstanceId string
	queueUrl          *string
	receiveMessage    chan model.NotificationMeta
	doneChannel       chan interface{}
	operationMode     commonmodel.OperationMode
	userQueueBaseUrl  *string
}

// NewNotificationService is a factory function that creates a new NotificationService instance
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
		receiveMessage:    make(chan model.NotificationMeta),
		doneChannel:       make(chan interface{}),
		operationMode:     factory.Mode(),
		userQueueBaseUrl:  userQueueBaseUrl,
	}

	//Subscribe to the service instance queue
	if factory.Mode() == commonmodel.ServiceInstanceQueue {
		errSubscribe := service.sqs.ReceiveNotification(&service.doneChannel, &service.receiveMessage, service.queueUrl, 15)
		if errSubscribe != nil {
			service.zLog.Fatal("Error while subscribing to the queue", zap.Any("error", errSubscribe))
		}
	}

	//Start handler for incoming messages
	go func() {
		for {
			select {
			case <-service.doneChannel:
				service.zLog.Debug("Done notification caught... Stopping handler function receiving messages")
			case notification := <-service.receiveMessage:
				err := service.HandleIncomingNotification(notification)
				needToBeDeleted := false
				if err == nil {
					needToBeDeleted = true
				} else {
					service.zLog.Error("Error while handling incoming notification", zap.String("message_id", notification.Notification.Id), zap.Any("error", err))
					if errors.Is(err, commonmodel.ErrSqsInvalidMessage) {
						needToBeDeleted = true
					}
				}
				if needToBeDeleted {
					// Delete the notification from the queue
					err = service.sqs.DeleteMessage(notification.QueueUrl, notification.ReceiptHandle)
					if err != nil {
						service.zLog.Error("Error while deleting notification from the queue", zap.String("message_id", notification.Notification.Id), zap.Any("error", err))
					}
				}
			}
		}
	}()

	return &service
}

// HandleIncomingNotification handles incoming awssqs.Message objects, received from different SQS queues
// The function validates the message and if it is valid, forwards it to the corresponding client through a dedicated channel
// All message is deleted from the queue after delivery, or if it is formally invalid, however it is kept in case of delivery failure
func (s NotificationService) HandleIncomingNotification(notification model.NotificationMeta) (err error) {

	// Request checked, performing delivery if the addressee is connected
	if channel, ok := s.sessions[notification.Notification.Addressee]; ok {
		channel <- notification.Notification.Body
	} else {
		s.zLog.Debug("User not connected, notification not delivered", zap.String("message_id", notification.Notification.Id))
		//Message is valid and addressee is provided, however we could not deliver it, possible addressee disconnected, thus
		//we do not delete the notification from the queue here, we let it reach the dead-letter-queue, so an alternative way of delivery
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
		common.ErrorResponse(c, 400, ErrorInvalidJwt, "Invalid token", c.GetHeader("trace-id"))
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
			common.ErrorResponse(c, 500, ErrorInternalServerError, "Internal Server Error", c.GetHeader("trace-id"))
			return
		}
	} else if s.operationMode == commonmodel.UserQueue {
		//Subscribe to the user queue
		errSubscribe := s.sqs.ReceiveNotification(&sessionClosed, &s.receiveMessage, getUserQueueUrl(*s.userQueueBaseUrl, client), 15)
		if errSubscribe != nil {
			s.zLog.Error("Error while subscribing to the queue", zap.Any("error", errSubscribe))
			s.sessions[client] = nil
			common.ErrorResponse(c, 500, ErrorInternalServerError, "Internal Server Error", c.GetHeader("trace-id"))
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
	stop := false
	for !stop {
		select {
		case <-closeNotify:
			s.zLog.Debug("HTTP connection just closed", zap.String("trace-id:", c.GetHeader("trace-id")))
			stop = true
		case <-timeoutChannel:
			s.zLog.Debug("Connection timed out", zap.String("trace-id:", c.GetHeader("trace-id")))
			stop = true
		case sessionMessage := <-sessionChannel:
			s.zLog.Debug("Sending message to client", zap.String("trace-id:", c.GetHeader("trace-id")), zap.Any("message", sessionMessage))
			_, err := c.Writer.WriteString(sessionMessage)
			if err != nil {
				s.zLog.Debug("Error while notifying client", zap.String("trace-id:", c.GetHeader("trace-id")), zap.Any("error", err))
			}
			c.Writer.Flush()
		}
	}
	s.zLog.Debug("Cleaning up after connection", zap.String("trace-id:", c.GetHeader("trace-id")))
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
	c.JSON(288, nil)
}

// getUserQueueUrl returns the URL of the user queue for the given user
func getUserQueueUrl(baseUrl, userId string) (queueUrl *string) {
	return common.GetStringPointer(baseUrl + "-" + userId)
}
