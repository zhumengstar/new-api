package gemini

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesFallbackMediaContentConvertsToGeminiContents(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Model: "gemini-test",
		Input: json.RawMessage(`[
			{"role":"system","content":[{"type":"input_text","text":"be concise"}]},
			{"role":"user","content":[
				{"type":"input_text","text":"describe this"},
				{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}
			]}
		]`),
	}

	chatRequest, err := openaicompat.ResponsesRequestToChatCompletionsRequest(request)
	require.NoError(t, err)
	require.Len(t, chatRequest.Messages, 2)
	assert.Equal(t, "be concise", chatRequest.Messages[0].StringContent())
	require.Len(t, chatRequest.Messages[1].ParseContent(), 2)

	geminiRequest, err := CovertOpenAI2Gemini(&gin.Context{}, *chatRequest, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeGemini,
			UpstreamModelName: "gemini-test",
		},
	})
	require.NoError(t, err)
	require.Len(t, geminiRequest.Contents, 1)
	require.Len(t, geminiRequest.Contents[0].Parts, 2)
	assert.Equal(t, "describe this", geminiRequest.Contents[0].Parts[0].Text)
	assert.NotNil(t, geminiRequest.Contents[0].Parts[1].InlineData)
	assert.Equal(t, "image/png", geminiRequest.Contents[0].Parts[1].InlineData.MimeType)
	assert.NotNil(t, geminiRequest.SystemInstructions)
}
