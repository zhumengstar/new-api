package relay

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

type responsesFallbackInputItem struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type responsesFallbackContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL any    `json:"image_url"`
	FileURL  any    `json:"file_url"`
}

func responsesFallbackDiagnostic(request *dto.OpenAIResponsesRequest, convertedRequest any) string {
	inputType := "missing"
	inputItems := 0
	messageItems := 0
	functionCalls := 0
	functionOutputs := 0
	contentParts := 0
	userMessages := 0
	systemMessages := 0
	assistantMessages := 0
	otherRoles := 0
	nonEmptyTextParts := 0
	imageParts := 0
	fileParts := 0
	if request != nil && len(request.Input) > 0 {
		inputType = common.GetJsonType(request.Input)
		if inputType == "array" {
			var items []responsesFallbackInputItem
			if common.Unmarshal(request.Input, &items) == nil {
				inputItems = len(items)
				for _, item := range items {
					switch strings.TrimSpace(item.Role) {
					case "user":
						userMessages++
					case "system", "developer":
						systemMessages++
					case "assistant":
						assistantMessages++
					case "":
					default:
						otherRoles++
					}
					switch item.Type {
					case "function_call":
						functionCalls++
					case "function_call_output":
						functionOutputs++
					default:
						messageItems++
					}
					if common.GetJsonType(item.Content) == "array" {
						var parts []responsesFallbackContentPart
						if common.Unmarshal(item.Content, &parts) == nil {
							contentParts += len(parts)
							for _, part := range parts {
								if strings.TrimSpace(part.Text) != "" {
									nonEmptyTextParts++
								}
								switch part.Type {
								case "input_image":
									imageParts++
								case "input_file":
									fileParts++
								}
							}
						}
					}
				}
			}
		}
	}

	tools := 0
	if request != nil && len(request.Tools) > 0 && common.GetJsonType(request.Tools) == "array" {
		var values []json.RawMessage
		if common.Unmarshal(request.Tools, &values) == nil {
			tools = len(values)
		}
	}

	geminiContents := 0
	geminiParts := 0
	geminiFunctionCalls := 0
	geminiFunctionOutputs := 0
	blankFunctionNames := 0
	geminiTools := 0
	convertedType := fmt.Sprintf("%T", convertedRequest)
	var geminiRequest *dto.GeminiChatRequest
	switch value := convertedRequest.(type) {
	case *dto.GeminiChatRequest:
		geminiRequest = value
	case dto.GeminiChatRequest:
		geminiRequest = &value
	}
	if geminiRequest != nil {
		geminiContents = len(geminiRequest.Contents)
		for _, content := range geminiRequest.Contents {
			geminiParts += len(content.Parts)
			for _, part := range content.Parts {
				if part.FunctionCall != nil {
					geminiFunctionCalls++
					if strings.TrimSpace(part.FunctionCall.FunctionName) == "" {
						blankFunctionNames++
					}
				}
				if part.FunctionResponse != nil {
					geminiFunctionOutputs++
					if strings.TrimSpace(part.FunctionResponse.Name) == "" {
						blankFunctionNames++
					}
				}
			}
		}
		geminiTools = len(geminiRequest.GetTools())
	}

	return fmt.Sprintf(
		"input_type=%s input_items=%d messages=%d roles_user=%d roles_system=%d roles_assistant=%d roles_other=%d content_parts=%d nonempty_text_parts=%d image_parts=%d file_parts=%d function_calls=%d function_outputs=%d tools=%d converted_type=%s gemini_contents=%d gemini_parts=%d gemini_function_calls=%d gemini_function_outputs=%d gemini_tools=%d blank_function_names=%d",
		inputType, inputItems, messageItems, userMessages, systemMessages, assistantMessages, otherRoles,
		contentParts, nonEmptyTextParts, imageParts, fileParts, functionCalls, functionOutputs, tools,
		convertedType, geminiContents, geminiParts, geminiFunctionCalls, geminiFunctionOutputs, geminiTools, blankFunctionNames,
	)
}
