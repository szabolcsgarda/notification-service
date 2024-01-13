package common_model

type OperationMode int

const (
	ServiceInstanceQueue OperationMode = iota
	UserQueue
)
