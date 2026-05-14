package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fixture struct mirrors the kind of request shape handlers accept.
type validateFixture struct {
	Runs        int     `json:"runs_per_variant" validate:"min=5,max=200"`
	Temperature float64 `json:"temperature" validate:"gte=0,lte=2"`
	Model       string  `json:"model" validate:"required"`
}

func TestValidate_NoErrorOnValidInput(t *testing.T) {
	fields := validate(validateFixture{Runs: 10, Temperature: 0.5, Model: "qwen"})
	if len(fields) != 0 {
		t.Fatalf("expected no field errors, got %+v", fields)
	}
}

func TestValidate_CollectsAllFieldErrors(t *testing.T) {
	fields := validate(validateFixture{Runs: 1, Temperature: 3.0, Model: ""})
	if len(fields) < 3 {
		t.Fatalf("expected ≥ 3 field errors, got %+v", fields)
	}

	byField := map[string]FieldError{}
	for _, fe := range fields {
		byField[fe.Field] = fe
	}
	if _, ok := byField["runs_per_variant"]; !ok {
		t.Error("runs_per_variant violation missing")
	}
	if _, ok := byField["temperature"]; !ok {
		t.Error("temperature violation missing")
	}
	if _, ok := byField["model"]; !ok {
		t.Error("model violation missing")
	}
}

func TestRenderError_EmitsStableShape(t *testing.T) {
	rec := httptest.NewRecorder()
	renderError(rec, context.Background(), http.StatusBadRequest, ErrCodeValidationFailed, "input validation failed", errors.New("internal: column 'foo' does not exist"))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", rec.Code)
	}

	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v (raw=%q)", err, rec.Body.String())
	}
	if body.Error != ErrCodeValidationFailed {
		t.Errorf("error code: got %q", body.Error)
	}
	if body.Message != "input validation failed" {
		t.Errorf("message: got %q", body.Message)
	}
	// The internal error string must NOT leak into the response.
	if strings.Contains(rec.Body.String(), "column 'foo'") {
		t.Error("internal error string leaked into response body")
	}
}

func TestRenderError_WithFieldsIncludesValidationDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	fields := []FieldError{{Field: "model", Rule: "required"}}
	renderValidationError(rec, context.Background(), fields)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", rec.Code)
	}
	var body struct {
		Error  string       `json:"error"`
		Fields []FieldError `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if body.Error != ErrCodeValidationFailed {
		t.Errorf("error code: got %q", body.Error)
	}
	if len(body.Fields) != 1 || body.Fields[0].Field != "model" {
		t.Errorf("fields not propagated: %+v", body.Fields)
	}
}

func TestDecodeAndValidate_RejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString("not json"))
	var target validateFixture
	if err := decodeAndValidate(req, &target); err == nil {
		t.Fatal("expected error on garbage JSON, got nil")
	}
}

func TestDecodeAndValidate_PassesValidPayload(t *testing.T) {
	body := `{"runs_per_variant":10,"temperature":0.5,"model":"qwen"}`
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	var target validateFixture
	if err := decodeAndValidate(req, &target); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	if target.Model != "qwen" {
		t.Errorf("decoded model: got %q", target.Model)
	}
}

func TestDecodeAndValidate_ReportsValidationErrors(t *testing.T) {
	body := `{"runs_per_variant":1,"temperature":5,"model":""}`
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	var target validateFixture
	err := decodeAndValidate(req, &target)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if len(verr.Fields) < 3 {
		t.Errorf("expected ≥ 3 field errors, got %d", len(verr.Fields))
	}
}
