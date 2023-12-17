package jwt

import (
	"errors"
	"github.com/MicahParks/keyfunc/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	model "notification-service/common/common-model"
	"notification-service/common/logging"
	"strings"
)

var zLog zap.Logger

const ErrorInvalidJwt = "ERROR_INVALID_JWT"
const ErrorInvalidSignature = "ERROR_INVALID_SIGNATURE"
const ErrorTokenExpired = "ERROR_TOKEN_EXPIRED"
const ErrorTokenProcessable = "ERROR_TOKEN_PROCESSABLE"
const ErrorInvalidApiKey = "ErrorInvalidApiKey"

func init() {
	zLog = *logging.Logger()
}

type Authorization struct {
	jwkAuthEnabled   bool
	apiKeyAutEnabled bool
	jwkUrl           string
	jwks             *keyfunc.JWKS
	apiKeys          map[string]string
	environment      string
}

type AuthorizationInterface interface {
	UpdateJwks()
	JwtAuthorizationHandlerGin(c *gin.Context)
	ParseJWTPayloadGin(c *gin.Context) (result jwt.MapClaims, err error)
}

func CreateAuthorization(environment string, jwkAuthEnabled bool, jwkUrl string) (auth *Authorization) {
	auth = &Authorization{
		jwkAuthEnabled: jwkAuthEnabled,
		jwkUrl:         jwkUrl,
		jwks:           nil,
		apiKeys:        make(map[string]string),
		environment:    environment,
	}
	if jwkAuthEnabled {
		auth.UpdateJwks()
	}
	return
}

func (auth *Authorization) UpdateJwks() {
	if len(auth.jwkUrl) == 0 {
		zLog.Fatal("Cannot retrieve JWK before providing its URL")
	}
	var err error = nil
	auth.jwks, err = keyfunc.Get(auth.jwkUrl, keyfunc.Options{})
	zLog.Info("Updating JWKs...")
	if err != nil {
		zLog.Warn("Failed to get the JWKS from the given URL", zap.Any("error", err))
	}
}

// JwtAuthorizationHandler JWT validation for Gin Router
func (auth *Authorization) JwtAuthorizationHandlerGin(c *gin.Context) {
	if !auth.jwkAuthEnabled {
		errMsg := model.ModelError{Error_: ErrorInvalidJwt, Message: "invalid JWT"}
		zLog.Info(ErrorInvalidJwt, zap.String("trace-id", c.GetHeader("trace-id")))
		c.JSON(401, errMsg)
		c.Abort()
		return
	}
	tokenStr := c.GetHeader("Authorization")
	tokenStr = strings.Replace(tokenStr, "Bearer ", "", -1)
	jwtPulled := false
	for {
		token, err := jwt.Parse(tokenStr, auth.jwks.Keyfunc)
		if token != nil && token.Valid {
			c.Next()
			return
		} else {
			if errors.Is(err, jwt.ErrTokenMalformed) {
				errMsg := model.ModelError{Error_: ErrorInvalidJwt,
					Message: "invalid JWT"}
				zLog.Info(ErrorInvalidJwt, zap.String("trace-id", c.GetHeader("trace-id")))
				c.JSON(401, errMsg)
				return
			} else if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
				//JWT might be outdated, retry after updating JWT
				if !jwtPulled {
					jwtPulled = true
					auth.UpdateJwks()
					continue
				}
				errMsg := model.ModelError{Error_: ErrorInvalidSignature,
					Message: "invalid JWT"}
				zLog.Info(ErrorInvalidSignature, zap.String("trace-id", c.GetHeader("trace-id")))
				c.JSON(401, errMsg)
				return
			} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
				errMsg := model.ModelError{Error_: ErrorTokenExpired,
					Message: "invalid JWT"}
				zLog.Info(ErrorTokenExpired, zap.String("trace-id", c.GetHeader("trace-id")))
				c.JSON(401, errMsg)
				return
			} else {
				errMsg := model.ModelError{Error_: ErrorTokenProcessable,
					Message: "invalid JWT"}
				zLog.Info(ErrorTokenProcessable, zap.String("trace-id", c.GetHeader("trace-id")))
				c.JSON(401, errMsg)
				return
			}
		}
	}

}

func (auth *Authorization) ParseJWTPayloadGin(c *gin.Context) (result jwt.MapClaims, err error) {
	tokenStr := c.GetHeader("Authorization")
	tokenStr = strings.Replace(tokenStr, "Bearer ", "", -1)
	token, err := jwt.Parse(tokenStr, auth.jwks.Keyfunc)
	if token == nil || !token.Valid {
		return result, errors.New(ErrorInvalidJwt)
	}
	result = token.Claims.(jwt.MapClaims)
	err = nil
	return
}
