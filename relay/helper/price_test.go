package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

func TestModelPriceHelperChannelTestAllowsUnconfiguredModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "definitely-unpriced-channel-test-model",
		UserId:          1,
		UserGroup:       "default",
		UsingGroup:      "default",
		IsChannelTest:   true,
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 37.5, priceData.ModelRatio)
	require.Equal(t, int(float64(common.PreConsumedQuota)*37.5), priceData.QuotaToPreConsume)
}

func TestModelPriceHelperPerCallChannelTestAllowsUnconfiguredModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "definitely-unpriced-per-call-channel-test-model",
		UserId:          1,
		UserGroup:       "default",
		UsingGroup:      "default",
		IsChannelTest:   true,
	}

	priceData, err := ModelPriceHelperPerCall(ctx, info)
	require.NoError(t, err)
	require.False(t, priceData.UsePrice)
	require.Equal(t, 37.5, priceData.ModelRatio)
	require.Equal(t, int(37.5/2*common.QuotaPerUnit), priceData.Quota)
}

func TestModelPriceHelperUsesUserPerCallPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "suno_music",
		UserGroup:       "default",
		UsingGroup:      "default",
		UserSetting: dto.UserSetting{
			UserModelPrices: map[string]float64{"suno_music": 0.42},
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, 0.42, priceData.ModelPrice)
	require.Equal(t, int(0.42*common.QuotaPerUnit), priceData.QuotaToPreConsume)
}

func TestModelPriceHelperUsesGroupSpecificUserPerCallPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "premium")

	info := &relaycommon.RelayInfo{
		OriginModelName: "suno_music",
		UserGroup:       "premium",
		UsingGroup:      "premium",
		UserSetting: dto.UserSetting{
			UserModelPriceRules: []dto.UserModelPriceRule{
				{Group: "default", Models: []string{"suno_music"}, Price: 0.25},
				{Group: "premium", Models: []string{"suno_music"}, Price: 0.8},
			},
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 0, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, 0.8, priceData.ModelPrice)
	require.Equal(t, 1.0, priceData.GroupRatioInfo.GroupRatio)
	require.Equal(t, int(0.8*common.QuotaPerUnit), priceData.QuotaToPreConsume)
}

func TestModelPriceHelperUsesUserRuleForModelWithoutGlobalPerCallPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "custom-per-call")

	info := &relaycommon.RelayInfo{
		OriginModelName: "channel-model-without-global-price",
		UserGroup:       "custom-per-call",
		UsingGroup:      "custom-per-call",
		UserSetting: dto.UserSetting{
			UserModelPriceRules: []dto.UserModelPriceRule{
				{Group: "custom-per-call", Models: []string{"channel-model-without-global-price"}, Price: 0.018},
			},
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, 0.018, priceData.ModelPrice)
	require.Equal(t, int(0.018*common.QuotaPerUnit), priceData.QuotaToPreConsume)
}

func TestModelPriceHelperPerCallUsesUserPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/task", nil)
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "suno_music",
		UserGroup:       "default",
		UsingGroup:      "default",
		UserSetting: dto.UserSetting{
			UserModelPrices: map[string]float64{"suno_music": 0.36},
		},
	}

	priceData, err := ModelPriceHelperPerCall(ctx, info)
	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.Equal(t, 0.36, priceData.ModelPrice)
	require.Equal(t, int(0.36*common.QuotaPerUnit), priceData.Quota)
}
