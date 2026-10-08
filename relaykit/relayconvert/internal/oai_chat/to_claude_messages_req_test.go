package oaichat

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestOpenAIChatRequestToClaudeMessagesNormalizesToolInputSchema(t *testing.T) {
	tests := []struct {
		name       string
		parameters any
		wantSchema map[string]any
	}{
		{
			name:       "omitted parameters",
			parameters: nil,
			wantSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			name: "missing type and properties",
			parameters: map[string]any{
				"additionalProperties": false,
			},
			wantSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			name: "non-string type",
			parameters: map[string]any{
				"type":       123,
				"properties": map[string]any{},
			},
			wantSchema: map[string]any{
				"type":       123,
				"properties": map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxTokens := uint(1024)
			got, err := OpenAIChatRequestToClaudeMessages(context.Background(), nil, dto.GeneralOpenAIRequest{
				Model:     "claude-test",
				MaxTokens: &maxTokens,
				Messages: []dto.Message{
					{Role: "user", Content: "Call the tool."},
				},
				Tools: []dto.ToolCallRequest{
					{
						Type: "function",
						Function: dto.FunctionRequest{
							Name:        "get_current_time",
							Description: "Get the current time",
							Parameters:  tt.parameters,
						},
					},
				},
			})

			require.NoError(t, err)
			tools, ok := got.Tools.([]any)
			require.True(t, ok)
			require.Len(t, tools, 1)
			tool, ok := tools[0].(*dto.Tool)
			require.True(t, ok)
			assert.Equal(t, "get_current_time", tool.Name)
			assert.Equal(t, tt.wantSchema, tool.InputSchema)
		})
	}
}

func TestOpenAIChatRequestToClaudeMessagesAdaptiveAdjustments(t *testing.T) {
	t.Run("auto lifts max_tokens when budget_tokens >= max_tokens and clears sampling", func(t *testing.T) {
		maxTokens := uint(1000)
		budget := 2000
		temp := 0.7
		topP := 0.9
		topK := 40
		got, err := OpenAIChatRequestToClaudeMessages(context.Background(), nil, dto.GeneralOpenAIRequest{
			Model:       "claude-3-7-sonnet-20250219",
			MaxTokens:   &maxTokens,
			Temperature: &temp,
			TopP:        &topP,
			TopK:        &topK,
			THINKING:    []byte(fmt.Sprintf(`{"type":"enabled","budget_tokens":%d}`, budget)),
			Messages: []dto.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: ""}, // Trailing empty assistant should be pruned
			},
		})

		require.NoError(t, err)
		require.NotNil(t, got.MaxTokens)
		assert.Greater(t, *got.MaxTokens, uint(budget), "max_tokens should be strictly greater than budget_tokens")
		assert.Nil(t, got.Temperature, "temperature should be nil when thinking enabled")
		assert.Nil(t, got.TopP, "top_p should be nil when thinking enabled")
		assert.Nil(t, got.TopK, "top_k should be nil when thinking enabled")

		// Trailing empty assistant was pruned, so only user message remains
		require.Len(t, got.Messages, 1)
		assert.Equal(t, "user", got.Messages[0].Role)
	})

	t.Run("cleans up empty tools and orphaned tool_choice", func(t *testing.T) {
		maxTokens := uint(1000)
		got, err := OpenAIChatRequestToClaudeMessages(context.Background(), nil, dto.GeneralOpenAIRequest{
			Model:      "claude-3-5-sonnet-20241022",
			MaxTokens:  &maxTokens,
			Tools:      []dto.ToolCallRequest{}, // Empty tools
			ToolChoice: "auto",
			Messages: []dto.Message{
				{Role: "user", Content: "Hello"},
			},
		})

		require.NoError(t, err)
		assert.Nil(t, got.Tools)
		assert.Nil(t, got.ToolChoice)
	})
}

