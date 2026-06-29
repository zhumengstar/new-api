package openai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/ai360"
	"github.com/QuantumNous/new-api/relay/channel/lingyiwanwu"

	//"github.com/QuantumNous/new-api/relay/channel/minimax"
	"github.com/QuantumNous/new-api/relay/channel/openrouter"
	"github.com/QuantumNous/new-api/relay/channel/xinference"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/common_handler"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	ChannelType    int
	ResponseFormat string
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	// 使用 service.GeminiToOpenAIRequest 转换请求格式
	openaiRequest, err := service.GeminiToOpenAIRequest(request, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, openaiRequest)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	//if !strings.Contains(request.Model, "claude") {
	//	return nil, fmt.Errorf("you are using openai channel type with path /v1/messages, only claude model supported convert, but got %s", request.Model)
	//}
	//if common.DebugEnabled {
	//	bodyBytes := []byte(common.GetJsonString(request))
	//	err := os.WriteFile(fmt.Sprintf("claude_request_%s.txt", c.GetString(common.RequestIdKey)), bodyBytes, 0644)
	//	if err != nil {
	//		println(fmt.Sprintf("failed to save request body to file: %v", err))
	//	}
	//}
	aiRequest, err := service.ClaudeToOpenAIRequest(*request, info)
	if err != nil {
		return nil, err
	}
	//if common.DebugEnabled {
	//	println(fmt.Sprintf("convert claude to openai request result: %s", common.GetJsonString(aiRequest)))
	//	// Save request body to file for debugging
	//	bodyBytes := []byte(common.GetJsonString(aiRequest))
	//	err = os.WriteFile(fmt.Sprintf("claude_to_openai_request_%s.txt", c.GetString(common.RequestIdKey)), bodyBytes, 0644)
	//	if err != nil {
	//		println(fmt.Sprintf("failed to save request body to file: %v", err))
	//	}
	//}
	if info.SupportStreamOptions && info.IsStream {
		aiRequest.StreamOptions = &dto.StreamOptions{
			IncludeUsage: true,
		}
	}
	return a.ConvertOpenAIRequest(c, info, aiRequest)
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType

	// initialize ThinkingContentInfo when thinking_to_content is enabled
	if info.ChannelSetting.ThinkingToContent {
		info.ThinkingContentInfo = relaycommon.ThinkingContentInfo{
			IsFirstThinkingContent:  true,
			SendLastThinkingContent: false,
			HasSentThinkingContent:  false,
		}
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if shouldUseGeminiImageChatCompatibility(info) {
		return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
	}
	if info.RelayMode == relayconstant.RelayModeRealtime {
		if strings.HasPrefix(info.ChannelBaseUrl, "https://") {
			baseUrl := strings.TrimPrefix(info.ChannelBaseUrl, "https://")
			baseUrl = "wss://" + baseUrl
			info.ChannelBaseUrl = baseUrl
		} else if strings.HasPrefix(info.ChannelBaseUrl, "http://") {
			baseUrl := strings.TrimPrefix(info.ChannelBaseUrl, "http://")
			baseUrl = "ws://" + baseUrl
			info.ChannelBaseUrl = baseUrl
		}
	}
	switch info.ChannelType {
	case constant.ChannelTypeAzure:
		apiVersion := info.ApiVersion
		if apiVersion == "" {
			apiVersion = constant.AzureDefaultAPIVersion
		}
		// https://learn.microsoft.com/en-us/azure/cognitive-services/openai/chatgpt-quickstart?pivots=rest-api&tabs=command-line#rest-api
		requestURL := strings.Split(info.RequestURLPath, "?")[0]
		requestURL = fmt.Sprintf("%s?api-version=%s", requestURL, apiVersion)
		task := strings.TrimPrefix(requestURL, "/v1/")

		if info.RelayFormat == types.RelayFormatClaude {
			task = strings.TrimPrefix(task, "messages")
			task = "chat/completions" + task
		}

		// 特殊处理 responses API（包含 compact）
		if info.RelayMode == relayconstant.RelayModeResponses || info.RelayMode == relayconstant.RelayModeResponsesCompact {
			responsesApiVersion := "preview"

			subUrl := "/openai/v1/responses"
			if strings.Contains(info.ChannelBaseUrl, "cognitiveservices.azure.com") {
				subUrl = "/openai/responses"
				responsesApiVersion = apiVersion
			}

			if info.ChannelOtherSettings.AzureResponsesVersion != "" {
				responsesApiVersion = info.ChannelOtherSettings.AzureResponsesVersion
			}

			// compact 模式追加 /compact
			if info.RelayMode == relayconstant.RelayModeResponsesCompact {
				subUrl = subUrl + "/compact"
			}

			requestURL = fmt.Sprintf("%s?api-version=%s", subUrl, responsesApiVersion)
			return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestURL, info.ChannelType), nil
		}

		model_ := info.UpstreamModelName
		// 2025年5月10日后创建的渠道不移除.
		if info.ChannelCreateTime < constant.AzureNoRemoveDotTime {
			model_ = strings.Replace(model_, ".", "", -1)
		}
		// https://github.com/songquanpeng/one-api/issues/67
		requestURL = fmt.Sprintf("/openai/deployments/%s/%s", model_, task)
		if info.RelayMode == relayconstant.RelayModeRealtime {
			requestURL = fmt.Sprintf("/openai/realtime?deployment=%s&api-version=%s", model_, apiVersion)
		}
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, requestURL, info.ChannelType), nil
	//case constant.ChannelTypeMiniMax:
	//	return minimax.GetRequestURL(info)
	case constant.ChannelTypeCustom:
		url := info.ChannelBaseUrl
		url = strings.Replace(url, "{model}", info.UpstreamModelName, -1)
		return url, nil
	default:
		if (info.RelayFormat == types.RelayFormatClaude || info.RelayFormat == types.RelayFormatGemini) &&
			info.RelayMode != relayconstant.RelayModeResponses &&
			info.RelayMode != relayconstant.RelayModeResponsesCompact {
			return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
		}
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	if info.ChannelType == constant.ChannelTypeAzure {
		header.Set("api-key", info.ApiKey)
		return nil
	}
	if info.ChannelType == constant.ChannelTypeOpenAI && "" != info.Organization {
		header.Set("OpenAI-Organization", info.Organization)
	}
	// 检查 Header Override 是否已设置 Authorization，如果已设置则跳过默认设置
	// 这样可以避免在 Header Override 应用时被覆盖（虽然 Header Override 会在之后应用，但这里作为额外保护）
	hasAuthOverride := false
	if len(info.HeadersOverride) > 0 {
		for k := range info.HeadersOverride {
			if strings.EqualFold(k, "Authorization") {
				hasAuthOverride = true
				break
			}
		}
	}
	if info.RelayMode == relayconstant.RelayModeRealtime {
		swp := c.Request.Header.Get("Sec-WebSocket-Protocol")
		if swp != "" {
			items := []string{
				"realtime",
				"openai-insecure-api-key." + info.ApiKey,
				"openai-beta.realtime-v1",
			}
			header.Set("Sec-WebSocket-Protocol", strings.Join(items, ","))
			//req.Header.Set("Sec-WebSocket-Key", c.Request.Header.Get("Sec-WebSocket-Key"))
			//req.Header.Set("Sec-Websocket-Extensions", c.Request.Header.Get("Sec-Websocket-Extensions"))
			//req.Header.Set("Sec-Websocket-Version", c.Request.Header.Get("Sec-Websocket-Version"))
		} else {
			header.Set("openai-beta", "realtime=v1")
			if !hasAuthOverride {
				header.Set("Authorization", "Bearer "+info.ApiKey)
			}
		}
	} else {
		if !hasAuthOverride {
			header.Set("Authorization", "Bearer "+info.ApiKey)
		}
	}
	if info.ChannelType == constant.ChannelTypeOpenRouter {
		if header.Get("HTTP-Referer") == "" {
			header.Set("HTTP-Referer", "https://www.newapi.ai")
		}
		if header.Get("X-OpenRouter-Title") == "" {
			header.Set("X-OpenRouter-Title", "New API")
		}
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if info.ChannelType != constant.ChannelTypeOpenAI && info.ChannelType != constant.ChannelTypeAzure {
		request.StreamOptions = nil
	}
	if info.ChannelType == constant.ChannelTypeOpenRouter {
		if len(request.Usage) == 0 {
			request.Usage = json.RawMessage(`{"include":true}`)
		}
		// 适配 OpenRouter 的 thinking 后缀
		if !model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) &&
			strings.HasSuffix(info.UpstreamModelName, "-thinking") {
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
			request.Model = info.UpstreamModelName
			if len(request.Reasoning) == 0 {
				reasoning := map[string]any{
					"enabled": true,
				}
				if request.ReasoningEffort != "" && request.ReasoningEffort != "none" {
					reasoning["effort"] = request.ReasoningEffort
				}
				marshal, err := common.Marshal(reasoning)
				if err != nil {
					return nil, fmt.Errorf("error marshalling reasoning: %w", err)
				}
				request.Reasoning = marshal
			}
			// 清空多余的ReasoningEffort
			request.ReasoningEffort = ""
		} else {
			if len(request.Reasoning) == 0 {
				// 适配 OpenAI 的 ReasoningEffort 格式
				if request.ReasoningEffort != "" {
					reasoning := map[string]any{
						"enabled": true,
					}
					if request.ReasoningEffort != "none" {
						reasoning["effort"] = request.ReasoningEffort
						marshal, err := common.Marshal(reasoning)
						if err != nil {
							return nil, fmt.Errorf("error marshalling reasoning: %w", err)
						}
						request.Reasoning = marshal
					}
				}
			}
			request.ReasoningEffort = ""
		}

		// https://docs.anthropic.com/en/api/openai-sdk#extended-thinking-support
		// 没有做排除3.5Haiku等，要出问题再加吧，最佳兼容性（不是
		if request.THINKING != nil && strings.HasPrefix(info.UpstreamModelName, "anthropic") {
			var thinking dto.Thinking // Claude标准Thinking格式
			if err := json.Unmarshal(request.THINKING, &thinking); err != nil {
				return nil, fmt.Errorf("error Unmarshal thinking: %w", err)
			}

			// 只有当 thinking.Type 是 "enabled" 时才处理
			if thinking.Type == "enabled" {
				// 检查 BudgetTokens 是否为 nil
				if thinking.BudgetTokens == nil {
					return nil, fmt.Errorf("BudgetTokens is nil when thinking is enabled")
				}

				reasoning := openrouter.RequestReasoning{
					Enabled:   true,
					MaxTokens: *thinking.BudgetTokens,
				}

				marshal, err := common.Marshal(reasoning)
				if err != nil {
					return nil, fmt.Errorf("error marshalling reasoning: %w", err)
				}

				request.Reasoning = marshal
			}

			// 清空 THINKING
			request.THINKING = nil
		}

	}
	if strings.HasPrefix(info.UpstreamModelName, "o") || strings.HasPrefix(info.UpstreamModelName, "gpt-5") {
		if lo.FromPtrOr(request.MaxCompletionTokens, uint(0)) == 0 && lo.FromPtrOr(request.MaxTokens, uint(0)) != 0 {
			request.MaxCompletionTokens = request.MaxTokens
			request.MaxTokens = nil
		}

		if strings.HasPrefix(info.UpstreamModelName, "o") {
			request.Temperature = nil
		}

		// gpt-5系列模型适配 归零不再支持的参数
		if strings.HasPrefix(info.UpstreamModelName, "gpt-5") {
			request.Temperature = nil
			request.TopP = nil
			request.LogProbs = nil
		}

		// 转换模型推理力度后缀
		effort, originModel := reasoning.ParseOpenAIReasoningEffortFromModelSuffix(info.UpstreamModelName)
		if effort != "" {
			request.ReasoningEffort = effort
			info.UpstreamModelName = originModel
			request.Model = originModel
		}

		info.ReasoningEffort = request.ReasoningEffort

		// o系列模型developer适配（o1-mini除外）
		if !strings.HasPrefix(info.UpstreamModelName, "o1-mini") && !strings.HasPrefix(info.UpstreamModelName, "o1-preview") {
			//修改第一个Message的内容，将system改为developer
			if len(request.Messages) > 0 && request.Messages[0].Role == "system" {
				request.Messages[0].Role = "developer"
			}
		}
	}

	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	a.ResponseFormat = request.ResponseFormat
	if info.RelayMode == relayconstant.RelayModeAudioSpeech {
		jsonData, err := common.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("error marshalling object: %w", err)
		}
		return bytes.NewReader(jsonData), nil
	} else {
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		writer.WriteField("model", request.Model)

		formData, err2 := common.ParseMultipartFormReusable(c)
		if err2 != nil {
			return nil, fmt.Errorf("error parsing multipart form: %w", err2)
		}

		// 打印类似 curl 命令格式的信息
		logger.LogDebug(c.Request.Context(), "--form 'model=\"%s\"'", request.Model)

		// 遍历表单字段并打印输出
		for key, values := range formData.Value {
			if key == "model" {
				continue
			}
			for _, value := range values {
				writer.WriteField(key, value)
				logger.LogDebug(c.Request.Context(), "--form '%s=\"%s\"'", key, value)
			}
		}

		// 从 formData 中获取文件
		fileHeaders := formData.File["file"]
		if len(fileHeaders) == 0 {
			return nil, errors.New("file is required")
		}

		// 使用 formData 中的第一个文件
		fileHeader := fileHeaders[0]
		logger.LogDebug(c.Request.Context(), "--form 'file=@\"%s\"' (size: %d bytes, content-type: %s)",
			fileHeader.Filename, fileHeader.Size, fileHeader.Header.Get("Content-Type"))

		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("error opening audio file: %v", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file", fileHeader.Filename)
		if err != nil {
			return nil, errors.New("create form file failed")
		}
		if _, err := io.Copy(part, file); err != nil {
			return nil, errors.New("copy file failed")
		}

		// 关闭 multipart 编写器以设置分界线
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		logger.LogDebug(c.Request.Context(), "--header 'Content-Type: %s'", writer.FormDataContentType())
		return &requestBody, nil
	}
}

func shouldUseGeminiImageChatCompatibility(info *relaycommon.RelayInfo) bool {
	if info == nil || info.ChannelType != constant.ChannelTypeOpenAI {
		return false
	}
	if info.RelayMode != relayconstant.RelayModeImagesGenerations && info.RelayMode != relayconstant.RelayModeImagesEdits {
		return false
	}
	model := strings.ToLower(strings.TrimSpace(info.UpstreamModelName))
	return strings.HasPrefix(model, "gemini-") && strings.Contains(model, "image")
}

func convertImageRequestToGeminiImageChat(c *gin.Context, request dto.ImageRequest, info *relaycommon.RelayInfo) (dto.GeneralOpenAIRequest, error) {
	prompt := imagePromptWithRequestConstraints(request, info)

	message := dto.Message{Role: "user"}
	if info.RelayMode == relayconstant.RelayModeImagesEdits && !isJSONRequest(c) {
		mediaContents := []dto.MediaContent{{Type: dto.ContentTypeText, Text: prompt}}
		imageContents, err := geminiImageEditMediaContents(c)
		if err != nil {
			return dto.GeneralOpenAIRequest{}, err
		}
		mediaContents = append(mediaContents, imageContents...)
		message.SetMediaContent(mediaContents)
	} else {
		message.SetStringContent(prompt)
	}

	openAIRequest := dto.GeneralOpenAIRequest{
		Model: info.UpstreamModelName,
		Messages: []dto.Message{
			message,
		},
	}
	applyGeminiImageNativeSizeFields(&openAIRequest, request, info)
	if request.Extra != nil {
		openAIRequest.ExtraBody = request.Extra["extra_body"]
	}
	if err := applyGeminiImageConfig(&openAIRequest, request, info); err != nil {
		return dto.GeneralOpenAIRequest{}, err
	}
	return openAIRequest, nil
}

func applyGeminiImageNativeSizeFields(openAIRequest *dto.GeneralOpenAIRequest, request dto.ImageRequest, info *relaycommon.RelayInfo) {
	if openAIRequest == nil || info == nil || !shouldUseGeminiImageChatCompatibility(info) {
		return
	}
	size := strings.TrimSpace(request.Size)
	if size != "" {
		openAIRequest.Size = size
	}
	if aspectRatio := geminiImageAspectRatioFromSize(request.Size); aspectRatio != "" {
		openAIRequest.AspectRatio = aspectRatio
	}
	if imageSize := geminiImageSizeFromRequest(request, info); imageSize != "" {
		openAIRequest.ImageSize = imageSize
	}
}

func applyGeminiImageConfig(openAIRequest *dto.GeneralOpenAIRequest, request dto.ImageRequest, info *relaycommon.RelayInfo) error {
	if info == nil || (info.ChannelType != constant.ChannelTypeGemini && info.ChannelType != constant.ChannelTypeVertexAi && !shouldUseGeminiImageChatCompatibility(info)) {
		return nil
	}

	imageConfig := make(map[string]interface{})
	aspectRatio := geminiImageAspectRatioFromSize(request.Size)
	if aspectRatio != "" {
		imageConfig["aspect_ratio"] = aspectRatio
	}
	imageSize := geminiImageSizeFromRequest(request, info)
	if imageSize != "" {
		imageConfig["image_size"] = imageSize
	}
	if len(imageConfig) == 0 {
		return nil
	}
	if shouldUseGeminiImageChatCompatibility(info) {
		if openAIRequest.ImageConfig == nil {
			openAIRequest.ImageConfig = make(map[string]any)
		}
		for key, value := range imageConfig {
			if _, exists := openAIRequest.ImageConfig[key]; !exists {
				openAIRequest.ImageConfig[key] = value
			}
		}
	}

	extraBody := make(map[string]interface{})
	if len(openAIRequest.ExtraBody) > 0 {
		if err := common.Unmarshal(openAIRequest.ExtraBody, &extraBody); err != nil {
			return fmt.Errorf("invalid extra body: %w", err)
		}
	}

	googleBody, _ := extraBody["google"].(map[string]interface{})
	if googleBody == nil {
		googleBody = make(map[string]interface{})
		extraBody["google"] = googleBody
	}

	userImageConfig, _ := googleBody["image_config"].(map[string]interface{})
	if userImageConfig == nil {
		userImageConfig = make(map[string]interface{})
		googleBody["image_config"] = userImageConfig
	}

	for key, value := range imageConfig {
		if _, exists := userImageConfig[key]; !exists {
			userImageConfig[key] = value
		}
	}
	if size := strings.TrimSpace(request.Size); size != "" {
		if _, exists := extraBody["size"]; !exists {
			extraBody["size"] = size
		}
	}
	if aspectRatio != "" {
		if _, exists := extraBody["aspect_ratio"]; !exists {
			extraBody["aspect_ratio"] = aspectRatio
		}
	}
	if imageSize != "" {
		if _, exists := extraBody["image_size"]; !exists {
			extraBody["image_size"] = imageSize
		}
	}

	extraBodyBytes, err := common.Marshal(extraBody)
	if err != nil {
		return fmt.Errorf("failed to marshal gemini image_config: %w", err)
	}
	openAIRequest.ExtraBody = extraBodyBytes
	return nil
}

func imagePromptWithRequestConstraints(request dto.ImageRequest, info *relaycommon.RelayInfo) string {
	prompt := strings.TrimSpace(request.Prompt)
	constraints := imageRequestConstraintLines(request, info)
	if len(constraints) == 0 {
		return prompt
	}
	return prompt + "\n\nImage generation constraints:\n- " + strings.Join(constraints, "\n- ")
}

func imageRequestConstraintLines(request dto.ImageRequest, info *relaycommon.RelayInfo) []string {
	lines := make([]string, 0, 8)

	if size := strings.TrimSpace(request.Size); size != "" {
		lines = append(lines, fmt.Sprintf("Requested output size: %s pixels.", size))
		if aspectRatio := geminiImageAspectRatioFromSize(size); aspectRatio != "" {
			lines = append(lines, fmt.Sprintf("Use aspect ratio %s.", aspectRatio))
		}
		if instruction := geminiImageOrientationInstruction(size); instruction != "" {
			lines = append(lines, instruction)
		}
		if imageSize := geminiImageSizeFromRequest(request, info); imageSize != "" {
			lines = append(lines, fmt.Sprintf("Target resolution tier: %s. Preserve the requested aspect ratio and avoid changing width/height orientation.", imageSize))
		}
	}

	if quality := strings.TrimSpace(request.Quality); quality != "" {
		lines = append(lines, fmt.Sprintf("Requested quality: %s.", quality))
	}
	if request.N != nil && *request.N > 0 {
		lines = append(lines, fmt.Sprintf("Requested image count: %d.", *request.N))
	}
	if format := strings.TrimSpace(request.ResponseFormat); format != "" {
		lines = append(lines, fmt.Sprintf("Requested response format: %s.", format))
	}
	if style := imageRawMessageString(request.Style); style != "" {
		lines = append(lines, fmt.Sprintf("Requested style: %s.", style))
	}
	if background := imageRawMessageString(request.Background); background != "" {
		lines = append(lines, fmt.Sprintf("Requested background: %s.", background))
	}
	if outputFormat := imageRawMessageString(request.OutputFormat); outputFormat != "" {
		lines = append(lines, fmt.Sprintf("Requested output format: %s.", outputFormat))
	}
	if compression := imageRawMessageString(request.OutputCompression); compression != "" {
		lines = append(lines, fmt.Sprintf("Requested output compression: %s.", compression))
	}

	return lines
}

func imageRawMessageString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := common.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(raw))
}

func geminiImageOrientationInstruction(size string) string {
	size = strings.TrimSpace(size)
	if size == "" || strings.Contains(size, ":") {
		if aspectRatio := closestGeminiImageAspectRatio(size); aspectRatio != "" {
			return geminiImageAspectRatioInstruction(aspectRatio)
		}
		return ""
	}
	normalized := strings.ToLower(size)
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return ""
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return ""
	}
	aspectRatio := geminiImageAspectRatioFromSize(size)
	if aspectRatio == "" {
		return ""
	}
	orientation := "square"
	if width < height {
		orientation = "portrait"
	} else if width > height {
		orientation = "landscape"
	}
	return fmt.Sprintf("Use a %s canvas with aspect ratio %s. Treat %d as width and %d as height; do not swap width and height.", orientation, aspectRatio, width, height)
}

func geminiImageAspectRatioInstruction(aspectRatio string) string {
	width, height, ok := parseAspectRatio(aspectRatio)
	if !ok {
		return ""
	}
	orientation := "square"
	if width < height {
		orientation = "portrait"
	} else if width > height {
		orientation = "landscape"
	}
	return fmt.Sprintf("Use a %s canvas with aspect ratio %s; do not swap width and height.", orientation, aspectRatio)
}

func geminiImageAspectRatioFromSize(size string) string {
	size = strings.TrimSpace(size)
	if size == "" {
		return ""
	}
	if strings.Contains(size, ":") {
		return closestGeminiImageAspectRatio(size)
	}

	normalized := strings.ToLower(size)
	switch normalized {
	case "256x256", "512x512", "1024x1024", "2048x2048", "4096x4096":
		return "1:1"
	case "1536x1024", "3072x2048":
		return "3:2"
	case "1024x1536", "2048x3072":
		return "2:3"
	case "1024x1792", "2160x3840":
		return "9:16"
	case "1792x1024", "3840x2160":
		return "16:9"
	}

	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return ""
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return ""
	}
	return closestGeminiImageAspectRatio(fmt.Sprintf("%d:%d", width, height))
}

func closestGeminiImageAspectRatio(raw string) string {
	width, height, ok := parseAspectRatio(raw)
	if !ok {
		return ""
	}
	target := float64(width) / float64(height)
	supported := []struct {
		label string
		ratio float64
	}{
		{"1:1", 1.0},
		{"3:4", 3.0 / 4.0},
		{"4:3", 4.0 / 3.0},
		{"3:2", 3.0 / 2.0},
		{"2:3", 2.0 / 3.0},
		{"9:16", 9.0 / 16.0},
		{"16:9", 16.0 / 9.0},
	}

	best := supported[0]
	bestDistance := absFloat(target - best.ratio)
	for _, candidate := range supported[1:] {
		distance := absFloat(target - candidate.ratio)
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	return best.label
}

func parseAspectRatio(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func geminiImageSizeFromRequest(request dto.ImageRequest, info *relaycommon.RelayInfo) string {
	model := ""
	if info != nil {
		model = strings.ToLower(info.UpstreamModelName)
	}
	if strings.Contains(model, "4k") {
		return "4K"
	}

	switch strings.ToLower(strings.TrimSpace(request.Size)) {
	case "4k", "4096x4096", "2160x3840", "3840x2160":
		return "4K"
	case "2k", "2048x2048":
		return "2K"
	case "1k", "1024x1024", "1024x1792", "1792x1024", "1024x1536", "1536x1024":
		return "1K"
	}

	quality := strings.ToLower(strings.TrimSpace(request.Quality))
	switch quality {
	case "4k", "hd", "high":
		return "4K"
	case "2k":
		return "2K"
	case "1k", "standard", "medium", "low", "auto":
		return "1K"
	}
	return ""
}

func geminiImageEditMediaContents(c *gin.Context) ([]dto.MediaContent, error) {
	mf := c.Request.MultipartForm
	if mf == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil, errors.New("failed to parse multipart form")
		}
		mf = c.Request.MultipartForm
	}
	if mf == nil || mf.File == nil {
		return nil, errors.New("no multipart form data found")
	}

	imageFiles := collectImageFileHeaders(mf)
	if len(imageFiles) == 0 {
		return nil, errors.New("image is required")
	}

	mediaContents := make([]dto.MediaContent, 0, len(imageFiles))
	for i, fileHeader := range imageFiles {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open image file %d: %w", i, err)
		}

		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read image file %d: %w", i, err)
		}

		mimeType := detectImageMimeType(fileHeader.Filename)
		mediaContents = append(mediaContents, dto.MediaContent{
			Type: dto.ContentTypeImageURL,
			ImageUrl: &dto.MessageImageUrl{
				Url:      fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)),
				MimeType: mimeType,
			},
		})
	}
	return mediaContents, nil
}

func collectImageFileHeaders(mf *multipart.Form) []*multipart.FileHeader {
	var imageFiles []*multipart.FileHeader
	if files := mf.File["image"]; len(files) > 0 {
		imageFiles = append(imageFiles, files...)
	}
	if files := mf.File["image[]"]; len(files) > 0 {
		imageFiles = append(imageFiles, files...)
	}
	for fieldName, files := range mf.File {
		if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
			imageFiles = append(imageFiles, files...)
		}
	}
	return imageFiles
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if shouldUseGeminiImageChatCompatibility(info) {
		return convertImageRequestToGeminiImageChat(c, request, info)
	}
	request.Prompt = imagePromptWithRequestConstraints(request, info)
	switch info.RelayMode {
	case relayconstant.RelayModeImagesEdits:
		if isJSONRequest(c) {
			return request, nil
		}

		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)

		writer.WriteField("model", request.Model)
		// 使用已解析的 multipart 表单，避免重复解析
		mf := c.Request.MultipartForm
		if mf == nil {
			if _, err := c.MultipartForm(); err != nil {
				return nil, errors.New("failed to parse multipart form")
			}
			mf = c.Request.MultipartForm
		}

		// 写入所有非文件字段
		if mf != nil {
			for key, values := range mf.Value {
				if key == "model" {
					continue
				}
				for _, value := range values {
					if key == "prompt" {
						value = request.Prompt
					}
					writer.WriteField(key, value)
				}
			}
		}

		if mf != nil && mf.File != nil {
			// Check if "image" field exists in any form, including array notation
			var imageFiles []*multipart.FileHeader
			var exists bool

			// First check for standard "image" field
			if imageFiles, exists = mf.File["image"]; !exists || len(imageFiles) == 0 {
				// If not found, check for "image[]" field
				if imageFiles, exists = mf.File["image[]"]; !exists || len(imageFiles) == 0 {
					// If still not found, iterate through all fields to find any that start with "image["
					foundArrayImages := false
					for fieldName, files := range mf.File {
						if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
							foundArrayImages = true
							imageFiles = append(imageFiles, files...)
						}
					}

					// If no image fields found at all
					if !foundArrayImages && (len(imageFiles) == 0) {
						return nil, errors.New("image is required")
					}
				}
			}

			// Process all image files
			for i, fileHeader := range imageFiles {
				file, err := fileHeader.Open()
				if err != nil {
					return nil, fmt.Errorf("failed to open image file %d: %w", i, err)
				}

				// If multiple images, use image[] as the field name
				fieldName := "image"
				if len(imageFiles) > 1 {
					fieldName = "image[]"
				}

				// Determine MIME type based on file extension
				mimeType := detectImageMimeType(fileHeader.Filename)

				// Create a form file with the appropriate content type
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileHeader.Filename))
				h.Set("Content-Type", mimeType)

				part, err := writer.CreatePart(h)
				if err != nil {
					return nil, fmt.Errorf("create form part failed for image %d: %w", i, err)
				}

				if _, err := io.Copy(part, file); err != nil {
					return nil, fmt.Errorf("copy file failed for image %d: %w", i, err)
				}

				// 复制完立即关闭，避免在循环内使用 defer 占用资源
				_ = file.Close()
			}

			// Handle mask file if present
			if maskFiles, exists := mf.File["mask"]; exists && len(maskFiles) > 0 {
				maskFile, err := maskFiles[0].Open()
				if err != nil {
					return nil, errors.New("failed to open mask file")
				}
				// 复制完立即关闭，避免在循环内使用 defer 占用资源

				// Determine MIME type for mask file
				mimeType := detectImageMimeType(maskFiles[0].Filename)

				// Create a form file with the appropriate content type
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="mask"; filename="%s"`, maskFiles[0].Filename))
				h.Set("Content-Type", mimeType)

				maskPart, err := writer.CreatePart(h)
				if err != nil {
					return nil, errors.New("create form file failed for mask")
				}

				if _, err := io.Copy(maskPart, maskFile); err != nil {
					return nil, errors.New("copy mask file failed")
				}
				_ = maskFile.Close()
			}
		} else {
			return nil, errors.New("no multipart form data found")
		}

		// 关闭 multipart 编写器以设置分界线
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &requestBody, nil

	default:
		return request, nil
	}
}

func isJSONRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json")
}

// detectImageMimeType determines the MIME type based on the file extension
func detectImageMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		// Try to detect from extension if possible
		if strings.HasPrefix(ext, ".jp") {
			return "image/jpeg"
		}
		// Default to png as a fallback
		return "image/png"
	}
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	//  转换模型推理力度后缀
	effort, originModel := reasoning.ParseOpenAIReasoningEffortFromModelSuffix(request.Model)
	if effort != "" {
		if request.Reasoning == nil {
			request.Reasoning = &dto.Reasoning{
				Effort: effort,
			}
		} else {
			request.Reasoning.Effort = effort
		}
		request.Model = originModel
	}
	if info != nil && request.Reasoning != nil && request.Reasoning.Effort != "" {
		info.ReasoningEffort = request.Reasoning.Effort
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == relayconstant.RelayModeAudioTranscription ||
		info.RelayMode == relayconstant.RelayModeAudioTranslation ||
		(info.RelayMode == relayconstant.RelayModeImagesEdits && !isJSONRequest(c)) {
		return channel.DoFormRequest(a, c, info, requestBody)
	} else if info.RelayMode == relayconstant.RelayModeRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	} else {
		return channel.DoApiRequest(a, c, info, requestBody)
	}
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeRealtime:
		err, usage = OpenaiRealtimeHandler(c, info)
	case relayconstant.RelayModeAudioSpeech:
		usage = OpenaiTTSHandler(c, resp, info)
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err, usage = OpenaiSTTHandler(c, resp, info, a.ResponseFormat)
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		if shouldUseGeminiImageChatCompatibility(info) {
			usage, err = GeminiImageChatCompatibilityHandler(c, info, resp)
		} else if info.IsStream {
			usage, err = OpenaiImageStreamHandler(c, info, resp)
		} else {
			usage, err = OpenaiImageHandler(c, info, resp)
		}
	case relayconstant.RelayModeRerank:
		usage, err = common_handler.RerankHandler(c, info, resp)
	case relayconstant.RelayModeResponses:
		if info.IsStream {
			usage, err = OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = OaiResponsesHandler(c, info, resp)
		}
	case relayconstant.RelayModeResponsesCompact:
		usage, err = OaiResponsesCompactionHandler(c, resp)
	default:
		if info.IsStream {
			usage, err = OaiStreamHandler(c, info, resp)
		} else {
			usage, err = OpenaiHandler(c, info, resp)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	switch a.ChannelType {
	case constant.ChannelType360:
		return ai360.ModelList
	case constant.ChannelTypeLingYiWanWu:
		return lingyiwanwu.ModelList
	//case constant.ChannelTypeMiniMax:
	//	return minimax.ModelList
	case constant.ChannelTypeXinference:
		return xinference.ModelList
	case constant.ChannelTypeOpenRouter:
		return openrouter.ModelList
	default:
		return ModelList
	}
}

func (a *Adaptor) GetChannelName() string {
	switch a.ChannelType {
	case constant.ChannelType360:
		return ai360.ChannelName
	case constant.ChannelTypeLingYiWanWu:
		return lingyiwanwu.ChannelName
	//case constant.ChannelTypeMiniMax:
	//	return minimax.ChannelName
	case constant.ChannelTypeXinference:
		return xinference.ChannelName
	case constant.ChannelTypeOpenRouter:
		return openrouter.ChannelName
	default:
		return ChannelName
	}
}
