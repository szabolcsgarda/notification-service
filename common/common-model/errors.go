package common_model

import (
	"errors"
	"go.uber.org/zap"
)

var _, _ = zap.NewDevelopment()

var ErrDbUnexpected = errors.New("ERROR_DB_UNEXPECTED_ERROR")
var ErrInvalidBody = errors.New("INVALID_REQUEST_BODY")
var ErrInvalidToken = errors.New("INVALID_TOKEN")

var ErrInvalidArgument = errors.New("ERROR_INVALID_ARGUMENT")
var ErrContentTooLong = errors.New("ERROR_CONTENT_TOO_LONG")
var ErrSqsUnexpected = errors.New("ERROR_SQS_UNEXPECTED_ERROR")
var ErrSqsInvalidMessage = errors.New("ERROR_SQS_INVALID_MESSAGE")
var ErrSqsInternalServerError = errors.New("ERROR_INTERNAL_SERVER_ERROR")

var ErrLongPollingCouldNotDeliver = errors.New("ERROR_LONG_POLLING_COULD_NOT_DELIVER")
