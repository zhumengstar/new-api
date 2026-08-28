package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestParseTaskResultAcceptsStringDuration(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"id": "cgt-test",
		"status": "succeeded",
		"content": {"video_url": "https://example.com/video.mp4"},
		"duration": "15",
		"usage": {"completion_tokens": 12, "total_tokens": 34}
	}`))

	require.NoError(t, err)
	require.Equal(t, model.TaskStatusSuccess, result.Status)
	require.Equal(t, "https://example.com/video.mp4", result.Url)
	require.Equal(t, 12, result.CompletionTokens)
	require.Equal(t, 34, result.TotalTokens)
}
