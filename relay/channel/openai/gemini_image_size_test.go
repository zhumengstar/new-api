package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestGeminiImageSizeFromRequestMaps4KDimensions(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gemini-3.1-flash-image-preview",
		},
	}

	tests := []struct {
		name string
		req  dto.ImageRequest
		want string
	}{
		{
			name: "portrait 4k dimensions",
			req:  dto.ImageRequest{Size: "2160x3840", Quality: "high"},
			want: "4K",
		},
		{
			name: "landscape 4k dimensions",
			req:  dto.ImageRequest{Size: "3840x2160"},
			want: "4K",
		},
		{
			name: "high quality fallback",
			req:  dto.ImageRequest{Quality: "high"},
			want: "4K",
		},
		{
			name: "2k dimensions",
			req:  dto.ImageRequest{Size: "2048x2048", Quality: "high"},
			want: "2K",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := geminiImageSizeFromRequest(tt.req, info)
			if got != tt.want {
				t.Fatalf("geminiImageSizeFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeminiImageSizeFromRequestUses4KModelName(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gemini-3.1-flash-image-4k",
		},
	}

	got := geminiImageSizeFromRequest(dto.ImageRequest{}, info)
	if got != "4K" {
		t.Fatalf("geminiImageSizeFromRequest() = %q, want 4K", got)
	}
}

func TestGeminiImageSizeFromRequestKeeps4KModelWhenClientSendsDefault1KSize(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gemini-3.1-flash-image-4k",
		},
	}

	got := geminiImageSizeFromRequest(dto.ImageRequest{Size: "1024x1024", Quality: "standard"}, info)
	if got != "4K" {
		t.Fatalf("geminiImageSizeFromRequest() = %q, want 4K", got)
	}
}
