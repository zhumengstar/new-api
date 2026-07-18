package relay

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

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
