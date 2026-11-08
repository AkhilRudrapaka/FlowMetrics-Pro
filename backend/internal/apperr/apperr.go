// Package apperr defines the single error type that crosses layer boundaries.
// Services and repositories return *Error for anything the client should see;
// the HTTP layer renders it, and treats every other error as an opaque 500.
package apperr

import "net/http"

type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func BadRequest(msg string) *Error   { return New(http.StatusBadRequest, "bad_request", msg) }
func Unauthorized(msg string) *Error { return New(http.StatusUnauthorized, "unauthorized", msg) }
func Forbidden(msg string) *Error    { return New(http.StatusForbidden, "forbidden", msg) }
func Conflict(msg string) *Error     { return New(http.StatusConflict, "conflict", msg) }
func NotFound(what string) *Error    { return New(http.StatusNotFound, "not_found", what+" not found") }

func Validation(details map[string]string) *Error {
	e := New(http.StatusUnprocessableEntity, "validation_error", "one or more fields are invalid")
	e.Details = details
	return e
}
