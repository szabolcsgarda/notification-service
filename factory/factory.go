package factory

import (
	"go.uber.org/zap"
	"notification-service/common/common"
	"notification-service/common/jwt"
	"notification-service/common/logging"
	sqs "notification-service/common/sqs"
	"notification-service/common/trace"
	"notification-service/database"
	dbconfig "notification-service/database/config"
)

type Factory struct {
	db         *database.Database
	zLog       *zap.Logger
	auth       *jwt.Authorization
	sqsService *sqs.SqsService
	trace      *trace.TraceMiddleware
}

type FactoryInterface interface {
	Db() database.DatabaseInterface
	Logger() zap.Logger
	Auth() jwt.AuthorizationInterface
	Sqs() sqs.SqsServiceInterface
	Trace() trace.TraceMiddlewareInterface
}

func NewFactory(environment string) (factory Factory) {
	factory.zLog = logging.Logger()
	if environment == "DEPLOYMENT" {
		//SQL database
		host := common.GetEnvWithDefault("DB_HOST", "localhost")
		//port := common.GetEnvWithDefault("TEST_DB_PORT", "5432")
		user := common.GetEnvWithDefault("DB_USER", "my_user")
		pw := common.GetEnvWithDefault("DB_PW", "my_password")
		dbName := common.GetEnvWithDefault("DB_NAME", "message_service")
		options := make(map[string]string)
		options["sslmode"] = "disable"
		if common.GetEnvWithDefault("DB_TLS", "false") == "true" {
			options["sslmode"] = "verify-ca"
			options["sslrootcert"] = "cert.pem"
		}
		config := dbconfig.NewConfiguration(host, dbName, user, pw, options)
		factory.db = database.GetNewDatabaseConnection(config)
		//Authorization
		factory.auth = jwt.CreateAuthorization(environment, true, common.GetEnvRequired("COGNITO_JWK_URL"))
		//SQS service
		factory.sqsService = sqs.NewSqsService(factory.zLog)
		//Tracing
		factory.trace = trace.NewTraceMiddleware(environment, "trace-id", factory.zLog)

	} else {
		panic("invalid environment: " + environment)
	}
	return
}

func (f Factory) Db() database.DatabaseInterface {
	return f.db
}

func (f Factory) Logger() zap.Logger {
	return *f.zLog
}

func (f Factory) Auth() jwt.AuthorizationInterface {
	return f.auth
}

func (f Factory) Sqs() sqs.SqsServiceInterface {
	return f.sqsService
}

func (f Factory) Trace() trace.TraceMiddlewareInterface {
	return f.trace
}
