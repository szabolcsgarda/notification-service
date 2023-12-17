package logging

import (
	"go.uber.org/zap"
	"log"
	"notification-service/common/common"
)

var logger *zap.Logger

func init() {
	InitLogger(common.GetEnvWithDefault("LOGGING_MODE", "DEVELOPMENT"))
}

func InitLogger(logMode string) {
	var err error = nil
	switch logMode {
	case "DEVELOPMENT":
		logger, err = zap.NewDevelopment()
	case "TEST":
		logger, err = zap.NewDevelopment()
	case "PRODUCTION":
		logger, err = zap.NewProduction()
	default:
		log.Fatal("Unknown logging mode", logMode)
	}
	if err != nil {
		log.Fatal("Error while configuring logging...")
	}
	defer func(logger *zap.Logger) {
		logger.Sync()
	}(logger)
	logger.Info("Logger Configured")
}

func Logger() *zap.Logger {
	return logger
}
