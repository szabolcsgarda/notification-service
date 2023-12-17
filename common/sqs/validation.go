package service

import (
	commonmodel "notification-service/common/common-model"
)

func validateSqsMessage(message string) error {
	if len(message) > MaxMessageBodySize {
		return commonmodel.ErrContentTooLong
	}
	//TODO add valuation for characters
	return nil
}
