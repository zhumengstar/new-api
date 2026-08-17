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

func TestShouldRetryKeepsRecoverableUpstreamFailuresRetryable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	tests := []struct {
		name string
		err  *types.NewAPIError
	}{
		{
			name: "empty responses output",
			err:  types.NewOpenAIError(errors.New("upstream responses returned no output"), types.ErrorCodeBadResponse, http.StatusBadGateway),
		},
		{
			name: "upstream transport failure",
			err:  types.NewOpenAIError(errors.New("connection reset by peer"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError),
		},
		{
			name: "upstream malformed response",
			err:  types.NewOpenAIError(errors.New("invalid upstream body"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		},
		{
			name: "upstream rate limit",
			err:  types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests),
		},
		{
			name: "upstream payment required",
			err:  types.NewOpenAIError(errors.New("account balance is insufficient"), types.ErrorCodeBadResponseStatusCode, http.StatusPaymentRequired),
		},
		{
			name: "upstream forbidden with insufficient credit",
			err:  types.NewOpenAIError(errors.New("insufficient credit"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, shouldRetry(ctx, tt.err, 1))
		})
	}
}

func TestIsEmptyResponsesOutputError(t *testing.T) {
	actualEmptyOutput := types.NewOpenAIError(
		errors.New("upstream responses returned no output"),
		types.ErrorCodeBadResponse,
		http.StatusBadGateway,
	)
	otherBadGateway := types.NewOpenAIError(
		errors.New("upstream returned an invalid response"),
		types.ErrorCodeBadResponse,
		http.StatusBadGateway,
	)

	require.True(t, isEmptyResponsesOutputError(actualEmptyOutput))
	require.False(t, isEmptyResponsesOutputError(otherBadGateway))
}

func TestShouldRetryKeepsNonRecoverableUpstreamFailuresOnCurrentChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	tests := []struct {
		name string
		err  *types.NewAPIError
	}{
		{
			name: "invalid request",
			err:  types.NewOpenAIError(errors.New("invalid argument"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest),
		},
		{
			name: "content safety rejection",
			err:  types.NewOpenAIError(errors.New("unsafe prompt"), types.ErrorCodePromptBlocked, http.StatusUnavailableForLegalReasons),
		},
		{
			name: "model not found",
			err:  types.NewOpenAIError(errors.New("model not found"), types.ErrorCodeModelNotFound, http.StatusNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.False(t, shouldRetry(ctx, tt.err, 1))
		})
	}
}

func TestShouldRecordFinalRelayError(t *testing.T) {
	err := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	require.False(t, shouldRecordFinalRelayError(true, nil))
	require.False(t, shouldRecordFinalRelayError(false, err))
	require.True(t, shouldRecordFinalRelayError(true, err))
}

func TestShouldRetryDoesNotTreatUserQuotaAsChannelBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	err := types.NewErrorWithStatusCode(
		errors.New("用户额度不足"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)
	require.False(t, shouldRetry(ctx, err, 1))
}

func TestRetrySelectionFailurePreservesPreviousUpstreamError(t *testing.T) {
	upstreamErr := types.NewOpenAIError(
		errors.New("temporary upstream failure"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)
	selectionErr := types.NewError(
		errors.New("no alternative channel"),
		types.ErrorCodeGetChannelFailed,
		types.ErrOptionWithSkipRetry(),
	)

	actual := preservePreviousRelayError(upstreamErr, selectionErr)
	require.Same(t, upstreamErr, actual)
	require.Equal(t, http.StatusServiceUnavailable, actual.StatusCode)
}

func TestRetrySelectionFailureUsesSelectionErrorWithoutPreviousAttempt(t *testing.T) {
	selectionErr := types.NewError(
		errors.New("no available channel"),
		types.ErrorCodeGetChannelFailed,
		types.ErrOptionWithSkipRetry(),
	)

	require.Same(t, selectionErr, preservePreviousRelayError(nil, selectionErr))
}
