package response

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"

	"github.com/lib/pq"
	"github.com/mstgnz/gopay/infra/logger"
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
	_ = WriteJSON(w, statusCode, resp)
}

// Return writes a response with data
func Return(w http.ResponseWriter, statusCode int, success bool, message string, data any) {
	resp := Response{
		Code:    statusCode,
		Success: success,
		Message: message,
		Data:    data,
	}
	_ = WriteJSON(w, statusCode, resp)
}

// Error writes an error response
func Error(w http.ResponseWriter, statusCode int, message string, err error) {
	resp := Response{
		Code:    statusCode,
		Success: false,
		Message: message,
	}

	if err != nil {
		resp.Error = clientErrorText(err)
	}

	_ = WriteJSON(w, statusCode, resp)
}

// internalErrorText replaces error text a client must not see.
const internalErrorText = "internal error"

// clientErrorText is the error text a response may carry. A database driver error names tables,
// columns and constraints, so it goes to the log instead. Other errors keep their text: consumers
// read it (provider decline reasons, sovtajyeri's "not found").
func clientErrorText(err error) string {
	var dbErr *pq.Error
	if errors.As(err, &dbErr) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone) {
		logger.Error("database error kept out of a response", err, logger.LogContext{})
		return internalErrorText
	}
	return err.Error()
}
