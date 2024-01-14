package common_model

// OperationMode is an emum to represent the operation mode of the service
// ServiceInstanceQueue is the mode when each service instance has a dedicated SQS queue
// UserQueue is the mode when each user has a dedicated SQS queue
type OperationMode int

const (
	ServiceInstanceQueue OperationMode = iota
	UserQueue
)
