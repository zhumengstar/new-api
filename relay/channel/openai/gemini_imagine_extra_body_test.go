package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestBuildGeminiImagineExtraBodyMapsSizeToImageSize(t *testing.T) {
	tests := []struct {
		name        string
		size        string
		quality     string
		wantAspect  string
		wantImgSize string
	}{
		{
			name:        "explicit 4k",
			size:        "4k",
			wantAspect:  "1:1",
			wantImgSize: "4K",
		},
		{
			name:        "landscape 4k dimensions",
			size:        "3840x2160",
			wantAspect:  "16:9",
			wantImgSize: "4K",
		},
		{
			name:        "square 2k dimensions",
			size:        "2048x2048",
			wantAspect:  "1:1",
			wantImgSize: "2K",
		},
		{
			name:        "quality fallback",
			size:        "1024x1024",
			quality:     "hd",
			wantAspect:  "1:1",
			wantImgSize: "1K",
		},
		{
			name:        "wxhb high 9:16 dimensions",
			size:        "1728x3072",
			quality:     "high",
			wantAspect:  "9:16",
			wantImgSize: "4K",
		},
		{
			name:        "wxhb high 16:9 dimensions",
			size:        "3072x1728",
			quality:     "high",
			wantAspect:  "16:9",
			wantImgSize: "4K",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGeminiImagineExtraBody(dto.ImageRequest{Size: tt.size, Quality: tt.quality}, "")
			config, ok := got["image_config"].(map[string]any)
			if !ok {
				t.Fatalf("image_config missing or wrong type: %#v", got)
			}
			if config["aspect_ratio"] != tt.wantAspect {
				t.Fatalf("aspect_ratio = %v, want %s", config["aspect_ratio"], tt.wantAspect)
			}
			if config["image_size"] != tt.wantImgSize {
				t.Fatalf("image_size = %v, want %s", config["image_size"], tt.wantImgSize)
			}
		})
	}
}

func TestBuildGeminiImagineExtraBodyUses4KModelName(t *testing.T) {
	got := buildGeminiImagineExtraBody(dto.ImageRequest{}, "gemini-3.1-flash-image-4k")
	config, ok := got["image_config"].(map[string]any)
	if !ok {
		t.Fatalf("image_config missing or wrong type: %#v", got)
	}
	if config["image_size"] != "4K" {
		t.Fatalf("image_size = %v, want 4K", config["image_size"])
	}
}
