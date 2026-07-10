package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiPartUnmarshalSnakeCaseFileData(t *testing.T) {
	var request GeminiChatRequest
	err := common.Unmarshal([]byte(`{
		"contents": [
			{
				"role": "user",
				"parts": [
					{
						"file_data": {
							"mime_type": "video/quicktime",
							"file_uri": "https://example.com/video.mov"
						}
					}
				]
			}
		]
	}`), &request)

	require.NoError(t, err)
	require.Len(t, request.Contents, 1)
	require.Len(t, request.Contents[0].Parts, 1)

	fileData := request.Contents[0].Parts[0].FileData
	require.NotNil(t, fileData)
	assert.Equal(t, "video/quicktime", fileData.MimeType)
	assert.Equal(t, "https://example.com/video.mov", fileData.FileUri)
}

func TestGeminiPartUnmarshalSnakeCaseOneofAliases(t *testing.T) {
	var part GeminiPart
	err := common.Unmarshal([]byte(`{
		"function_call": {
			"name": "lookup",
			"args": {"q": "test"}
		},
		"video_metadata": {"start_offset": "1s"},
		"media_resolution": {"level": "high"}
	}`), &part)

	require.NoError(t, err)
	require.NotNil(t, part.FunctionCall)
	assert.Equal(t, "lookup", part.FunctionCall.FunctionName)
	assert.JSONEq(t, `{"start_offset":"1s"}`, string(part.VideoMetadata))
	assert.JSONEq(t, `{"level":"high"}`, string(part.MediaResolution))
}
