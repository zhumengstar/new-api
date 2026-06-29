package openai

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

func buildGeminiImagineExtraBody(request dto.ImageRequest, modelName string) map[string]any {
	imageConfig := make(map[string]any)
	if aspectRatio := geminiImagineAspectRatio(request.Size); aspectRatio != "" {
		imageConfig["aspect_ratio"] = aspectRatio
	}
	if imageSize := geminiImagineOutputSize(request, modelName); imageSize != "" {
		imageConfig["image_size"] = imageSize
	}
	if len(imageConfig) == 0 {
		return nil
	}
	return map[string]any{"image_config": imageConfig}
}

func geminiImagineOutputSize(request dto.ImageRequest, modelName string) string {
	if strings.Contains(normalizedGeminiImagineSizeValue(modelName), "4k") {
		return "4K"
	}

	switch normalizedGeminiImagineSizeValue(request.Size) {
	case "4k", "4096x4096", "2160x3840", "3840x2160":
		return "4K"
	case "2k", "2048x2048":
		return "2K"
	case "1k", "1024x1024", "1024x1792", "1792x1024", "1024x1536", "1536x1024":
		return "1K"
	}

	switch normalizedGeminiImagineSizeValue(request.Quality) {
	case "hd", "high", "4k":
		return "4K"
	case "2k":
		return "2K"
	case "standard", "medium", "low", "auto", "1k":
		return "1K"
	}
	return ""
}

func geminiImagineAspectRatio(size string) string {
	normalizedSize := normalizedGeminiImagineSizeValue(size)
	switch normalizedSize {
	case "1024x1792", "2160x3840":
		return "9:16"
	case "1792x1024", "3840x2160":
		return "16:9"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	case "256x256", "512x512", "1024x1024", "2048x2048", "4096x4096", "1k", "2k", "4k":
		return "1:1"
	default:
		if strings.Contains(normalizedSize, ":") {
			return normalizedSize
		}
		if aspectRatio, ok := reduceGeminiImaginePixelSizeToAspectRatio(normalizedSize); ok {
			return aspectRatio
		}
	}
	return ""
}

func reduceGeminiImaginePixelSizeToAspectRatio(size string) (string, bool) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return "", false
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return "", false
	}
	divisor := gcdGeminiImagineDimensions(width, height)
	return fmt.Sprintf("%d:%d", width/divisor, height/divisor), true
}

func gcdGeminiImagineDimensions(a int, b int) int {
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

func normalizedGeminiImagineSizeValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
