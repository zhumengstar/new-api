package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateManagedUserGroupRatiosRequiresGreaterThanAdminRatio(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       3001,
		Username: "ratio-admin",
		Role:     common.RoleAdminUser,
		Group:    "default",
		Setting:  `{"user_group_ratios":{"default":0.5}}`,
		Status:   common.UserStatusEnabled,
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("id", 3001)
	ctx.Set("role", common.RoleAdminUser)

	err := validateManagedUserGroupRatios(ctx, map[string]float64{"default": 0.5})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be greater than admin ratio")

	require.NoError(t, validateManagedUserGroupRatios(ctx, map[string]float64{"default": 0.51}))
}

func TestAttachEffectiveGroupRatiosIncludesPublicOverride(t *testing.T) {
	users := []*model.User{{
		Id:      3002,
		Group:   "default",
		Setting: `{"user_group_ratios":{"vip":2.25}}`,
	}}

	attachEffectiveGroupRatios(users)

	require.Equal(t, 2.25, users[0].EffectiveGroupRatios["vip"])
	_, hasDefault := users[0].EffectiveGroupRatios["default"]
	require.True(t, hasDefault)
}
