package relay

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestShouldUseResponsesChatFallbackForOpenAICompatibleGeminiNamespace(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Tools: []byte(`[{"type":"namespace","name":"codex","tools":[{"type":"function","name":"run","inputSchema":{"type":"object"}}]}]`)}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: "gemini-3.1-flash-lite-preview",
	}}
	require.True(t, shouldUseResponsesChatFallback(info, request))
}

func TestShouldNotUseResponsesChatFallbackForPlainFunction(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Tools: []byte(`[{"type":"function","name":"run","parameters":{"type":"object"}}]`)}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeOpenAI, UpstreamModelName: "gemini-3.1-flash-lite-preview",
	}}
	require.False(t, shouldUseResponsesChatFallback(info, request))
}

func TestShouldFallbackResponsesConvertErrorForMissingConvertedContents(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Input: []byte(`[{"role":"user","content":"hello"}]`)}
	require.True(t, shouldFallbackResponsesConvertError(errors.New("contents is required"), request))
}

func TestShouldNotFallbackResponsesConvertErrorForActuallyMissingInput(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{}
	require.False(t, shouldFallbackResponsesConvertError(errors.New("contents is required"), request))
}

func TestShouldFallbackResponsesHTTPErrorPreservesBody(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Input: []byte(`"hello"`)}
	response := &http.Response{Body: io.NopCloser(strings.NewReader(`{"error":"contents is required"}`))}
	require.True(t, shouldFallbackResponsesHTTPError(response, request))
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "contents is required")
}
