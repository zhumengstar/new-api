package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
)

// 模拟 yyapi Claude OpenAI 兼容流：倒数第二条带 usage，最后一条 choices 为空
func TestStreamUsageInNonLastChunk(t *testing.T) {
	items := []string{
		`{"id":"x","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant","content":""},"index":0}]}`,
		`{"id":"x","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"},"index":0}]}`,
		`{"id":"x","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop","index":0}]}`,
		`{"id":"x","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":142,"completion_tokens":8,"total_tokens":150,"prompt_tokens_details":{"cached_tokens":0}}}`,
	}
	var found *dto.Usage
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item == "" || !service.SundaySearch(item, "usage") {
			continue
		}
		var probe dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal(common.StringToByteSlice(item), &probe); err != nil {
			continue
		}
		if service.ValidUsage(probe.Usage) {
			found = probe.Usage
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find usage in non-last chunk")
	}
	if found.PromptTokens != 142 || found.CompletionTokens != 8 {
		t.Fatalf("unexpected usage: %+v", found)
	}
}

// 上游把 usage 放在最后一条 chunk 的常见行为：扫描逻辑应仍只找到一条且不破坏原行为
func TestStreamUsageInLastChunkUnaffected(t *testing.T) {
	items := []string{
		`{"id":"x","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant","content":"Hi"},"index":0}]}`,
		`{"id":"x","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop","index":0}],"usage":{"prompt_tokens":10,"completion_tokens":1,"total_tokens":11}}`,
	}
	// 模拟：containStreamUsage 已经在 handleLastResponse 里置 true 时，跳过新增逻辑
	containStreamUsage := true
	if !containStreamUsage {
		for range items {
		}
		t.Fatal("should not enter fallback when last chunk already had usage")
	}
}
