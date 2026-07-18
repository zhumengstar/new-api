package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func shouldFallbackResponsesConvertError(err error, request *dto.OpenAIResponsesRequest) bool {
	if err == nil || request == nil {
		return false
	}
	return shouldFallbackResponsesErrorText(err.Error(), request)
}

func shouldFallbackResponsesHTTPError(resp *http.Response, request *dto.OpenAIResponsesRequest) bool {
	if resp == nil || resp.Body == nil || request == nil {
		return false
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return shouldFallbackResponsesErrorText(string(data), request)
}

func shouldFallbackResponsesErrorText(text string, request *dto.OpenAIResponsesRequest) bool {
	if isResponsesUnsupportedErrorText(text) {
		return true
	}
	if request == nil || len(request.Input) == 0 || common.GetJsonType(request.Input) == "null" {
		return false
	}
	return strings.Contains(strings.ToLower(text), "contents is required")
}

func isResponsesUnsupportedErrorText(text string) bool {
	normalized := strings.ToLower(text)
	return strings.Contains(normalized, "not implemented") ||
		strings.Contains(normalized, "not support") ||
		strings.Contains(normalized, "unsupported endpoint") ||
		strings.Contains(normalized, "unsupported path") ||
		strings.Contains(normalized, "responses") && strings.Contains(normalized, "unsupported")
}

func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	chatReq, err := service.ResponsesRequestToChatCompletionsRequest(request)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	info.AppendRequestConversion(types.RelayFormatOpenAI)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	savedRequest := info.Request
	savedIsStream := info.IsStream
	savedFinalFormat := info.FinalRequestRelayFormat
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
		info.Request = savedRequest
		info.IsStream = savedIsStream
		info.FinalRequestRelayFormat = savedFinalFormat
	}()

	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"
	info.Request = chatReq
	info.IsStream = lo.FromPtrOr(request.Stream, false)
	info.FinalRequestRelayFormat = types.RelayFormatOpenAI

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, chatReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, "responses fallback chat request body: %s", jsonData)
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	jsonData = nil
	info.UpstreamRequestBodySize = size

	resp, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("empty upstream response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	httpResp := resp.(*http.Response)
	statusCodeMappingStr := c.GetString("status_code_mapping")
	if httpResp.StatusCode != http.StatusOK {
		logger.LogInfo(c, fmt.Sprintf("responses fallback upstream rejection: %s", responsesFallbackDiagnostic(request, convertedRequest)))
		newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return nil, newAPIError
	}
	if info.IsStream {
		openaichannel.InitResponsesFallbackStream(c, request)
		usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
		if newAPIError != nil {
			return nil, newAPIError
		}
		usageDto, ok := usage.(*dto.Usage)
		if !ok || usageDto == nil {
			return nil, types.NewError(fmt.Errorf("invalid streaming usage type %T", usage), types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
		}
		return usageDto, nil
	}
	defer service.CloseResponseBodyGracefully(httpResp)

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(data, &chatResp); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	responsesResp, usage, err := service.ChatCompletionsResponseToResponsesResponse(&chatResp, request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	out, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponse, types.ErrOptionWithSkipRetry())
	}
	c.Data(http.StatusOK, "application/json", out)
	return usage, nil
}
