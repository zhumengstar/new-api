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
	cachedData, err := loadFromBase64(" base64:data:image/png;base64,aG\nVs bG8=\t ", "")
	require.NoError(t, err)
	require.NotNil(t, cachedData)
	defer cachedData.Close()

	assert.Equal(t, "image/png", cachedData.MimeType)
	assert.Equal(t, int64(5), cachedData.Size)
	base64Data, err := cachedData.GetBase64Data()
	require.NoError(t, err)
	assert.Equal(t, "aGVsbG8=", base64Data)
}
