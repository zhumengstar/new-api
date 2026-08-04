package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestResponsesUsageForBilling_UsesAuthoritativeTotalForHiddenReasoning(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	upstream := &dto.Usage{
		InputTokens:  100,
		OutputTokens: 8,
		TotalTokens:  133,
		CompletionTokenDetails: dto.OutputTokenDetails{
			ReasoningTokens: 25,
		},
	}

	usage := responsesUsageForBilling(info, upstream, nil)

	require.Equal(t, 100, usage.PromptTokens)
	require.Equal(t, 33, usage.CompletionTokens)
	require.Equal(t, 133, usage.TotalTokens)
	require.Equal(t, 25, usage.CompletionTokenDetails.ReasoningTokens)
	require.Equal(t, "upstream_responses", usage.UsageSource)
}

func TestResponsesUsageForBilling_DoesNotDoubleCountInclusiveOutput(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	upstream := &dto.Usage{
		InputTokens:  100,
		OutputTokens: 33,
		TotalTokens:  133,
		CompletionTokenDetails: dto.OutputTokenDetails{
			ReasoningTokens: 25,
		},
	}

	usage := responsesUsageForBilling(info, upstream, nil)

	require.Equal(t, 33, usage.CompletionTokens)
	require.Equal(t, 133, usage.TotalTokens)
}

func TestResponsesUsageForBilling_UsesReasoningWhenTotalMissing(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	upstream := &dto.Usage{
		InputTokens:  100,
		OutputTokens: 0,
		CompletionTokenDetails: dto.OutputTokenDetails{
			ReasoningTokens: 25,
		},
	}

	usage := responsesUsageForBilling(info, upstream, nil)

	require.Equal(t, 25, usage.CompletionTokens)
	require.Equal(t, 125, usage.TotalTokens)
}

func TestResponsesUsageForBilling_FallsBackToEstimatedPromptWithoutOutput(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(321)

	usage := responsesUsageForBilling(info, nil, nil)

	require.Equal(t, 321, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
	require.Equal(t, 321, usage.TotalTokens)
	require.Empty(t, usage.UsageSource)
}
