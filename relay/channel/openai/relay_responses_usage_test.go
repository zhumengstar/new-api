package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAppendResponsesSnapshotDeduplicatesDoneEvent(t *testing.T) {
	var builder strings.Builder
	snapshots := make(map[string]string)

	appendResponsesSnapshot(&builder, snapshots, "output", "hello")
	appendResponsesSnapshot(&builder, snapshots, "output", "hello world")
	appendResponsesSnapshot(&builder, snapshots, "output", "hello world!")

	require.Equal(t, "hello world!", builder.String())
}

func TestRecordResponsesBuiltInToolCallCountsWebAndFileSearch(t *testing.T) {
	info := &relaycommon.RelayInfo{ResponsesUsageInfo: &relaycommon.ResponsesUsageInfo{
		BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
			dto.BuildInToolWebSearchPreview: {ToolName: dto.BuildInToolWebSearchPreview},
			dto.BuildInToolFileSearch:       {ToolName: dto.BuildInToolFileSearch},
		},
	}}

	recordResponsesBuiltInToolCall(info, dto.BuildInCallWebSearchCall)
	recordResponsesBuiltInToolCall(info, "file_search_call")

	require.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview].CallCount)
	require.Equal(t, 1, info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch].CallCount)
}

func TestOaiResponsesStreamHandlerEstimatesOutputEventsWhenUsageIsPartial(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 60
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-4.5"}}
	stream := strings.Join([]string{
		`data: {"type":"response.reasoning_summary_text.delta","delta":"thinking"}`,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","name":"get_status"}}`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{}"}`,
		`data: {"type":"response.output_text.delta","delta":"OK"}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":0,"total_tokens":10}}}`,
		`data: [DONE]`,
	}, "\n\n") + "\n\n"

	usage, apiErr := OaiResponsesStreamHandler(c, info, &http.Response{
		Body:    io.NopCloser(strings.NewReader(stream)),
		Request: httptest.NewRequest(http.MethodPost, "/v1/responses", nil),
	})

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Greater(t, usage.CompletionTokens, 0)
	require.Equal(t, "estimated_stream_output", usage.UsageSource)
}

func TestOaiResponsesStreamHandlerCorrectsOneTokenUsageWhenStreamContainsMoreOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitTokenEncoders()
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 60
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.6-sol"}}
	longOutput := strings.Repeat("reliable streamed output ", 32)
	stream := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"` + longOutput + `"}`,
		`data: {"type":"response.reasoning_summary_text.delta","delta":"thinking through the answer"}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":100,"output_tokens":1,"total_tokens":101}}}`,
		`data: [DONE]`,
	}, "\n\n") + "\n\n"

	usage, apiErr := OaiResponsesStreamHandler(c, info, &http.Response{
		Body:    io.NopCloser(strings.NewReader(stream)),
		Request: httptest.NewRequest(http.MethodPost, "/v1/responses", nil),
	})

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Greater(t, usage.CompletionTokens, 1)
	require.Equal(t, usage.CompletionTokens, usage.OutputTokens)
	require.Equal(t, "estimated_stream_output", usage.UsageSource)
	require.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
}

func TestOaiResponsesStreamHandlerKeepsGenuineOneTokenUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitTokenEncoders()
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 60
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.6-sol"}}
	stream := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"OK"}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11}}}`,
		`data: [DONE]`,
	}, "\n\n") + "\n\n"

	usage, apiErr := OaiResponsesStreamHandler(c, info, &http.Response{
		Body:    io.NopCloser(strings.NewReader(stream)),
		Request: httptest.NewRequest(http.MethodPost, "/v1/responses", nil),
	})

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 1, usage.CompletionTokens)
	require.Equal(t, "upstream_responses", usage.UsageSource)
}

func TestOaiResponsesStreamHandlerRejectsTrulyEmptyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 60
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-4.5"}}
	stream := "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":0,\"total_tokens\":10}}}\n\ndata: [DONE]\n\n"

	usage, apiErr := OaiResponsesStreamHandler(c, info, &http.Response{
		Body:    io.NopCloser(strings.NewReader(stream)),
		Request: httptest.NewRequest(http.MethodPost, "/v1/responses", nil),
	})

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "no output")
}

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

func TestResponsesUsageForBilling_EmptyUpstreamUsageKeepsEstimatedPrompt(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(321)
	upstream := &dto.Usage{}

	usage := responsesUsageForBilling(info, upstream, nil)

	require.Equal(t, 321, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
	require.Equal(t, 321, usage.TotalTokens)
	require.Equal(t, "upstream_responses", usage.UsageSource)
}
