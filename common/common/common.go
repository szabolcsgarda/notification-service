package common

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	commonmodel "notification-service/common/common-model"
	"os"
	"regexp"
)

func GetEnvWithDefault(key string, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

func GetEnvRequired(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		panic(fmt.Sprintf("%s MUST be provided as an environmental variable", key))
	}
	return value
}

type JsonType int64

const (
	Body JsonType = iota
	Header
)

// RestrictRequestJson replaces sensitive values in the request body with "RESTRICTED"
func RestrictRequestJson(body string, jsonType JsonType) string {
	// List of sensitive keys
	sensitiveBodyKeys := []string{"password", "token", "secret"}
	sensitiveHeaderKeys := []string{"token", "Authorization"}
	var keys []string
	switch jsonType {
	case Body:
		keys = sensitiveBodyKeys
	case Header:
		keys = sensitiveHeaderKeys
	}
	// Iterate through sensitive keys and replace corresponding values
	for _, key := range keys {
		m := regexp.MustCompile(fmt.Sprintf(`"%s": ?".*"|"%s": ?\[.*\]`, key, key))
		body = m.ReplaceAllString(body, fmt.Sprintf(`"%s": "RESTRICTED"`, key))
	}
	return body
}

// GetGinHeaderAsString returns the request headers as a JSON string
func GetGinHeaderAsString(req *http.Request) string {
	// Get the headers from the request
	headers := req.Header

	// Convert headers to a map
	headerMap := make(map[string]interface{})
	for key, values := range headers {
		// If there's only one value, store it directly; otherwise, store as an array
		if len(values) == 1 {
			headerMap[key] = values[0]
		} else {
			headerMap[key] = values
		}
	}

	// Convert the map to JSON
	headerJSON, err := json.MarshalIndent(headerMap, "", "  ")
	if err != nil {
		// Handle error if needed
		return "could not convert"
	}
	return string(headerJSON)
}

// ErrorResponse contructs and send an error response to the client with the given parameters and response code
func ErrorResponse(c *gin.Context, code int, err, message, context string) {
	response := commonmodel.ModelError{
		Error_:  err,
		Message: message,
		Context: context,
	}
	c.JSON(code, response)
	c.Abort()
}

func GetStringPointer(value string) *string {
	return &value
}
