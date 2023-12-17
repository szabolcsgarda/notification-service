package main

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"notification-service/api"
	"notification-service/common/common"
	"notification-service/factory"
	"os"
	"os/signal"
	"time"
)

var zLog zap.Logger
var f factory.Factory

func NewGinServer(business *api.NotificationService) *gin.Engine {
	router := gin.Default()
	router.Use(business.F.Trace().EnsureTracingGin)
	router.Use(business.F.Trace().LogIncomingRequestGin)
	router.Use(business.F.Auth().JwtAuthorizationHandlerGin)
	router.GET("/health", func(c *gin.Context) { return })
	router.GET("/notifications", business.GetNotificationSubscribe)
	return router
}
func main() {
	f = factory.NewFactory("DEPLOYMENT")
	zLog = f.Logger()
	zLog.Info("Server is starting...")

	// Create an instance of handler
	service := api.NewNotificationService(f)
	s := NewGinServer(service)

	server := &http.Server{
		Addr:    ":" + common.GetEnvWithDefault("SERVER_PORT", "8080"),
		Handler: s,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zLog.Fatal("Error while starting server", zap.Any("error", err))
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	select {
	case <-c:
		zLog.Info("SIGINT signal received...")
		zLog.Info("Gracefully shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			zLog.Fatal("Server forced to shutdown: ", zap.Any("error", err))
		}
	}

	zLog.Info("Main thread is terminating...")
	_ = zLog.Sync()
}
