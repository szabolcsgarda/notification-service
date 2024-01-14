package trace

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"io"
	"notification-service/common/common"
)

// TraceMiddleware is a middleware to generate traceId for each incoming request (if it doesn't have) and log it
type TraceMiddleware struct {
	TracingHeaderParameter string
	Environment            string
	Logger                 *zap.Logger
}

func NewTraceMiddleware(environment, headerParameter string, logger *zap.Logger) *TraceMiddleware {
	service := TraceMiddleware{Environment: environment, TracingHeaderParameter: headerParameter, Logger: logger}
	return &service
}

type TraceMiddlewareInterface interface {
	EnsureTracingGin(c *gin.Context)
	LogIncomingRequestGin(c *gin.Context)
}

// EnsureTracingGin ensures that each incoming request has a traceId
func (t *TraceMiddleware) EnsureTracingGin(c *gin.Context) {
	traceId := c.GetHeader(t.TracingHeaderParameter)
	if traceId == "" {
		traceId = uuid.New().String()
		c.Request.Header.Set(t.TracingHeaderParameter, traceId)
	}
	c.Next()
}

// LogIncomingRequestGin logs the incoming request while restricts sensitive data
func (t *TraceMiddleware) LogIncomingRequestGin(c *gin.Context) {
	restrictedBody := ""
	restrictedHeader := ""
	bodyStr, err := io.ReadAll(c.Request.Body)
	if err != nil {
		restrictedBody = "Couldn't get body"
	} else {
		restrictedBody = common.RestrictRequestJson(string(bodyStr), common.Body)
	}
	restrictedHeader = common.RestrictRequestJson(common.GetGinHeaderAsString(c.Request), common.Header)
	t.Logger.Debug("New request",
		zap.String("method", c.Request.Method),
		zap.String("url", c.Request.URL.String()),
		zap.String("client-ip", c.ClientIP()),
		zap.String("trace-id", c.Request.Header.Get(t.TracingHeaderParameter)),
		zap.String("body", restrictedBody),
		zap.String("header", restrictedHeader),
		zap.String("environment", t.Environment),
		//zap.String("container-id", c.Request.Method),
		//zap.String("container-name", c.Request.Method),
	)
}
