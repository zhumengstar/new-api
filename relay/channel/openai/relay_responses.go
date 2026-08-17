package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := responsesUsageForBilling(info, responsesResponse.Usage, responseBody)
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return usage, nil
	}
	// 只统计上游实际返回的工具调用，不统计请求中声明但未调用的工具。
	for _, output := range responsesResponse.Output {
		recordResponsesBuiltInToolCall(info, output.Type)
	}
	return usage, nil
}

// responsesUsageForBilling converts the Responses API usage shape into NewAPI's
// internal billing shape without changing the response sent to the client.
//
// Some compatible upstreams expose visible output_tokens separately from hidden
// reasoning_tokens while including both in total_tokens. NewAPI settles quota
// from PromptTokens + CompletionTokens, so normalize against the authoritative
// upstream total to avoid silently dropping hidden reasoning usage.
func responsesUsageForBilling(info *relaycommon.RelayInfo, upstream *dto.Usage, responseBody []byte) *dto.Usage {
	usage := &dto.Usage{}
	if upstream != nil {
		usage.PromptTokens = upstream.InputTokens
		usage.CompletionTokens = upstream.OutputTokens
		usage.TotalTokens = upstream.TotalTokens
		usage.InputTokens = upstream.InputTokens
		usage.OutputTokens = upstream.OutputTokens
		usage.UsageSource = "upstream_responses"
		usage.PromptTokensDetails = upstream.PromptTokensDetails
		usage.CompletionTokenDetails = upstream.CompletionTokenDetails
		usage.InputTokensDetails = upstream.InputTokensDetails
		if upstream.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = upstream.InputTokensDetails.CachedTokens
		}

		// If total_tokens is unavailable, reasoning_tokens is the only safe
		// signal for hidden output. max(output, reasoning) avoids double
		// counting compliant upstreams where output_tokens already includes it.
		if usage.TotalTokens == 0 && usage.CompletionTokenDetails.ReasoningTokens > usage.CompletionTokens {
			usage.CompletionTokens = usage.CompletionTokenDetails.ReasoningTokens
		}
	}

	if usage.PromptTokens == 0 && info != nil {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	if usage.InputTokens == 0 && upstream != nil && upstream.InputTokens > 0 {
		usage.InputTokens = upstream.InputTokens
	}

	normalizeOpenAIUsageTotals(usage)
	applyUsagePostProcessing(info, usage, responseBody)
	normalizeOpenAIUsageTotals(usage)
	return usage
}

func recordResponsesBuiltInToolCall(info *relaycommon.RelayInfo, itemType string) {
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return
	}

	toolName := ""
	switch itemType {
	case dto.BuildInCallWebSearchCall:
		toolName = dto.BuildInToolWebSearchPreview
	case "file_search_call":
		toolName = dto.BuildInToolFileSearch
	}
	if tool, exists := info.ResponsesUsageInfo.BuiltInTools[toolName]; exists && tool != nil {
		tool.CallCount++
	}
}

func appendResponsesSnapshot(builder *strings.Builder, snapshots map[string]string, key string, value string) {
	if value == "" {
		return
	}
	previous := snapshots[key]
	if previous != "" && strings.HasPrefix(value, previous) {
		value = value[len(previous):]
	}
	if value != "" {
		builder.WriteString(value)
	}
	snapshots[key] = previous + value
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var usage = &dto.Usage{}
	var completedUsage *dto.Usage
	var completedResponseBody []byte
	var responseTextBuilder strings.Builder
	outputTextSnapshots := make(map[string]string)
	reasoningTextSnapshots := make(map[string]string)
	functionArgumentSnapshots := make(map[string]string)
	functionNames := make(map[string]bool)
	sawOutput := false

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		sendEvent := true
		switch streamResponse.Type {
		case "response.completed":
			if streamResponse.Response != nil {
				if len(streamResponse.Response.Output) > 0 {
					sawOutput = true
				}
				if streamResponse.Response.Usage != nil {
					completedUsage = streamResponse.Response.Usage
					completedResponseBody = common.StringToByteSlice(data)
				}
				if streamResponse.Response.HasImageGenerationCall() {
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
					c.Set("image_generation_call_size", streamResponse.Response.GetSize())
				}
				if !sawOutput && (streamResponse.Response.Usage == nil || (streamResponse.Response.Usage.OutputTokens == 0 && streamResponse.Response.Usage.TotalTokens <= streamResponse.Response.Usage.InputTokens)) {
					sendEvent = false
				}
			}
		case "response.output_text.delta":
			if streamResponse.Delta != "" {
				sawOutput = true
			}
			appendResponsesSnapshot(&responseTextBuilder, outputTextSnapshots, "output", streamResponse.Delta)
		case "response.output_text.done":
			if streamResponse.Text != "" {
				sawOutput = true
			}
			appendResponsesSnapshot(&responseTextBuilder, outputTextSnapshots, "output", streamResponse.Text)
		case "response.reasoning_text.delta", "response.reasoning_summary_text.delta":
			if streamResponse.Delta != "" {
				sawOutput = true
			}
			appendResponsesSnapshot(&responseTextBuilder, reasoningTextSnapshots, "reasoning", streamResponse.Delta)
		case "response.reasoning_text.done", "response.reasoning_summary_text.done":
			if streamResponse.Text != "" {
				sawOutput = true
			}
			appendResponsesSnapshot(&responseTextBuilder, reasoningTextSnapshots, "reasoning", streamResponse.Text)
		case "response.function_call_arguments.delta":
			if streamResponse.Delta != "" {
				sawOutput = true
			}
			callKey := streamResponse.ItemID
			if callKey == "" {
				callKey = "function_call"
			}
			appendResponsesSnapshot(&responseTextBuilder, functionArgumentSnapshots, callKey, functionArgumentSnapshots[callKey]+streamResponse.Delta)
		case "response.output_item.added", "response.output_item.done":
			if streamResponse.Item == nil {
				break
			}
			sawOutput = true
			if streamResponse.Item.Type == "function_call" {
				sawOutput = true
				callKey := streamResponse.Item.ID
				if callKey == "" {
					callKey = streamResponse.Item.CallId
				}
				if callKey == "" {
					callKey = "function_call"
				}
				if streamResponse.Item.Name != "" && !functionNames[callKey] {
					responseTextBuilder.WriteString(streamResponse.Item.Name)
					functionNames[callKey] = true
				}
				appendResponsesSnapshot(&responseTextBuilder, functionArgumentSnapshots, callKey, streamResponse.Item.ArgumentsString())
			}
			if streamResponse.Type == dto.ResponsesOutputTypeItemDone {
				recordResponsesBuiltInToolCall(info, streamResponse.Item.Type)
			}
		}
		if sendEvent {
			sendResponsesStreamData(c, streamResponse, data)
		}
	})

	if completedUsage != nil {
		usage = responsesUsageForBilling(info, completedUsage, completedResponseBody)
	}

	// Some compatible Responses upstreams send response.completed with an empty
	// or obviously underreported usage object. Count the output-bearing events
	// received here before persisting billing. Tool execution results arrive in a
	// later request and belong to that request's input tokens, so they are not
	// added here. A real one-token answer stays unchanged because its locally
	// counted content is also one token.
	streamOutput := responseTextBuilder.String()
	if len(streamOutput) > 0 && info != nil {
		estimatedOutputTokens := service.CountTextToken(streamOutput, info.UpstreamModelName)
		if usage.CompletionTokens == 0 || (usage.CompletionTokens <= 1 && estimatedOutputTokens > usage.CompletionTokens) {
			usage.CompletionTokens = estimatedOutputTokens
			usage.OutputTokens = estimatedOutputTokens
			usage.UsageSource = "estimated_stream_output"
		}
	}

	if usage.PromptTokens == 0 && info != nil {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	normalizeOpenAIUsageTotals(usage)
	if !sawOutput && usage.CompletionTokens == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("upstream responses returned no output"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}

	return usage, nil
}
