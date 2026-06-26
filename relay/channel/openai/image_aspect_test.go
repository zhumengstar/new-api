package openai

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestAspectRatioFromString(t *testing.T) {
	tests := []struct {
		value string
		want  float64
	}{
		{"1:1", 1},
		{"9:16", 0.5625},
		{"1024x1792", 1024.0 / 1792.0},
		{"4k", 1},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, ok := aspectRatioFromString(tt.value)
			if !ok {
				t.Fatalf("aspectRatioFromString(%q) returned !ok", tt.value)
			}
			if got != tt.want {
				t.Fatalf("aspectRatioFromString(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestCropBase64ImageToAspectRatio(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1408, 768))
	for y := 0; y < 768; y++ {
		for x := 0; x < 1408; x++ {
			src.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	cropped, ok := cropBase64ImageToAspectRatio(base64.StdEncoding.EncodeToString(buf.Bytes()), 1)
	if !ok {
		t.Fatal("cropBase64ImageToAspectRatio returned !ok")
	}
	data, err := base64.StdEncoding.DecodeString(cropped)
	if err != nil {
		t.Fatalf("decode cropped image: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode cropped config: %v", err)
	}
	if cfg.Width != cfg.Height {
		t.Fatalf("cropped dimensions = %dx%d, want square", cfg.Width, cfg.Height)
	}
}

func TestEnforceImageResponseAspectRatio(t *testing.T) {
	info := &relaycommon.RelayInfo{
		Request: &dto.ImageRequest{
			Size: "1:1",
		},
	}
	img := image.NewRGBA(image.Rect(0, 0, 1408, 768))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	resp := &dto.ImageResponse{
		Data: []dto.ImageData{{B64Json: base64.StdEncoding.EncodeToString(buf.Bytes())}},
	}
	enforceImageResponseAspectRatio(info, resp)
	data, err := base64.StdEncoding.DecodeString(resp.Data[0].B64Json)
	if err != nil {
		t.Fatalf("decode strict image: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode strict config: %v", err)
	}
	if cfg.Width != cfg.Height {
		t.Fatalf("strict dimensions = %dx%d, want square", cfg.Width, cfg.Height)
	}
}

func TestEnforceImageResponseAspectRatioCanBeDisabled(t *testing.T) {
	strict := false
	info := &relaycommon.RelayInfo{
		Request: &dto.ImageRequest{
			Size:              "1:1",
			StrictAspectRatio: &strict,
		},
	}
	img := image.NewRGBA(image.Rect(0, 0, 1408, 768))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	resp := &dto.ImageResponse{
		Data: []dto.ImageData{{B64Json: base64.StdEncoding.EncodeToString(buf.Bytes())}},
	}
	enforceImageResponseAspectRatio(info, resp)
	data, err := base64.StdEncoding.DecodeString(resp.Data[0].B64Json)
	if err != nil {
		t.Fatalf("decode image: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.Width != 1408 || cfg.Height != 768 {
		t.Fatalf("dimensions = %dx%d, want original 1408x768", cfg.Width, cfg.Height)
	}
}
