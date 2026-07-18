package relay

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
)

func TestResponsesFallbackDiagnosticContainsOnlyStructuralCounts(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"private text"}]},{"type":"function_call","name":"private_name","call_id":"private_id"}]`),
		Tools: json.RawMessage(`[{"type":"function","name":"private_tool"}]`),
	}
	converted := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{
		Role: "user", Parts: []dto.GeminiPart{{Text: "private text"}},
	}}}

	diagnostic := responsesFallbackDiagnostic(request, converted)
	assert.Contains(t, diagnostic, "input_items=2")
	assert.Contains(t, diagnostic, "function_calls=1")
	assert.Contains(t, diagnostic, "gemini_contents=1")
	assert.NotContains(t, diagnostic, "private")
}
