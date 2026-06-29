package service

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	maxLoggedRequestBodyBytes = 256 << 10
	maxLoggedFormFieldBytes   = 8 << 10
	maxLoggedJSONDepth        = 12
	maxLoggedJSONArrayItems   = 20
	maxLoggedJSONObjectKeys   = 80
)

func appendRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if other == nil {
		return
	}
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil {
		if path := ctx.Request.URL.Path; path != "" {
			other["request_path"] = path
			return
		}
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		path := relayInfo.RequestURLPath
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		other["request_path"] = path
	}
}

func appendRequestBodyInfo(ctx *gin.Context, other map[string]interface{}) {
	if ctx == nil || ctx.Request == nil || other == nil {
		return
	}
	storage, err := common.GetBodyStorage(ctx)
	if err != nil || storage == nil {
		return
	}
	defer func() {
		_, _ = storage.Seek(0, io.SeekStart)
	}()

	contentType := ctx.Request.Header.Get("Content-Type")
	size := storage.Size()
	info := map[string]interface{}{
		"method":       ctx.Request.Method,
		"content_type": contentType,
		"size":         size,
	}
	if ctx.Request.URL != nil {
		info["path"] = ctx.Request.URL.Path
	}

	mediaType, params, _ := mime.ParseMediaType(contentType)
	mediaType = strings.ToLower(mediaType)
	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		_, _ = storage.Seek(0, io.SeekStart)
		info["body"] = captureMultipartRequestBody(storage, params["boundary"])
	case mediaType == "application/x-www-form-urlencoded":
		if size > maxLoggedRequestBodyBytes {
			info["truncated"] = true
			break
		}
		if body, err := storage.Bytes(); err == nil {
			info["body"] = captureFormRequestBody(body)
		}
	case mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") || mediaType == "":
		if size > maxLoggedRequestBodyBytes {
			info["truncated"] = true
			break
		}
		if body, err := storage.Bytes(); err == nil {
			info["body"] = captureJSONRequestBody(body)
		}
	default:
		if size > maxLoggedRequestBodyBytes {
			info["truncated"] = true
			break
		}
		if body, err := storage.Bytes(); err == nil {
			info["body"] = truncateLoggedString(string(body), maxLoggedFormFieldBytes)
		}
	}

	other["request_body"] = info
}

func captureJSONRequestBody(body []byte) interface{} {
	var value interface{}
	if err := common.Unmarshal(body, &value); err == nil {
		return truncateLoggedJSONValue(value, 0)
	}
	return truncateLoggedString(string(body), maxLoggedFormFieldBytes)
}

func truncateLoggedJSONValue(value interface{}, depth int) interface{} {
	if depth >= maxLoggedJSONDepth {
		return "...(max depth exceeded)"
	}
	switch v := value.(type) {
	case string:
		return truncateLoggedString(v, maxLoggedFormFieldBytes)
	case []interface{}:
		limit := len(v)
		if limit > maxLoggedJSONArrayItems {
			limit = maxLoggedJSONArrayItems
		}
		result := make([]interface{}, 0, limit+1)
		for i := 0; i < limit; i++ {
			result = append(result, truncateLoggedJSONValue(v[i], depth+1))
		}
		if len(v) > limit {
			result = append(result, map[string]interface{}{
				"truncated_items": len(v) - limit,
			})
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		count := 0
		for key, item := range v {
			if count >= maxLoggedJSONObjectKeys {
				result["truncated_keys"] = len(v) - count
				break
			}
			result[key] = truncateLoggedJSONValue(item, depth+1)
			count++
		}
		return result
	default:
		return value
	}
}

func captureFormRequestBody(body []byte) map[string]interface{} {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return map[string]interface{}{
			"raw": truncateLoggedString(string(body), maxLoggedFormFieldBytes),
		}
	}
	result := make(map[string]interface{}, len(values))
	for key, vals := range values {
		if len(vals) == 1 {
			result[key] = truncateLoggedString(vals[0], maxLoggedFormFieldBytes)
			continue
		}
		items := make([]string, 0, len(vals))
		for _, val := range vals {
			items = append(items, truncateLoggedString(val, maxLoggedFormFieldBytes))
		}
		result[key] = items
	}
	return result
}

func captureMultipartRequestBody(reader io.Reader, boundary string) map[string]interface{} {
	result := map[string]interface{}{
		"fields": map[string]interface{}{},
		"files":  map[string][]map[string]interface{}{},
	}
	if boundary == "" {
		result["error"] = "missing multipart boundary"
		return result
	}
	mr := multipart.NewReader(reader, boundary)
	fields := result["fields"].(map[string]interface{})
	files := result["files"].(map[string][]map[string]interface{})
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			result["error"] = err.Error()
			break
		}
		name := part.FormName()
		if name == "" {
			continue
		}
		filename := part.FileName()
		if filename == "" {
			buf := new(bytes.Buffer)
			_, _ = io.Copy(buf, io.LimitReader(part, maxLoggedFormFieldBytes+1))
			value := truncateLoggedString(buf.String(), maxLoggedFormFieldBytes)
			if existing, ok := fields[name]; ok {
				switch v := existing.(type) {
				case []string:
					fields[name] = append(v, value)
				case string:
					fields[name] = []string{v, value}
				default:
					fields[name] = value
				}
			} else {
				fields[name] = value
			}
			continue
		}
		n, _ := io.Copy(io.Discard, part)
		files[name] = append(files[name], map[string]interface{}{
			"filename":     filename,
			"content_type": part.Header.Get("Content-Type"),
			"size":         n,
		})
	}
	return result
}

func truncateLoggedString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func GenerateTextOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64, modelPrice float64, userGroupRatio float64) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_ratio"] = modelRatio
	other["group_ratio"] = groupRatio
	other["completion_ratio"] = completionRatio
	other["cache_tokens"] = cacheTokens
	other["cache_ratio"] = cacheRatio
	other["model_price"] = modelPrice
	other["user_group_ratio"] = userGroupRatio
	other["frt"] = float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli())
	if relayInfo.ReasoningEffort != "" {
		other["reasoning_effort"] = relayInfo.ReasoningEffort
	}
	if relayInfo.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = relayInfo.UpstreamModelName
	}

	isSystemPromptOverwritten := common.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride)
	if isSystemPromptOverwritten {
		other["is_system_prompt_overwritten"] = true
	}

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = ctx.GetStringSlice("use_channel")
	isMultiKey := common.GetContextKeyBool(ctx, constant.ContextKeyChannelIsMultiKey)
	if isMultiKey {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex)
	}

	isLocalCountTokens := common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens)
	if isLocalCountTokens {
		adminInfo["local_count_tokens"] = isLocalCountTokens
	}

	AppendChannelAffinityAdminInfo(ctx, adminInfo)

	other["admin_info"] = adminInfo
	appendRequestPath(ctx, relayInfo, other)
	appendRequestBodyInfo(ctx, other)
	appendRequestConversionChain(relayInfo, other)
	appendFinalRequestFormat(relayInfo, other)
	appendBillingInfo(relayInfo, other)
	appendParamOverrideInfo(relayInfo, other)
	appendStreamStatus(relayInfo, other)
	return other
}

func appendParamOverrideInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || len(relayInfo.ParamOverrideAudit) == 0 {
		return
	}
	other["po"] = relayInfo.ParamOverrideAudit
}

func appendStreamStatus(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil || !relayInfo.IsStream || relayInfo.StreamStatus == nil {
		return
	}
	ss := relayInfo.StreamStatus
	status := "ok"
	if !ss.IsNormalEnd() || ss.HasErrors() {
		status = "error"
	}
	streamInfo := map[string]interface{}{
		"status":     status,
		"end_reason": string(ss.EndReason),
	}
	if ss.EndError != nil {
		streamInfo["end_error"] = ss.EndError.Error()
	}
	if ss.ErrorCount > 0 {
		streamInfo["error_count"] = ss.ErrorCount
		messages := make([]string, 0, len(ss.Errors))
		for _, e := range ss.Errors {
			messages = append(messages, e.Message)
		}
		streamInfo["errors"] = messages
	}
	other["stream_status"] = streamInfo
}

func appendBillingInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	// billing_source: "wallet" or "subscription"
	if relayInfo.BillingSource != "" {
		other["billing_source"] = relayInfo.BillingSource
	}
	if relayInfo.UserSetting.BillingPreference != "" {
		other["billing_preference"] = relayInfo.UserSetting.BillingPreference
	}
	if relayInfo.BillingSource == "subscription" {
		if relayInfo.SubscriptionId != 0 {
			other["subscription_id"] = relayInfo.SubscriptionId
		}
		if relayInfo.SubscriptionPreConsumed > 0 {
			other["subscription_pre_consumed"] = relayInfo.SubscriptionPreConsumed
		}
		// post_delta: settlement delta applied after actual usage is known (can be negative for refund)
		if relayInfo.SubscriptionPostDelta != 0 {
			other["subscription_post_delta"] = relayInfo.SubscriptionPostDelta
		}
		if relayInfo.SubscriptionPlanId != 0 {
			other["subscription_plan_id"] = relayInfo.SubscriptionPlanId
		}
		if relayInfo.SubscriptionPlanTitle != "" {
			other["subscription_plan_title"] = relayInfo.SubscriptionPlanTitle
		}
		// Compute "this request" subscription consumed + remaining
		consumed := relayInfo.SubscriptionPreConsumed + relayInfo.SubscriptionPostDelta
		usedFinal := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		if consumed < 0 {
			consumed = 0
		}
		if usedFinal < 0 {
			usedFinal = 0
		}
		if relayInfo.SubscriptionAmountTotal > 0 {
			remain := relayInfo.SubscriptionAmountTotal - usedFinal
			if remain < 0 {
				remain = 0
			}
			other["subscription_total"] = relayInfo.SubscriptionAmountTotal
			other["subscription_used"] = usedFinal
			other["subscription_remain"] = remain
		}
		if consumed > 0 {
			other["subscription_consumed"] = consumed
		}
		// Wallet quota is not deducted when billed from subscription.
		other["wallet_quota_deducted"] = 0
	}
}

func appendRequestConversionChain(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if len(relayInfo.RequestConversionChain) == 0 {
		return
	}
	chain := make([]string, 0, len(relayInfo.RequestConversionChain))
	for _, f := range relayInfo.RequestConversionChain {
		switch f {
		case types.RelayFormatOpenAI:
			chain = append(chain, "OpenAI Compatible")
		case types.RelayFormatClaude:
			chain = append(chain, "Claude Messages")
		case types.RelayFormatGemini:
			chain = append(chain, "Google Gemini")
		case types.RelayFormatOpenAIResponses:
			chain = append(chain, "OpenAI Responses")
		default:
			chain = append(chain, string(f))
		}
	}
	if len(chain) == 0 {
		return
	}
	other["request_conversion"] = chain
}

func appendFinalRequestFormat(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || other == nil {
		return
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		// claude indicates the final upstream request format is Claude Messages.
		// Frontend log rendering uses this to keep the original Claude input display.
		other["claude"] = true
	}
}

func GenerateWssOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["ws"] = true
	info["audio_input"] = usage.InputTokenDetails.AudioTokens
	info["audio_output"] = usage.OutputTokenDetails.AudioTokens
	info["text_input"] = usage.InputTokenDetails.TextTokens
	info["text_output"] = usage.OutputTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateAudioOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info["audio"] = true
	info["audio_input"] = usage.PromptTokensDetails.AudioTokens
	info["audio_output"] = usage.CompletionTokenDetails.AudioTokens
	info["text_input"] = usage.PromptTokensDetails.TextTokens
	info["text_output"] = usage.CompletionTokenDetails.TextTokens
	info["audio_ratio"] = audioRatio
	info["audio_completion_ratio"] = audioCompletionRatio
	return info
}

func GenerateClaudeOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64,
	cacheCreationTokens int, cacheCreationRatio float64,
	cacheCreationTokens5m int, cacheCreationRatio5m float64,
	cacheCreationTokens1h int, cacheCreationRatio1h float64,
	modelPrice float64, userGroupRatio float64) map[string]interface{} {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, cacheTokens, cacheRatio, modelPrice, userGroupRatio)
	info["claude"] = true
	info["cache_creation_tokens"] = cacheCreationTokens
	info["cache_creation_ratio"] = cacheCreationRatio
	if cacheCreationTokens5m != 0 {
		info["cache_creation_tokens_5m"] = cacheCreationTokens5m
		info["cache_creation_ratio_5m"] = cacheCreationRatio5m
	}
	if cacheCreationTokens1h != 0 {
		info["cache_creation_tokens_1h"] = cacheCreationTokens1h
		info["cache_creation_ratio_1h"] = cacheCreationRatio1h
	}
	return info
}

func GenerateMjOtherInfo(relayInfo *relaycommon.RelayInfo, priceData types.PriceData) map[string]interface{} {
	other := make(map[string]interface{})
	other["model_price"] = priceData.ModelPrice
	other["group_ratio"] = priceData.GroupRatioInfo.GroupRatio
	if priceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = priceData.GroupRatioInfo.GroupSpecialRatio
	}
	appendRequestPath(nil, relayInfo, other)
	return other
}

// InjectTieredBillingInfo overlays tiered billing fields onto an existing
// module-specific other map. Call this after GenerateTextOtherInfo /
// GenerateClaudeOtherInfo / etc. when the request used tiered_expr billing.
func InjectTieredBillingInfo(other map[string]interface{}, relayInfo *relaycommon.RelayInfo, result *billingexpr.TieredResult) {
	if relayInfo == nil || other == nil {
		return
	}
	snap := relayInfo.TieredBillingSnapshot
	if snap == nil {
		return
	}
	other["billing_mode"] = "tiered_expr"
	other["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
	if result != nil {
		other["matched_tier"] = result.MatchedTier
	}
}
