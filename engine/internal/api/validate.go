package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/mustafaselman/frameval/engine/internal/logging"
)

// Stable error codes returned to clients. Keep this list small; every new
// code is a public contract surface. UI clients should branch on these
// rather than parse messages.
const (
	ErrCodeValidationFailed = "validation_failed"
	ErrCodeNotFound         = "not_found"
	ErrCodeConflict         = "conflict"
	ErrCodeForbidden        = "forbidden"
	ErrCodeBadRequest       = "bad_request"
	ErrCodeInternal         = "internal_error"
	ErrCodeGraderDown       = "grader_unavailable"
	ErrCodeRateLimited      = "rate_limited"
)

// FieldError is the per-field shape returned in a validation-failed
// response. JSON keys are stable; clients may use them to render
// per-field UI feedback.
type FieldError struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
	Param string `json:"param,omitempty"`
}

// ValidationError is a typed error returned by decodeAndValidate when a
// request body parses but fails struct-tag validation. Handlers `errors.As`
// it to distinguish from "JSON was syntactically invalid" errors.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, fe := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s:%s", fe.Field, fe.Rule))
	}
	return "validation failed: " + strings.Join(parts, ", ")
}

// Module-level validator. Safe for concurrent use per the library docs.
// The RegisterTagNameFunc swap makes field names in errors match the
// `json:"..."` tag (which is what clients see) rather than the Go field
// name.
var validatorInstance = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		tag := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if tag == "-" || tag == "" {
			return fld.Name
		}
		return tag
	})
	return v
}()

// validate runs validator over `target` and returns any field errors.
// Returns an empty slice when input is valid. Panics on a non-struct
// argument — handlers always pass struct types.
func validate(target any) []FieldError {
	err := validatorInstance.Struct(target)
	if err == nil {
		return nil
	}
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		// Anything else (e.g., *InvalidValidationError on non-struct) is a
		// developer bug. Surface as a single synthetic field error so the
		// HTTP layer still returns something structured rather than panicking.
		return []FieldError{{Field: "_validator", Rule: "internal"}}
	}
	out := make([]FieldError, 0, len(ve))
	for _, fe := range ve {
		out = append(out, FieldError{
			Field: fe.Field(),
			Rule:  fe.Tag(),
			Param: fe.Param(),
		})
	}
	return out
}

// decodeAndValidate parses the request body into target and runs
// struct-tag validation. Returns either a JSON-decode error or a
// *ValidationError; nil on success.
func decodeAndValidate(r *http.Request, target any) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	if fields := validate(target); len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// errorResponse is the wire shape of every error response.
type errorResponse struct {
	Error   string       `json:"error"`
	Message string       `json:"message,omitempty"`
	Fields  []FieldError `json:"fields,omitempty"`
}

// renderError writes a structured JSON error response and logs the
// internal error (if any) with the request's trace_id. The publicMessage
// is what the client sees; the internalErr is what operators see in logs.
//
// Log level is picked from the HTTP status: 5xx → ERROR (this is an
// engine bug or downstream failure we need to investigate), 4xx → WARN
// (the client did something wrong; routine but worth visibility).
// internalErr == nil skips the log emission entirely — when there's no
// extra context to surface, the response itself is the only signal worth
// emitting.
func renderError(w http.ResponseWriter, ctx context.Context, status int, code, publicMessage string, internalErr error) {
	if internalErr != nil {
		logger := logging.FromContext(ctx, nil)
		attrs := []any{"code", code, "status", status, "err", internalErr}
		if status >= 500 {
			logger.Error("api_error", attrs...)
		} else {
			logger.Warn("api_error", attrs...)
		}
	}
	JSON(w, status, errorResponse{Error: code, Message: publicMessage})
}

// renderValidationError is the canonical helper for "client sent bad
// input" cases: 400 status, validation_failed code, the field list
// echoed back, and no internal-error log emission (validation failures
// are routine and not worth WARN-level noise).
func renderValidationError(w http.ResponseWriter, _ context.Context, fields []FieldError) {
	JSON(w, http.StatusBadRequest, errorResponse{
		Error:   ErrCodeValidationFailed,
		Message: "input validation failed",
		Fields:  fields,
	})
}
