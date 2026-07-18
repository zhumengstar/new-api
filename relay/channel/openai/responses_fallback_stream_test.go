package openai

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesFallbackStreamEmitsResponsesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	stream := true
	request := &dto.OpenAIResponsesRequest{
		Model:  "gemini-test",
		Input:  json.RawMessage(`"hello"`),
		Stream: &stream,
	}
	InitResponsesFallbackStream(context, request)
	info := &relaycommon.RelayInfo{}

	chunks := []string{
		`{"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"gemini-test","choices":[{"index":0,"delta":{"role":"assistant","content":"hel"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"gemini-test","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"gemini-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	}
	for _, chunk := range chunks {
		require.NoError(t, handleResponsesFallbackStream(context, info, chunk))
	}
	usage := &dto.Usage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3}
	finalizeResponsesFallbackStream(context, info, "chatcmpl-test", 123, "gemini-test", usage)

	body := recorder.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, "event: response.output_item.added")
	require.Contains(t, body, "event: response.content_part.added")
	require.Contains(t, body, `"type":"response.output_text.delta"`)
	require.Contains(t, body, `"delta":"hel"`)
	require.Contains(t, body, `"delta":"lo"`)
	require.Contains(t, body, `"text":"hello"`)
	require.Contains(t, body, "event: response.completed")
	require.Contains(t, body, `"input_tokens":2`)
	require.Contains(t, body, `"output_tokens":1`)
	require.NotContains(t, body, "data: [DONE]")
}

func TestResponsesFallbackStreamPreservesIncompleteStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	InitResponsesFallbackStream(context, &dto.OpenAIResponsesRequest{Model: "test"})

	require.NoError(t, handleResponsesFallbackStream(context, &relaycommon.RelayInfo{},
		`{"id":"chatcmpl-test","model":"test","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":"length"}]}`))
	finalizeResponsesFallbackStream(context, &relaycommon.RelayInfo{}, "chatcmpl-test", 123, "test", &dto.Usage{})

	require.Contains(t, recorder.Body.String(), `"status":"incomplete"`)
}

func TestResponsesFallbackStreamEmitsFunctionCallEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	InitResponsesFallbackStream(context, &dto.OpenAIResponsesRequest{
		Model: "test", Tools: json.RawMessage(`[{"type":"function","name":"lookup"}]`),
	})

	chunks := []string{
		`{"id":"chatcmpl-tools","model":"test","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]},"finish_reason":null}]}`,
		`{"id":"chatcmpl-tools","model":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]},"finish_reason":"tool_calls"}]}`,
	}
	for _, chunk := range chunks {
		require.NoError(t, handleResponsesFallbackStream(context, &relaycommon.RelayInfo{}, chunk))
	}
	finalizeResponsesFallbackStream(context, &relaycommon.RelayInfo{}, "chatcmpl-tools", 123, "test", &dto.Usage{})

	body := recorder.Body.String()
	require.Contains(t, body, "event: response.function_call_arguments.delta")
	require.Contains(t, body, "event: response.function_call_arguments.done")
	require.Contains(t, body, `"type":"function_call"`)
	require.Contains(t, body, `"call_id":"call_1"`)
	require.Contains(t, body, `"name":"lookup"`)
	require.Contains(t, body, `"arguments":"{\"q\":\"x\"}"`)
	require.Contains(t, body, "event: response.completed")
}
