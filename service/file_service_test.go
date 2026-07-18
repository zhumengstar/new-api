package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromBase64DataURL(t *testing.T) {
	cachedData, err := loadFromBase64("data:image/png;base64,aGVsbG8=", "")
	require.NoError(t, err)
	require.NotNil(t, cachedData)
	defer cachedData.Close()

	assert.Equal(t, "image/png", cachedData.MimeType)
	assert.Equal(t, int64(5), cachedData.Size)
	base64Data, err := cachedData.GetBase64Data()
	require.NoError(t, err)
	assert.Equal(t, "aGVsbG8=", base64Data)
}

func TestLoadFromBase64ToleratesCompatibilityPrefixesAndWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSize int64
		wantData string
	}{
		{name: "compatibility prefix and whitespace", input: " base64:data:image/png;base64,aG\nVs\u00a0bG8=\t ", wantSize: 5, wantData: "aGVsbG8="},
		{name: "raw standard encoding", input: "data:image/png;base64,aGVsbG8", wantSize: 5, wantData: "aGVsbG8="},
		{name: "URL-safe encoding", input: "data:image/png;base64,-_8=", wantSize: 2, wantData: "+/8="},
		{name: "percent-escaped encoding", input: "data:image/png;base64,aGVsbG8%3D", wantSize: 5, wantData: "aGVsbG8="},
		{name: "case-insensitive data URL", input: "DATA:image/png;base64,aGVsbG8=", wantSize: 5, wantData: "aGVsbG8="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cachedData, err := loadFromBase64(tt.input, "")
			require.NoError(t, err)
			require.NotNil(t, cachedData)
			defer cachedData.Close()

			assert.Equal(t, "image/png", cachedData.MimeType)
			assert.Equal(t, tt.wantSize, cachedData.Size)
			base64Data, err := cachedData.GetBase64Data()
			require.NoError(t, err)
			assert.Equal(t, tt.wantData, base64Data)
		})
	}
}

func TestLoadFromBase64RejectsInvalidPayload(t *testing.T) {
	_, err := loadFromBase64("data:image/png;base64,not!base64", "")
	require.ErrorContains(t, err, "failed to decode base64 data")
}
