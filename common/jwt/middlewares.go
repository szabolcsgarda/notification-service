package jwt

import (
	"errors"
	"github.com/MicahParks/keyfunc/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"notification-service/common/common"
	model "notification-service/common/common-model"
	"notification-service/common/logging"
	"strings"
)

var zLog zap.Logger

const ErrorInvalidJwt = "ERROR_INVALID_JWT"
const ErrorInvalidSignature = "ERROR_INVALID_SIGNATURE"
const ErrorTokenExpired = "ERROR_TOKEN_EXPIRED"
const ErrorTokenProcessable = "ERROR_TOKEN_PROCESSABLE"

func init() {
	zLog = *logging.Logger()
}

// Authorization is an object type that contains all objects to manage JWT and token-based authentication
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

// CreateAuthorization is a factory function that creates a new Authorization instance
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

// UpdateJwks updates the JWKs from the given URL (tested with AWS cognito)
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

// JwtAuthorizationHandlerGin JWT validation for Gin Router
// Returns with error and terminates the connection if the JWT is invalid
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
				zLog.Info(ErrorInvalidJwt, zap.String("trace-id", c.GetHeader("trace-id")))
				common.ErrorResponse(c, 401, ErrorInvalidJwt, "Invalid JWT", c.GetHeader("trace-id"))
			} else if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
				//JWT might be outdated, retry after updating JWT
				if !jwtPulled {
					jwtPulled = true
					auth.UpdateJwks()
					continue
				}
				zLog.Info(ErrorInvalidSignature, zap.String("trace-id", c.GetHeader("trace-id")))
				common.ErrorResponse(c, 401, ErrorInvalidSignature, "Invalid JWT signature", c.GetHeader("trace-id"))
			} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
				zLog.Info(ErrorTokenExpired, zap.String("trace-id", c.GetHeader("trace-id")))
				common.ErrorResponse(c, 401, ErrorTokenExpired, "Token expired", c.GetHeader("trace-id"))
			} else {
				zLog.Info(ErrorTokenProcessable, zap.String("trace-id", c.GetHeader("trace-id")))
				common.ErrorResponse(c, 401, ErrorTokenProcessable, "Invalid JWT", c.GetHeader("trace-id"))
			}
			return
		}
	}

}

// ParseJWTPayloadGin parses the JWT payload from the given Gin context and returns a jwt.MapClaims object
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
