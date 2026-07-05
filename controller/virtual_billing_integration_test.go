package controller

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVirtualBillingUsesRealQuotaBeforeAdminFundedQuota(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.UserVirtualQuota{}))
	common.BatchUpdateEnabled = false

	admin := &model.User{
		Id:       4001,
		Username: "billing-admin",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Quota:    1000,
		Group:    "default",
		Setting:  `{"user_group_ratios":{"default":0.5}}`,
		AffCode:  "billing-admin-aff",
	}
	child := &model.User{
		Id:        4002,
		Username:  "billing-child",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Quota:     500,
		Group:     "default",
		InviterId: admin.Id,
		AffCode:   "billing-child-aff",
	}
	require.NoError(t, db.Create(admin).Error)
	require.NoError(t, db.Create(child).Error)
	require.NoError(t, model.SetUserVirtualQuota(admin.Id, child.Id, 800))

	newRelayInfo := func() *relaycommon.RelayInfo {
		now := time.Now()
		return &relaycommon.RelayInfo{
			UserId:            child.Id,
			UsingGroup:        "default",
			UserGroup:         "default",
			IsPlayground:      true,
			StartTime:         now,
			FirstResponseTime: now,
			UserSetting: dto.UserSetting{
				BillingPreference: "wallet_only",
			},
			PriceData: types.PriceData{
				GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
			},
			ChannelMeta: &relaycommon.ChannelMeta{},
		}
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	realInfo := newRelayInfo()
	realSession, apiErr := service.NewBillingSession(ctx, realInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, realSession.Settle(100))

	childAfterReal, err := model.GetUserById(child.Id, true)
	require.NoError(t, err)
	require.Equal(t, 400, childAfterReal.Quota)
	adminAfterReal, err := model.GetUserById(admin.Id, true)
	require.NoError(t, err)
	require.Equal(t, 1000, adminAfterReal.Quota)
	virtualAfterReal, err := model.GetUserVirtualQuota(child.Id)
	require.NoError(t, err)
	require.Equal(t, 0, virtualAfterReal.UsedQuota)
	require.False(t, realInfo.VirtualBilling)

	require.NoError(t, db.Model(&model.User{}).Where("id = ?", child.Id).Update("quota", 0).Error)
	virtualInfo := newRelayInfo()
	virtualSession, apiErr := service.NewBillingSession(ctx, virtualInfo, 100)
	require.Nil(t, apiErr)
	require.NoError(t, virtualSession.Settle(100))

	virtualAfterFallback, err := model.GetUserVirtualQuota(child.Id)
	require.NoError(t, err)
	require.Equal(t, 100, virtualAfterFallback.UsedQuota)
	adminAfterFallback, err := model.GetUserById(admin.Id, true)
	require.NoError(t, err)
	require.Equal(t, 950, adminAfterFallback.Quota)
	require.True(t, virtualInfo.VirtualBilling)

	other := service.GenerateTextOtherInfo(ctx, virtualInfo, 0, 1, 0, 0, 0, 0, 1)
	require.Equal(t, 1.0, other["virtual_group_ratio"])
	require.Equal(t, 0.5, other["actual_group_ratio"])
	require.Equal(t, 100, other["virtual_quota"])
	require.Equal(t, 50, other["actual_quota"])
}
