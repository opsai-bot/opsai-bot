package apierror

import "fmt"

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func New(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WithDetail(code int, message, detail string) *Error {
	return &Error{Code: code, Message: message, Detail: detail}
}

func NotFound(resource string) *Error {
	return New(404, fmt.Sprintf("%s not found", resource))
}

func BadRequest(message string) *Error {
	return New(400, message)
}

func Internal(message string) *Error {
	return New(500, message)
}

func Unauthorized(message string) *Error {
	return New(401, message)
}
