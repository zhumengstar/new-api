package gemini

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if len(request.Contents) > 0 {
		for i, content := range request.Contents {
			if i == 0 {
				if request.Contents[0].Role == "" {
					request.Contents[0].Role = "user"
				}
			}
			for _, part := range content.Parts {
				if part.FileData != nil {
					if part.FileData.MimeType == "" && strings.Contains(part.FileData.FileUri, "www.youtube.com") {
						part.FileData.MimeType = "video/webm"
					}
				}
			}
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := openai.Adaptor{}
	oaiReq, err := adaptor.ConvertClaudeRequest(c, info, req)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, oaiReq.(*dto.GeneralOpenAIRequest))
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode == constant.RelayModeImagesGenerations && model_setting.IsGeminiModelSupportImagine(info.UpstreamModelName) {
		return a.convertImageGenerationRequest(c, info, request)
	}
	if info.RelayMode == constant.RelayModeImagesEdits && model_setting.IsGeminiModelSupportImagine(info.UpstreamModelName) {
		return a.convertImageEditRequest(c, info, request)
	}

	if !strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return nil, errors.New("not supported model for image generation, only imagen models are supported")
	}

	// convert size to aspect ratio but allow user to specify aspect ratio
	aspectRatio := "1:1" // default aspect ratio
	size := strings.TrimSpace(request.Size)
	if size != "" {
		if strings.Contains(size, ":") {
			aspectRatio = size
		} else {
			switch size {
			case "256x256", "512x512", "1024x1024":
				aspectRatio = "1:1"
			case "1536x1024":
				aspectRatio = "3:2"
			case "1024x1536":
				aspectRatio = "2:3"
			case "1024x1792":
				aspectRatio = "9:16"
			case "1792x1024":
				aspectRatio = "16:9"
			}
		}
	}

	// build gemini imagen request
	geminiRequest := dto.GeminiImageRequest{
		Instances: []dto.GeminiImageInstance{
			{
				Prompt: request.Prompt,
			},
		},
		Parameters: dto.GeminiImageParameters{
			SampleCount:      int(lo.FromPtrOr(request.N, uint(1))),
			AspectRatio:      aspectRatio,
			PersonGeneration: "allow_adult", // default allow adult
		},
	}

	if imageSize := geminiImageOutputSize(request, info.UpstreamModelName); imageSize != "" {
		geminiRequest.Parameters.ImageSize = imageSize
	}

	return geminiRequest, nil
}

func (a *Adaptor) convertImageGenerationRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	openAIRequest := dto.GeneralOpenAIRequest{
		Model: info.OriginModelName,
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: request.Prompt,
			},
		},
	}

	geminiRequest, err := CovertOpenAI2Gemini(c, openAIRequest, info)
	if err != nil {
		return nil, err
	}

	if imageConfig := buildGeminiEditImageConfig(request); len(imageConfig) > 0 {
		imageConfigBytes, err := common.Marshal(imageConfig)
		if err != nil {
			return nil, err
		}
		geminiRequest.GenerationConfig.ImageConfig = imageConfigBytes
	}
	if request.N != nil && *request.N > 0 {
		candidateCount := int(*request.N)
		geminiRequest.GenerationConfig.CandidateCount = &candidateCount
	}
	return geminiRequest, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeGemini {
		if strings.Contains(info.RequestURLPath, ":embedContent") ||
			strings.Contains(info.RequestURLPath, ":batchEmbedContents") {
			return NativeGeminiEmbeddingHandler(c, resp, info)
		}
		if info.IsStream {
			return GeminiTextGenerationStreamHandler(c, info, resp)
		}
		return GeminiTextGenerationHandler(c, info, resp)
	}

	if info.RelayMode == constant.RelayModeImagesEdits && model_setting.IsGeminiModelSupportImagine(info.UpstreamModelName) {
		return GeminiImageEditHandler(c, info, resp)
	}
	if info.RelayMode == constant.RelayModeImagesGenerations && model_setting.IsGeminiModelSupportImagine(info.UpstreamModelName) {
		return GeminiImageEditHandler(c, info, resp)
	}

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return GeminiImageHandler(c, info, resp)
	}

	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		return GeminiEmbeddingHandler(c, info, resp)
	}

	if info.IsStream {
		return GeminiChatStreamHandler(c, info, resp)
	}
	return GeminiChatHandler(c, info, resp)
}

func (a *Adaptor) convertImageEditRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if !strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		return nil, errors.New("Gemini image edit requires multipart/form-data image input")
	}

	imageFiles, err := getImageEditFiles(c)
	if err != nil {
		return nil, err
	}

	content := []any{
		map[string]any{
			"type": "text",
			"text": request.Prompt,
		},
	}
	for _, fileHeader := range imageFiles {
		dataURL, err := multipartImageToDataURL(fileHeader)
		if err != nil {
			return nil, err
		}
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": dataURL,
			},
		})
	}

	if maskFiles := c.Request.MultipartForm.File["mask"]; len(maskFiles) > 0 {
		dataURL, err := multipartImageToDataURL(maskFiles[0])
		if err != nil {
			return nil, err
		}
		content = append(content,
			map[string]any{
				"type": "text",
				"text": "Use the following mask image as the edit mask.",
			},
			map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": dataURL,
				},
			},
		)
	}

	openAIRequest := dto.GeneralOpenAIRequest{
		Model: info.OriginModelName,
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: content,
			},
		},
	}

	if imageConfig := buildGeminiEditImageConfig(request); len(imageConfig) > 0 {
		imageConfigBytes, err := common.Marshal(imageConfig)
		if err != nil {
			return nil, err
		}
		geminiRequest, err := CovertOpenAI2Gemini(c, openAIRequest, info)
		if err != nil {
			return nil, err
		}
		geminiRequest.GenerationConfig.ImageConfig = imageConfigBytes
		if request.N != nil && *request.N > 0 {
			candidateCount := int(*request.N)
			geminiRequest.GenerationConfig.CandidateCount = &candidateCount
		}
		return geminiRequest, nil
	}

	geminiRequest, err := CovertOpenAI2Gemini(c, openAIRequest, info)
	if err != nil {
		return nil, err
	}
	if request.N != nil && *request.N > 0 {
		candidateCount := int(*request.N)
		geminiRequest.GenerationConfig.CandidateCount = &candidateCount
	}
	return geminiRequest, nil
}

func getImageEditFiles(c *gin.Context) ([]*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
		}
	}
	mf := c.Request.MultipartForm
	if mf == nil || mf.File == nil {
		return nil, errors.New("image is required")
	}

	var imageFiles []*multipart.FileHeader
	for _, fieldName := range []string{"image", "image[]"} {
		if files := mf.File[fieldName]; len(files) > 0 {
			imageFiles = append(imageFiles, files...)
		}
	}
	for fieldName, files := range mf.File {
		if fieldName == "image" || fieldName == "image[]" {
			continue
		}
		if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
			imageFiles = append(imageFiles, files...)
		}
	}
	if len(imageFiles) == 0 {
		return nil, errors.New("image is required")
	}
	return imageFiles, nil
}

func multipartImageToDataURL(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open image file %s: %w", fileHeader.Filename, err)
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read image file %s: %w", fileHeader.Filename, err)
	}
	mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectGeminiImageMimeType(fileHeader.Filename, fileBytes)
	}
	if _, ok := geminiSupportedMimeTypes[strings.ToLower(mimeType)]; !ok {
		return "", fmt.Errorf("mime type is not supported by Gemini: '%s', file: '%s'", mimeType, fileHeader.Filename)
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fileBytes)), nil
}

func detectGeminiImageMimeType(filename string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	}
	mimeType := http.DetectContentType(data)
	if strings.HasPrefix(mimeType, "image/") {
		return mimeType
	}
	return "image/png"
}

func buildGeminiEditImageConfig(request dto.ImageRequest) map[string]any {
	imageConfig := make(map[string]any)
	if aspectRatio := geminiImageAspectRatio(request.Size); aspectRatio != "" {
		imageConfig["aspectRatio"] = aspectRatio
	}

	if imageSize := geminiImageOutputSize(request, ""); imageSize != "" {
		imageConfig["imageSize"] = imageSize
	}
	return imageConfig
}

func geminiImageOutputSize(request dto.ImageRequest, modelName string) string {
	switch normalizedGeminiImageSizeValue(request.Size) {
	case "4k", "4096x4096", "2160x3840", "3840x2160":
		return "4K"
	case "2k", "2048x2048":
		return "2K"
	case "1k", "1024x1024", "1024x1792", "1792x1024", "1024x1536", "1536x1024":
		return "1K"
	}

	switch normalizedGeminiImageSizeValue(request.Quality) {
	case "high", "hd", "4k":
		return "4K"
	case "2k":
		return "2K"
	case "low", "medium", "standard", "auto", "1k":
		return "1K"
	}
	if strings.Contains(normalizedGeminiImageSizeValue(modelName), "4k") {
		return "4K"
	}
	return ""
}

func geminiImageAspectRatio(size string) string {
	normalizedSize := normalizedGeminiImageSizeValue(size)
	switch normalizedSize {
	case "1024x1792":
		return "9:16"
	case "1792x1024":
		return "16:9"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	case "256x256", "512x512", "1024x1024", "2048x2048", "4096x4096", "1k", "2k", "4k":
		return "1:1"
	case "2160x3840":
		return "9:16"
	case "3840x2160":
		return "16:9"
	default:
		if strings.Contains(normalizedSize, ":") {
			return normalizedSize
		}
		if aspectRatio, ok := reduceGeminiPixelSizeToAspectRatio(normalizedSize); ok {
			return aspectRatio
		}
	}
	return ""
}

func reduceGeminiPixelSizeToAspectRatio(size string) (string, bool) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return "", false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return "", false
	}
	divisor := gcdGeminiDimensions(width, height)
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor), true
}

func gcdGeminiDimensions(a int, b int) int {
	a = int(math.Abs(float64(a)))
	b = int(math.Abs(float64(b)))
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

func normalizedGeminiImageSizeValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {

}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {

	if model_setting.GetGeminiSettings().ThinkingAdapterEnabled &&
		!model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) {
		// 新增逻辑：处理 -thinking-<budget> 格式
		if strings.Contains(info.UpstreamModelName, "-thinking-") {
			parts := strings.Split(info.UpstreamModelName, "-thinking-")
			info.UpstreamModelName = parts[0]
		} else if strings.HasSuffix(info.UpstreamModelName, "-thinking") { // 旧的适配
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
		} else if strings.HasSuffix(info.UpstreamModelName, "-nothinking") {
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-nothinking")
		} else if baseModel, level, ok := reasoning.TrimEffortSuffix(info.UpstreamModelName); ok && level != "" {
			info.UpstreamModelName = baseModel
		}
	}

	version := model_setting.GetGeminiVersionSetting(info.UpstreamModelName)

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return fmt.Sprintf("%s/%s/models/%s:predict", info.ChannelBaseUrl, version, info.UpstreamModelName), nil
	}

	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		action := "embedContent"
		if info.IsGeminiBatchEmbedding {
			action = "batchEmbedContents"
		}
		return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
	}

	action := "generateContent"
	if info.IsStream {
		action = "streamGenerateContent?alt=sse"
		if info.RelayMode == constant.RelayModeGemini {
			info.DisablePing = true
		}
	}
	return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("x-goog-api-key", info.ApiKey)
	if (info.RelayMode == constant.RelayModeImagesEdits || info.RelayMode == constant.RelayModeImagesGenerations) &&
		model_setting.IsGeminiModelSupportImagine(info.UpstreamModelName) {
		req.Set("Content-Type", "application/json")
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	geminiRequest, err := CovertOpenAI2Gemini(c, *request, info)
	if err != nil {
		return nil, err
	}

	return geminiRequest, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	if request.Input == nil {
		return nil, errors.New("input is required")
	}

	inputs := request.ParseInput()
	if len(inputs) == 0 {
		return nil, errors.New("input is empty")
	}
	// We always build a batch-style payload with `requests`, so ensure we call the
	// batch endpoint upstream to avoid payload/endpoint mismatches.
	info.IsGeminiBatchEmbedding = true
	// process all inputs
	geminiRequests := make([]map[string]interface{}, 0, len(inputs))
	for _, input := range inputs {
		geminiRequest := map[string]interface{}{
			"model": fmt.Sprintf("models/%s", info.UpstreamModelName),
			"content": dto.GeminiChatContent{
				Parts: []dto.GeminiPart{
					{
						Text: input,
					},
				},
			},
		}

		// set specific parameters for different models
		// https://ai.google.dev/api/embeddings?hl=zh-cn#method:-models.embedcontent
		switch info.UpstreamModelName {
		case "text-embedding-004", "gemini-embedding-exp-03-07", "gemini-embedding-001":
			// Only newer models introduced after 2024 support OutputDimensionality
			dimensions := lo.FromPtrOr(request.Dimensions, 0)
			if dimensions > 0 {
				geminiRequest["outputDimensionality"] = dimensions
			}
		}
		geminiRequests = append(geminiRequests, geminiRequest)
	}

	return map[string]interface{}{
		"requests": geminiRequests,
	}, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
