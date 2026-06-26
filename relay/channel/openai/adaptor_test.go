package openai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestConvertOpenAIRequestSanitizesOpenAICompatibleGeminiExtras(t *testing.T) {
	stream := true
	parallelToolCalls := true
	logProbs := true
	topLogProbs := 2
	temperature := 0.7
	reasoning := json.RawMessage(`{"effort":"medium"}`)
	store := json.RawMessage(`true`)

	request := &dto.GeneralOpenAIRequest{
		Model: "gemini-3-flash",
		Messages: []dto.Message{{
			Role:             "user",
			Content:          []any{map[string]any{"type": "text", "text": "hello", "cache_control": map[string]any{"type": "ephemeral"}}},
			ReasoningContent: ptrOf("hidden reasoning"),
			Reasoning:        ptrOf("hidden reasoning"),
			Name:             ptrOf("tester"),
		}},
		Stream:           &stream,
		Temperature:      &temperature,
		ResponseFormat:   &dto.ResponseFormat{Type: "json_object"},
		ParallelTooCalls: &parallelToolCalls,
		ToolChoice:       "auto",
		LogProbs:         &logProbs,
		TopLogProbs:      &topLogProbs,
		Store:            store,
		Metadata:         json.RawMessage(`{"trace":"abc"}`),
		Reasoning:        reasoning,
		ReasoningEffort:  "medium",
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gemini-3-flash",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got := converted.(*dto.GeneralOpenAIRequest)

	if got.Stream == nil || *got.Stream != true {
		t.Fatalf("Stream was not preserved")
	}
	if got.Temperature == nil || *got.Temperature != temperature {
		t.Fatalf("Temperature was not preserved")
	}
	if got.ResponseFormat != nil {
		t.Fatalf("ResponseFormat was not cleared")
	}
	if got.ParallelTooCalls != nil {
		t.Fatalf("ParallelTooCalls was not cleared")
	}
	if got.ToolChoice != nil {
		t.Fatalf("ToolChoice was not cleared")
	}
	if got.LogProbs != nil || got.TopLogProbs != nil {
		t.Fatalf("logprobs fields were not cleared")
	}
	if got.Store != nil || got.Metadata != nil {
		t.Fatalf("OpenAI-only metadata fields were not cleared")
	}
	if got.Reasoning != nil || got.ReasoningEffort != "" {
		t.Fatalf("reasoning fields were not cleared")
	}
	if got.Messages[0].Name != nil || got.Messages[0].ReasoningContent != nil || got.Messages[0].Reasoning != nil {
		t.Fatalf("message extension fields were not cleared")
	}
	content := got.Messages[0].Content.([]any)
	block := content[0].(map[string]any)
	if _, ok := block["cache_control"]; ok {
		t.Fatalf("content cache_control was not cleared")
	}
}

func TestConvertOpenAIRequestDoesNotSanitizeNonGeminiCompatibleModel(t *testing.T) {
	parallelToolCalls := true
	request := &dto.GeneralOpenAIRequest{
		Model:            "gpt-4o-mini",
		ParallelTooCalls: &parallelToolCalls,
		ToolChoice:       "auto",
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-4o-mini",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got := converted.(*dto.GeneralOpenAIRequest)
	if got.ParallelTooCalls == nil || *got.ParallelTooCalls != true {
		t.Fatalf("ParallelTooCalls should be preserved for non-Gemini models")
	}
	if got.ToolChoice != "auto" {
		t.Fatalf("ToolChoice should be preserved for non-Gemini models")
	}
}

func ptrOf[T any](value T) *T {
	return &value
}
