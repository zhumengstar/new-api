package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesRequestToChatCompletionsRequestStringInput(t *testing.T) {
	input, err := common.Marshal("hello")
	require.NoError(t, err)

	req := &dto.OpenAIResponsesRequest{
		Model: "test-model",
		Input: input,
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 1)
	assert.Equal(t, "test-model", chatReq.Model)
	assert.Equal(t, "user", chatReq.Messages[0].Role)
	assert.Equal(t, "hello", chatReq.Messages[0].Content)
}

func TestResponsesRequestToChatCompletionsRequestInstructionsAndImage(t *testing.T) {
	input := json.RawMessage(`[
		{
			"role": "user",
			"content": [
				{"type": "input_text", "text": "describe"},
				{"type": "input_image", "image_url": "data:image/png;base64,abc", "detail": "low"}
			]
		}
	]`)
	instructions, err := common.Marshal("be concise")
	require.NoError(t, err)

	req := &dto.OpenAIResponsesRequest{
		Model:        "test-model",
		Input:        input,
		Instructions: instructions,
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)
	assert.Equal(t, "system", chatReq.Messages[0].Role)
	assert.Equal(t, "be concise", chatReq.Messages[0].Content)

	parts, ok := chatReq.Messages[1].Content.([]dto.MediaContent)
	require.True(t, ok)
	require.Len(t, parts, 2)
	assert.Equal(t, dto.ContentTypeText, parts[0].Type)
	assert.Equal(t, dto.ContentTypeImageURL, parts[1].Type)
	img, ok := parts[1].ImageUrl.(*dto.MessageImageUrl)
	require.True(t, ok)
	assert.Equal(t, "data:image/png;base64,abc", img.Url)
	assert.Equal(t, "low", img.Detail)
}

func TestResponsesRequestToChatCompletionsRequestSupportsStream(t *testing.T) {
	stream := true
	req := &dto.OpenAIResponsesRequest{
		Model:  "test-model",
		Input:  json.RawMessage(`"hello"`),
		Stream: &stream,
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.NotNil(t, chatReq.Stream)
	require.True(t, *chatReq.Stream)
}

func TestResponsesRequestToChatCompletionsRequestSupportsFunctionTools(t *testing.T) {
	parallel := json.RawMessage(`true`)
	req := &dto.OpenAIResponsesRequest{
		Model: "test-model",
		Input: json.RawMessage(`[
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"found"},
			{"role":"user","content":"continue"}
		]`),
		Tools:             json.RawMessage(`[{"type":"function","name":"lookup","description":"look up","parameters":{"type":"object"}}]`),
		ToolChoice:        json.RawMessage(`{"type":"function","name":"lookup"}`),
		ParallelToolCalls: parallel,
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)
	assert.Equal(t, "lookup", chatReq.Tools[0].Function.Name)
	require.NotNil(t, chatReq.ParallelTooCalls)
	assert.True(t, *chatReq.ParallelTooCalls)
	require.Len(t, chatReq.Messages, 3)
	assert.Equal(t, "assistant", chatReq.Messages[0].Role)
	assert.Equal(t, "lookup", chatReq.Messages[0].ParseToolCalls()[0].Function.Name)
	assert.Equal(t, "tool", chatReq.Messages[1].Role)
	assert.Equal(t, "call_1", chatReq.Messages[1].ToolCallId)
}

func TestResponsesRequestToChatCompletionsRequestRejectsBuiltInTools(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "test-model", Input: json.RawMessage(`"hello"`),
		Tools: json.RawMessage(`[{"type":"web_search_preview"}]`),
	}
	_, err := ResponsesRequestToChatCompletionsRequest(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be represented")
}

func TestChatCompletionsToolCallResponseToResponsesResponse(t *testing.T) {
	toolCalls, err := common.Marshal([]dto.ToolCallRequest{{
		ID: "call_1", Type: "function",
		Function: dto.FunctionRequest{Name: "lookup", Arguments: `{"q":"x"}`},
	}})
	require.NoError(t, err)
	resp := &dto.OpenAITextResponse{
		Id: "chatcmpl_tools", Model: "test-model",
		Choices: []dto.OpenAITextResponseChoice{{
			Message: dto.Message{Role: "assistant", ToolCalls: toolCalls}, FinishReason: "tool_calls",
		}},
	}
	req := &dto.OpenAIResponsesRequest{Model: "test-model", Tools: json.RawMessage(`[{"type":"function","name":"lookup"}]`)}
	out, _, err := ChatCompletionsResponseToResponsesResponse(resp, req)
	require.NoError(t, err)
	require.Len(t, out.Output, 1)
	assert.Equal(t, "function_call", out.Output[0].Type)
	assert.Equal(t, "call_1", out.Output[0].CallId)
	assert.Equal(t, "lookup", out.Output[0].Name)
	assert.Equal(t, `{"q":"x"}`, dto.ResponsesArgumentsString(out.Output[0].Arguments))
}

func TestChatCompletionsResponseToResponsesResponse(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{Model: "test-model"}
	resp := &dto.OpenAITextResponse{
		Id:     "chatcmpl_1",
		Model:  "test-model",
		Object: "chat.completion",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: "ok",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     3,
			CompletionTokens: 2,
			TotalTokens:      5,
		},
	}

	out, usage, err := ChatCompletionsResponseToResponsesResponse(resp, req)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 3, usage.InputTokens)
	assert.Equal(t, 2, usage.OutputTokens)
	assert.Equal(t, 5, usage.TotalTokens)
	assert.Equal(t, "response", out.Object)
	assert.Equal(t, "chatcmpl_1", out.ID)
	require.Len(t, out.Output, 1)
	require.Len(t, out.Output[0].Content, 1)
	assert.Equal(t, "ok", out.Output[0].Content[0].Text)
}
