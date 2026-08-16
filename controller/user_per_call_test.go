package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildPerCallModelCatalogIncludesAllEnabledModelsInEligibleGroup(t *testing.T) {
	pricing := []model.Pricing{
		{
			ModelName:   "gemini-fixed-c",
			QuotaType:   1,
			ModelPrice:  0.005,
			EnableGroup: []string{"gemini-per-call"},
		},
		{
			ModelName:   "token-only-model",
			QuotaType:   0,
			ModelRatio:  1,
			EnableGroup: []string{"gemini-per-call"},
		},
	}
	abilities := []model.AbilityWithChannel{
		{Ability: model.Ability{Group: "gemini-per-call", Model: "gemini-fixed-c", Enabled: true}},
		{Ability: model.Ability{Group: "gemini-per-call", Model: "channel-model-without-global-price", Enabled: true}},
		{Ability: model.Ability{Group: "normal-token-group", Model: "token-only-model", Enabled: true}},
	}

	catalog := buildPerCallModelCatalog(pricing, abilities)
	require.Len(t, catalog, 2)
	require.Equal(t, "channel-model-without-global-price", catalog[0].Model)
	require.False(t, catalog[0].HasGlobalPrice)
	require.Equal(t, []string{"gemini-per-call"}, catalog[0].Groups)
	require.Equal(t, "gemini-fixed-c", catalog[1].Model)
	require.True(t, catalog[1].HasGlobalPrice)
	require.Equal(t, 0.005, catalog[1].Price)
}
