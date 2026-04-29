package errcode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	err := New(400, 40001, "invalid parameter")
	assert.Equal(t, "code=40001, message=invalid parameter", err.Error())
}

func TestAppError_WithMsg(t *testing.T) {
	original := New(400, 40001, "invalid parameter")
	modified := original.WithMsg("custom message")

	assert.Equal(t, "custom message", modified.Message)
	assert.Equal(t, 40001, modified.Code)
	assert.Equal(t, 400, modified.HTTPStatus)
	// 浅拷贝不影响原值
	assert.Equal(t, "invalid parameter", original.Message)
}

func TestAppError_WithMsgf(t *testing.T) {
	original := New(400, 60105, "amount out of range")
	modified := original.WithMsgf("amount must be between %s and %s", "10", "1000")

	assert.Equal(t, "amount must be between 10 and 1000", modified.Message)
	assert.Equal(t, 60105, modified.Code)
	assert.Equal(t, 400, modified.HTTPStatus)
	// 浅拷贝不影响原值
	assert.Equal(t, "amount out of range", original.Message)
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		httpStatus int
		code       int
	}{
		{"Success", Success, 200, 0},
		{"ErrParam", ErrParam, 400, 40001},
		{"ErrUnauthorized", ErrUnauthorized, 401, 40100},
		{"ErrInternal", ErrInternal, 500, 50000},
		{"ErrDatabase", ErrDatabase, 500, 50001},
		{"ErrRedis", ErrRedis, 500, 50002},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.httpStatus, tt.err.HTTPStatus)
			assert.Equal(t, tt.code, tt.err.Code)
		})
	}
}
