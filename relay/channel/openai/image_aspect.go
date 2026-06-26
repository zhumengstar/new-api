package openai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"golang.org/x/image/webp"
)

func enforceImageResponseAspectRatio(info *relaycommon.RelayInfo, imageResponse *dto.ImageResponse) {
	if imageResponse == nil || len(imageResponse.Data) == 0 {
		return
	}
	ratio, ok := requestedStrictAspectRatio(info)
	if !ok || ratio <= 0 {
		return
	}
	for i := range imageResponse.Data {
		cropped, ok := cropBase64ImageToAspectRatio(imageResponse.Data[i].B64Json, ratio)
		if ok {
			imageResponse.Data[i].B64Json = cropped
			imageResponse.Data[i].Url = ""
		}
	}
}

func requestedStrictAspectRatio(info *relaycommon.RelayInfo) (float64, bool) {
	if info == nil {
		return 0, false
	}
	if req, ok := info.Request.(*dto.ImageRequest); ok && req != nil {
		if req.StrictAspectRatio != nil && !*req.StrictAspectRatio {
			return 0, false
		}
		return aspectRatioFromImageRequest(*req)
	}
	if req, ok := info.Request.(*dto.GeneralOpenAIRequest); ok && req != nil {
		ratio, ok := aspectRatioFromGeneralOpenAIRequest(*req)
		return ratio, ok
	}
	return 0, false
}

func aspectRatioFromImageRequest(req dto.ImageRequest) (float64, bool) {
	if ratio, ok := aspectRatioFromString(req.Size); ok {
		return ratio, true
	}
	for _, key := range []string{"aspect_ratio", "aspectRatio"} {
		if raw, ok := req.Extra[key]; ok {
			var value string
			if json.Unmarshal(raw, &value) == nil {
				return aspectRatioFromString(value)
			}
		}
	}
	return 0, false
}

func aspectRatioFromGeneralOpenAIRequest(req dto.GeneralOpenAIRequest) (float64, bool) {
	if len(req.ExtraBody) == 0 {
		return 0, false
	}
	var extra map[string]any
	if json.Unmarshal(req.ExtraBody, &extra) != nil {
		return 0, false
	}
	if !boolFromNested(extra, "strict_aspect_ratio") {
		return 0, false
	}
	if ratio, ok := ratioFromNested(extra, "aspect_ratio"); ok {
		return ratio, true
	}
	if google, ok := extra["google"].(map[string]any); ok {
		if imageConfig, ok := google["image_config"].(map[string]any); ok {
			if ratio, ok := ratioFromNested(imageConfig, "aspect_ratio"); ok {
				return ratio, true
			}
		}
	}
	return 0, false
}

func boolFromNested(m map[string]any, key string) bool {
	value, ok := m[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func ratioFromNested(m map[string]any, key string) (float64, bool) {
	value, ok := m[key]
	if !ok {
		return 0, false
	}
	if text, ok := value.(string); ok {
		return aspectRatioFromString(text)
	}
	return 0, false
}

func aspectRatioFromString(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "1k" || value == "2k" || value == "4k" {
		return 1, value != ""
	}
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		w, wErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		h, hErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if wErr == nil && hErr == nil && w > 0 && h > 0 {
			return w / h, true
		}
		return 0, false
	}
	if strings.Contains(value, "x") {
		parts := strings.SplitN(value, "x", 2)
		w, wErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		h, hErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if wErr == nil && hErr == nil && w > 0 && h > 0 {
			return w / h, true
		}
	}
	return 0, false
}

func cropBase64ImageToAspectRatio(value string, targetRatio float64) (string, bool) {
	if targetRatio <= 0 {
		return "", false
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		if webpImg, webpErr := webp.Decode(bytes.NewReader(data)); webpErr == nil {
			img = webpImg
		} else {
			return "", false
		}
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return "", false
	}
	currentRatio := float64(width) / float64(height)
	if math.Abs(currentRatio-targetRatio) < 0.001 {
		return value, true
	}

	cropWidth := width
	cropHeight := height
	if currentRatio > targetRatio {
		cropWidth = int(math.Round(float64(height) * targetRatio))
	} else {
		cropHeight = int(math.Round(float64(width) / targetRatio))
	}
	if cropWidth <= 0 || cropHeight <= 0 || cropWidth > width || cropHeight > height {
		return "", false
	}

	x0 := bounds.Min.X + (width-cropWidth)/2
	y0 := bounds.Min.Y + (height-cropHeight)/2
	dst := image.NewRGBA(image.Rect(0, 0, cropWidth, cropHeight))
	draw.Draw(dst, dst.Bounds(), img, image.Point{X: x0, Y: y0}, draw.Src)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 95}); err != nil {
		return "", false
	}
	return base64.StdEncoding.EncodeToString(out.Bytes()), true
}
