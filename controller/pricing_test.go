package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestApplyUserModelPriceRulesUsesGroupSpecificPrices(t *testing.T) {
	pricing := []model.Pricing{
		{
			ModelName:       "per-call-model",
			QuotaType:       1,
			ModelPrice:      0.2,
			ModelRatio:      0,
			CompletionRatio: 0,
			EnableGroup:     []string{"default", "premium"},
		},
	}
	setting := dto.UserSetting{
		UserModelPriceRules: []dto.UserModelPriceRule{
			{Group: "default", Models: []string{"per-call-model"}, Price: 0.5},
			{Group: "premium", Models: []string{"per-call-model"}, Price: 0.8},
		},
	}

	result := applyUserModelPriceRules(pricing, setting)
	require.Len(t, result, 1)
	require.Equal(t, map[string]float64{"default": 0.5, "premium": 0.8}, result[0].UserGroupPrices)
	require.Equal(t, 1, result[0].QuotaType)
	require.Equal(t, 1.0, result[0].ModelRatio)
	require.Equal(t, 1.0, result[0].CompletionRatio)
	require.Nil(t, pricing[0].UserGroupPrices)
}

func TestApplyUserModelPriceRulesDoesNotExposeUnavailableGroup(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "per-call-model", QuotaType: 1, EnableGroup: []string{"default"}},
	}
	setting := dto.UserSetting{
		UserModelPriceRules: []dto.UserModelPriceRule{
			{Group: "private", Models: []string{"per-call-model"}, Price: 0.8},
		},
	}

	result := applyUserModelPriceRules(pricing, setting)
	require.Nil(t, result[0].UserGroupPrices)
}
