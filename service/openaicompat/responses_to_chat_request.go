package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

type responsesInputItem struct {
	Type      string          `json:"type,omitempty"`
	Role      string          `json:"role,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	Text      string          `json:"text,omitempty"`
	ImageURL  any             `json:"image_url,omitempty"`
	Detail    string          `json:"detail,omitempty"`
	FileURL   any             `json:"file_url,omitempty"`
	File      any             `json:"file,omitempty"`
	InputFile any             `json:"input_file,omitempty"`
}

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}
	if lo.FromPtrOr(req.Stream, false) {
		return nil, errors.New("stream responses fallback is not supported")
	}
	if strings.TrimSpace(req.PreviousResponseID) != "" {
		return nil, errors.New("previous_response_id is not supported in responses fallback")
	}
	if len(req.Tools) > 0 || len(req.ToolChoice) > 0 || req.MaxToolCalls != nil {
		return nil, errors.New("tools are not supported in responses fallback")
	}
	if hasRawValue(req.Conversation) || hasRawValue(req.ContextManagement) || hasRawValue(req.Prompt) {
		return nil, errors.New("conversation, context_management and prompt are not supported in responses fallback")
	}

	messages := make([]dto.Message, 0)
	if len(req.Instructions) > 0 {
		instructions, err := rawMessageString(req.Instructions)
		if err != nil {
			return nil, fmt.Errorf("instructions must be a string in responses fallback: %w", err)
		}
		if strings.TrimSpace(instructions) != "" {
			messages = append(messages, dto.Message{
				Role:    "system",
				Content: instructions,
			})
		}
	}

	inputMessages, err := responsesInputToChatMessages(req.Input)
	if err != nil {
		return nil, err
	}
	if len(inputMessages) == 0 {
		return nil, errors.New("input is required in responses fallback")
	}
	messages = append(messages, inputMessages...)

	chatReq := &dto.GeneralOpenAIRequest{
		Model:                req.Model,
		Messages:             messages,
		Stream:               req.Stream,
		MaxTokens:            req.MaxOutputTokens,
		Temperature:          req.Temperature,
		TopP:                 req.TopP,
		User:                 req.User,
		Store:                req.Store,
		PromptCacheRetention: req.PromptCacheRetention,
		Metadata:             req.Metadata,
		SafetyIdentifier:     req.SafetyIdentifier,
		EnableThinking:       req.EnableThinking,
	}
	if len(req.PromptCacheKey) > 0 {
		chatReq.PromptCacheKey, _ = rawMessageString(req.PromptCacheKey)
	}
	if req.Reasoning != nil && strings.TrimSpace(req.Reasoning.Effort) != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}
	if strings.TrimSpace(req.ServiceTier) != "" {
		chatReq.ServiceTier, _ = common.Marshal(req.ServiceTier)
	}
	return chatReq, nil
}

func ChatCompletionsResponseToResponsesResponse(resp *dto.OpenAITextResponse, req *dto.OpenAIResponsesRequest) (*dto.OpenAIResponsesResponse, *dto.Usage, error) {
	if resp == nil {
		return nil, nil, errors.New("response is nil")
	}
	if err := resp.GetOpenAIError(); err != nil {
		return nil, nil, fmt.Errorf("%s", err.Message)
	}

	text := ""
	role := "assistant"
	finishReason := "stop"
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		role = strings.TrimSpace(choice.Message.Role)
		if role == "" {
			role = "assistant"
		}
		text = choice.Message.StringContent()
		if text == "" && choice.Message.Content != nil {
			if data, err := common.Marshal(choice.Message.Content); err == nil {
				text = string(data)
			}
		}
		if strings.TrimSpace(choice.FinishReason) != "" {
			finishReason = choice.FinishReason
		}
	}

	usage := resp.Usage
	if usage.InputTokens == 0 {
		usage.InputTokens = usage.PromptTokens
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = usage.CompletionTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if usage.InputTokensDetails == nil {
		details := usage.PromptTokensDetails
		usage.InputTokensDetails = &details
	}

	status := json.RawMessage(`"completed"`)
	if finishReason == "length" {
		status = json.RawMessage(`"incomplete"`)
	}

	id := strings.TrimSpace(resp.Id)
	if id == "" {
		id = "resp_" + common.GetRandomString(24)
	}
	model := resp.Model
	if model == "" && req != nil {
		model = req.Model
	}

	var maxOutputTokens int
	var temperature float64
	var topP float64
	var instructions json.RawMessage
	var previousResponseID json.RawMessage = json.RawMessage("null")
	var toolChoice json.RawMessage = json.RawMessage(`"auto"`)
	var truncation json.RawMessage = json.RawMessage(`"disabled"`)
	var user json.RawMessage
	var metadata json.RawMessage
	var store bool
	var reasoning *dto.Reasoning
	if req != nil {
		maxOutputTokens = int(lo.FromPtrOr(req.MaxOutputTokens, uint(0)))
		temperature = lo.FromPtrOr(req.Temperature, 0)
		topP = lo.FromPtrOr(req.TopP, 0)
		instructions = req.Instructions
		if req.PreviousResponseID != "" {
			previousResponseID, _ = common.Marshal(req.PreviousResponseID)
		}
		if len(req.ToolChoice) > 0 {
			toolChoice = req.ToolChoice
		}
		if len(req.Truncation) > 0 {
			truncation = req.Truncation
		}
		user = req.User
		metadata = req.Metadata
		store = rawMessageBool(req.Store)
		reasoning = req.Reasoning
	}

	out := &dto.OpenAIResponsesResponse{
		ID:                 id,
		Object:             "response",
		CreatedAt:          int(common.GetTimestamp()),
		Status:             status,
		Instructions:       instructions,
		MaxOutputTokens:    maxOutputTokens,
		Model:              model,
		ParallelToolCalls:  false,
		PreviousResponseID: previousResponseID,
		Reasoning:          reasoning,
		Store:              store,
		Temperature:        temperature,
		ToolChoice:         toolChoice,
		Tools:              []map[string]any{},
		TopP:               topP,
		Truncation:         truncation,
		Usage:              &usage,
		User:               user,
		Metadata:           metadata,
		Output: []dto.ResponsesOutput{
			{
				Type:   "message",
				ID:     "msg_" + common.GetRandomString(24),
				Status: "completed",
				Role:   role,
				Content: []dto.ResponsesOutputContent{
					{
						Type:        "output_text",
						Text:        text,
						Annotations: []interface{}{},
					},
				},
			},
		},
	}
	if finishReason == "length" {
		out.IncompleteDetails = &dto.IncompleteDetails{Reasoning: "max_output_tokens"}
		out.Output[0].Status = "incomplete"
	}
	return out, &usage, nil
}

func responsesInputToChatMessages(input json.RawMessage) ([]dto.Message, error) {
	if len(input) == 0 || common.GetJsonType(input) == "null" {
		return nil, nil
	}
	if common.GetJsonType(input) == "string" {
		text, err := rawMessageString(input)
		if err != nil {
			return nil, err
		}
		return []dto.Message{{Role: "user", Content: text}}, nil
	}
	if common.GetJsonType(input) != "array" {
		return nil, errors.New("input must be a string or array in responses fallback")
	}

	var items []responsesInputItem
	if err := common.Unmarshal(input, &items); err != nil {
		return nil, err
	}

	messages := make([]dto.Message, 0, len(items))
	var directParts []dto.MediaContent
	for _, item := range items {
		if item.Type == "input_text" || item.Type == "input_image" || item.Type == "input_file" {
			part, err := responsesInputPartToChatPart(item)
			if err != nil {
				return nil, err
			}
			directParts = append(directParts, part)
			continue
		}

		role := normalizeResponsesRole(item.Role)
		if len(item.Content) == 0 || common.GetJsonType(item.Content) == "null" {
			if strings.TrimSpace(item.Text) == "" {
				continue
			}
			messages = append(messages, dto.Message{Role: role, Content: item.Text})
			continue
		}
		switch common.GetJsonType(item.Content) {
		case "string":
			text, err := rawMessageString(item.Content)
			if err != nil {
				return nil, err
			}
			messages = append(messages, dto.Message{Role: role, Content: text})
		case "array":
			parts, err := responsesContentPartsToChatParts(item.Content)
			if err != nil {
				return nil, err
			}
			messages = append(messages, dto.Message{Role: role, Content: parts})
		default:
			return nil, errors.New("message content must be a string or array in responses fallback")
		}
	}
	if len(directParts) > 0 {
		messages = append([]dto.Message{{Role: "user", Content: directParts}}, messages...)
	}
	return messages, nil
}

func responsesContentPartsToChatParts(raw json.RawMessage) ([]dto.MediaContent, error) {
	var items []responsesInputItem
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	parts := make([]dto.MediaContent, 0, len(items))
	for _, item := range items {
		part, err := responsesInputPartToChatPart(item)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}

func responsesInputPartToChatPart(item responsesInputItem) (dto.MediaContent, error) {
	switch item.Type {
	case "input_text", "output_text":
		return dto.MediaContent{Type: dto.ContentTypeText, Text: item.Text}, nil
	case "input_image":
		imageURL := normalizeResponsesURL(item.ImageURL)
		if imageURL == "" {
			return dto.MediaContent{}, errors.New("input_image.image_url is required in responses fallback")
		}
		return dto.MediaContent{
			Type: dto.ContentTypeImageURL,
			ImageUrl: dto.MessageImageUrl{
				Url:    imageURL,
				Detail: item.Detail,
			},
		}, nil
	case "input_file":
		return dto.MediaContent{}, errors.New("input_file is not supported in responses fallback")
	default:
		return dto.MediaContent{}, fmt.Errorf("unsupported input content type %q in responses fallback", item.Type)
	}
}

func rawMessageString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || common.GetJsonType(raw) == "null" {
		return "", nil
	}
	if common.GetJsonType(raw) != "string" {
		return "", fmt.Errorf("got %s", common.GetJsonType(raw))
	}
	var out string
	if err := common.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out, nil
}

func rawMessageBool(raw json.RawMessage) bool {
	if len(raw) == 0 || common.GetJsonType(raw) != "bool" {
		return false
	}
	var out bool
	_ = common.Unmarshal(raw, &out)
	return out
}

func hasRawValue(raw json.RawMessage) bool {
	return len(raw) > 0 && common.GetJsonType(raw) != "null"
}

func normalizeResponsesRole(role string) string {
	switch strings.TrimSpace(role) {
	case "system", "developer", "assistant", "user":
		return strings.TrimSpace(role)
	default:
		return "user"
	}
}

func normalizeResponsesURL(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case map[string]any:
		return common.Interface2String(vv["url"])
	default:
		return ""
	}
}
