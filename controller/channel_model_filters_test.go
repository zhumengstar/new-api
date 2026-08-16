package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestChannelModelFamilyMatchesPricingEditorRules(t *testing.T) {
	require.Equal(t, "OpenAI", getChannelModelFamily("gpt-5.2"))
	require.Equal(t, "Claude", getChannelModelFamily("claude-sonnet-5"))
	require.Equal(t, "Gemini", getChannelModelFamily("nano-banana-pro"))
	require.Equal(t, "Other", getChannelModelFamily("custom-model"))
}

func TestChannelModelTypeIsIndependentFromBillingType(t *testing.T) {
	pricing := map[string]model.Pricing{
		"imagen-4": {ModelName: "imagen-4", QuotaType: 1},
		"veo-3":    {ModelName: "veo-3", QuotaType: 0},
	}

	require.Equal(t, channelModelTypeImage, getChannelModelType("imagen-4"))
	require.Equal(t, channelBillingTypePerRequest, getChannelBillingType("imagen-4", pricing))
	require.Equal(t, channelModelTypeVideo, getChannelModelType("veo-3"))
	require.Equal(t, channelBillingTypePerToken, getChannelBillingType("veo-3", pricing))
	require.Equal(t, channelModelTypeText, getChannelModelType("claude-sonnet-5"))
}
