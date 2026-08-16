package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCapacityUnavailableErrorRetriesNextChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	err := types.NewErrorWithStatusCode(
		errors.New("GPM 当前模型或账号池暂时无可用容量"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
		types.ErrOptionWithSkipRetry(),
	)

	require.True(t, shouldRetry(ctx, err, 1))
	ctx.Set("specific_channel_id", 123)
	require.False(t, shouldRetry(ctx, err, 1))
}

func TestCapacityErrorIsHiddenFromCommonUserOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	err := types.NewErrorWithStatusCode(
		errors.New("GPM 当前模型或账号池暂时无可用容量"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	commonCtx, _ := gin.CreateTestContext(nil)
	commonCtx.Set("role", common.RoleCommonUser)
	require.True(t, shouldHideCapacityError(commonCtx, err))

	adminCtx, _ := gin.CreateTestContext(nil)
	adminCtx.Set("role", common.RoleAdminUser)
	require.False(t, shouldHideCapacityError(adminCtx, err))
}
