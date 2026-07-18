package openai

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const responsesFallbackStreamStateKey = "responses_fallback_stream_state"

type responsesFallbackStreamState struct {
	Request      *dto.OpenAIResponsesRequest
	ResponseID   string
	ItemID       string
	Model        string
	Role         string
	Text         strings.Builder
	FinishReason string
	Started      bool
	Sequence     int
	ToolCalls    map[int]*responsesFallbackToolCall
}

type responsesFallbackToolCall struct {
	OutputIndex int
	ItemID      string
	CallID      string
	Name        string
	Arguments   strings.Builder
}

func InitResponsesFallbackStream(c *gin.Context, request *dto.OpenAIResponsesRequest) {
	c.Set(responsesFallbackStreamStateKey, &responsesFallbackStreamState{
		Request:   request,
		Role:      "assistant",
		ToolCalls: make(map[int]*responsesFallbackToolCall),
	})
}

func getResponsesFallbackStreamState(c *gin.Context) *responsesFallbackStreamState {
	if value, ok := c.Get(responsesFallbackStreamStateKey); ok {
		if state, ok := value.(*responsesFallbackStreamState); ok && state != nil {
			return state
		}
	}
	state := &responsesFallbackStreamState{Role: "assistant", ToolCalls: make(map[int]*responsesFallbackToolCall)}
	c.Set(responsesFallbackStreamStateKey, state)
	return state
}

func sendResponsesFallbackEvent(c *gin.Context, state *responsesFallbackStreamState, eventType string, fields map[string]any) error {
	state.Sequence++
	payload := map[string]any{
		"type":            eventType,
		"sequence_number": state.Sequence,
	}
	for key, value := range fields {
		payload[key] = value
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventType}, string(data))
	return nil
}

func startResponsesFallbackStream(c *gin.Context, state *responsesFallbackStreamState, chunk *dto.ChatCompletionsStreamResponse) error {
	if state.Started {
		return nil
	}
	state.Started = true
	state.ResponseID = strings.TrimSpace(chunk.Id)
	if state.ResponseID == "" {
		state.ResponseID = "resp_" + common.GetRandomString(24)
	}
	state.ItemID = "msg_" + common.GetRandomString(24)
	state.Model = chunk.Model
	if state.Model == "" && state.Request != nil {
		state.Model = state.Request.Model
	}

	createdResponse := buildResponsesFallbackResponse(state, nil, "in_progress")
	createdResponse.Output = []dto.ResponsesOutput{}
	if err := sendResponsesFallbackEvent(c, state, "response.created", map[string]any{"response": createdResponse}); err != nil {
		return err
	}
	item := responsesFallbackOutputItem(state, "in_progress", "")
	item.Content = []dto.ResponsesOutputContent{}
	if err := sendResponsesFallbackEvent(c, state, "response.output_item.added", map[string]any{
		"output_index": 0,
		"item":         item,
	}); err != nil {
		return err
	}
	return sendResponsesFallbackEvent(c, state, "response.content_part.added", map[string]any{
		"item_id":       state.ItemID,
		"output_index":  0,
		"content_index": 0,
		"part": map[string]any{
			"type":        "output_text",
			"text":        "",
			"annotations": []any{},
		},
	})
}

func handleResponsesFallbackStream(c *gin.Context, info *relaycommon.RelayInfo, data string) error {
	var chunk dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
		return err
	}
	state := getResponsesFallbackStreamState(c)
	if err := startResponsesFallbackStream(c, state, &chunk); err != nil {
		return err
	}
	for _, choice := range chunk.Choices {
		if role := strings.TrimSpace(choice.Delta.Role); role != "" {
			state.Role = role
		}
		text := choice.Delta.GetContentString()
		if reasoning := choice.Delta.GetReasoningContent(); reasoning != "" {
			text += reasoning
		}
		if text != "" {
			state.Text.WriteString(text)
			if err := sendResponsesFallbackEvent(c, state, "response.output_text.delta", map[string]any{
				"item_id":       state.ItemID,
				"output_index":  0,
				"content_index": 0,
				"delta":         text,
			}); err != nil {
				return err
			}
		}
		for position, toolDelta := range choice.Delta.ToolCalls {
			index := position
			if toolDelta.Index != nil {
				index = *toolDelta.Index
			}
			toolCall := state.ToolCalls[index]
			if toolCall == nil {
				callID := strings.TrimSpace(toolDelta.ID)
				if callID == "" {
					callID = "call_" + common.GetRandomString(24)
				}
				toolCall = &responsesFallbackToolCall{
					OutputIndex: index + 1,
					ItemID:      "fc_" + common.GetRandomString(24),
					CallID:      callID,
					Name:        "",
				}
				state.ToolCalls[index] = toolCall
				if err := sendResponsesFallbackEvent(c, state, "response.output_item.added", map[string]any{
					"output_index": toolCall.OutputIndex,
					"item":         responsesFallbackToolOutputItem(toolCall, "in_progress"),
				}); err != nil {
					return err
				}
			}
			if toolDelta.ID != "" {
				toolCall.CallID = toolDelta.ID
			}
			if toolDelta.Function.Name != "" {
				toolCall.Name += toolDelta.Function.Name
			}
			if toolDelta.Function.Arguments != "" {
				toolCall.Arguments.WriteString(toolDelta.Function.Arguments)
				if err := sendResponsesFallbackEvent(c, state, "response.function_call_arguments.delta", map[string]any{
					"item_id": toolCall.ItemID, "output_index": toolCall.OutputIndex,
					"delta": toolDelta.Function.Arguments,
				}); err != nil {
					return err
				}
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			state.FinishReason = *choice.FinishReason
		}
	}
	return nil
}

func finalizeResponsesFallbackStream(c *gin.Context, info *relaycommon.RelayInfo, responseID string, createdAt int64, model string, usage *dto.Usage) {
	state := getResponsesFallbackStreamState(c)
	if !state.Started {
		chunk := &dto.ChatCompletionsStreamResponse{Id: responseID, Created: createdAt, Model: model}
		if err := startResponsesFallbackStream(c, state, chunk); err != nil {
			return
		}
	}
	if state.ResponseID == "" {
		state.ResponseID = responseID
	}
	if state.Model == "" {
		state.Model = model
	}
	text := state.Text.String()
	_ = sendResponsesFallbackEvent(c, state, "response.output_text.done", map[string]any{
		"item_id":       state.ItemID,
		"output_index":  0,
		"content_index": 0,
		"text":          text,
	})
	part := map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
	_ = sendResponsesFallbackEvent(c, state, "response.content_part.done", map[string]any{
		"item_id":       state.ItemID,
		"output_index":  0,
		"content_index": 0,
		"part":          part,
	})
	itemStatus := "completed"
	if state.FinishReason == "length" {
		itemStatus = "incomplete"
	}
	_ = sendResponsesFallbackEvent(c, state, "response.output_item.done", map[string]any{
		"output_index": 0,
		"item":         responsesFallbackOutputItem(state, itemStatus, text),
	})
	for index := 0; index < len(state.ToolCalls); index++ {
		toolCall := state.ToolCalls[index]
		if toolCall == nil {
			continue
		}
		_ = sendResponsesFallbackEvent(c, state, "response.function_call_arguments.done", map[string]any{
			"item_id": toolCall.ItemID, "output_index": toolCall.OutputIndex,
			"arguments": toolCall.Arguments.String(),
		})
		_ = sendResponsesFallbackEvent(c, state, "response.output_item.done", map[string]any{
			"output_index": toolCall.OutputIndex,
			"item":         responsesFallbackToolOutputItem(toolCall, "completed"),
		})
	}
	_ = sendResponsesFallbackEvent(c, state, "response.completed", map[string]any{
		"response": buildResponsesFallbackResponse(state, usage, itemStatus),
	})
}

func responsesFallbackToolOutputItem(toolCall *responsesFallbackToolCall, status string) dto.ResponsesOutput {
	arguments, _ := common.Marshal(toolCall.Arguments.String())
	return dto.ResponsesOutput{
		Type: "function_call", ID: toolCall.ItemID, Status: status,
		CallId: toolCall.CallID, Name: toolCall.Name, Arguments: arguments,
	}
}

func responsesFallbackOutputItem(state *responsesFallbackStreamState, status string, text string) dto.ResponsesOutput {
	return dto.ResponsesOutput{
		Type:   "message",
		ID:     state.ItemID,
		Status: status,
		Role:   state.Role,
		Content: []dto.ResponsesOutputContent{{
			Type:        "output_text",
			Text:        text,
			Annotations: []interface{}{},
		}},
	}
}

func buildResponsesFallbackResponse(state *responsesFallbackStreamState, usage *dto.Usage, status string) *dto.OpenAIResponsesResponse {
	outputs := []dto.ResponsesOutput{responsesFallbackOutputItem(state, status, state.Text.String())}
	for index := 0; index < len(state.ToolCalls); index++ {
		if toolCall := state.ToolCalls[index]; toolCall != nil {
			outputs = append(outputs, responsesFallbackToolOutputItem(toolCall, "completed"))
		}
	}
	response := &dto.OpenAIResponsesResponse{
		ID:        state.ResponseID,
		Object:    "response",
		CreatedAt: int(common.GetTimestamp()),
		Status:    json.RawMessage(`"` + status + `"`),
		Model:     state.Model,
		Output:    outputs,
		Usage:     usage,
		Tools:     []map[string]any{},
	}
	if state.Request == nil {
		return response
	}
	seed := &dto.OpenAITextResponse{
		Id:    state.ResponseID,
		Model: state.Model,
		Choices: []dto.OpenAITextResponseChoice{{
			Message:      dto.Message{Role: state.Role, Content: state.Text.String()},
			FinishReason: state.FinishReason,
		}},
	}
	if usage != nil {
		seed.Usage = *usage
	}
	if converted, _, err := service.ChatCompletionsResponseToResponsesResponse(seed, state.Request); err == nil {
		converted.ID = state.ResponseID
		converted.CreatedAt = int(common.GetTimestamp())
		converted.Status = response.Status
		converted.Output = response.Output
		return converted
	}
	return response
}
