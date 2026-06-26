package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryRejectsUserSideFailuresEvenWhenStatusCodeWouldRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	oldRetryStatusCodes := operation_setting.AutomaticRetryStatusCodeRanges
	t.Cleanup(func() {
		operation_setting.AutomaticRetryStatusCodeRanges = oldRetryStatusCodes
	})
	operation_setting.AutomaticRetryStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 400, End: 499}, {Start: 500, End: 599}}

	tests := []struct {
		name string
		err  *types.NewAPIError
	}{
		{
			name: "invalid request body",
			err:  types.NewErrorWithStatusCode(errors.New("bad json"), types.ErrorCodeInvalidRequest, http.StatusBadRequest),
		},
		{
			name: "sensitive words",
			err:  types.NewErrorWithStatusCode(errors.New("sensitive"), types.ErrorCodeSensitiveWordsDetected, http.StatusBadRequest),
		},
		{
			name: "access denied",
			err:  types.NewErrorWithStatusCode(errors.New("access denied"), types.ErrorCodeAccessDenied, http.StatusForbidden),
		},
		{
			name: "bad request body",
			err:  types.NewErrorWithStatusCode(errors.New("bad body"), types.ErrorCodeBadRequestBody, http.StatusBadRequest),
		},
		{
			name: "insufficient user quota",
			err:  types.NewErrorWithStatusCode(errors.New("quota"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden),
		},
		{
			name: "prompt blocked",
			err:  types.NewErrorWithStatusCode(errors.New("blocked"), types.ErrorCodePromptBlocked, http.StatusForbidden),
		},
		{
			name: "model not found",
			err:  types.NewErrorWithStatusCode(errors.New("model"), types.ErrorCodeModelNotFound, http.StatusNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.False(t, shouldRetry(ctx, tt.err, 1))
		})
	}
}

func TestShouldRetryHonorsRetryBudgetBeforeChannelError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	err := types.NewErrorWithStatusCode(errors.New("channel invalid key"), types.ErrorCodeChannelInvalidKey, http.StatusUnauthorized)

	require.False(t, shouldRetry(ctx, err, 0))
}

func TestShouldRetryKeepsChannelTransientFailuresRetryable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	err := types.NewOpenAIError(errors.New("bad response status code 502"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)

	require.True(t, shouldRetry(ctx, err, 1))
}
