package gemini

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestBuildGeminiEditImageConfigMapsSizeToImageSize(t *testing.T) {
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
			name:        "portrait 4k dimensions",
			size:        "2160x3840",
			wantAspect:  "9:16",
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
			quality:     "high",
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
			got := buildGeminiEditImageConfig(dto.ImageRequest{Size: tt.size, Quality: tt.quality})
			if got["aspectRatio"] != tt.wantAspect {
				t.Fatalf("aspectRatio = %v, want %s", got["aspectRatio"], tt.wantAspect)
			}
			if got["imageSize"] != tt.wantImgSize {
				t.Fatalf("imageSize = %v, want %s", got["imageSize"], tt.wantImgSize)
			}
		})
	}
}

func TestGeminiImageOutputSizeUses4KModelName(t *testing.T) {
	got := geminiImageOutputSize(dto.ImageRequest{}, "gemini-3.1-flash-image-4k")
	if got != "4K" {
		t.Fatalf("imageSize = %v, want 4K", got)
	}
}
