package response

import (
	"net/http"
)

// Response is a standardized API response structure
type Response struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// Success writes a successful response with data
func Success(w http.ResponseWriter, statusCode int, message string, data any) {
	resp := Response{
		Code:    statusCode,
		Success: true,
		Message: message,
		Data:    data,
	}
	WriteJSON(w, statusCode, resp)
}

// Error writes an error response
func Error(w http.ResponseWriter, statusCode int, message string, err error) {
	resp := Response{
		Code:    statusCode,
		Success: false,
		Message: message,
	}

	if err != nil {
		resp.Error = err.Error()
	}

	WriteJSON(w, statusCode, resp)
}
