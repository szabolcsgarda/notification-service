package common_model

type ModelError struct {
	// The machine interpretable error code
	Error_ string `json:"error"`
	// A human readable error message
	Message string `json:"message,omitempty"`
	// Optional error context information (or trace id)
	Context string `json:"context,omitempty"`
}
