package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGeminiMimeType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "quicktime mov alias",
			input:    " video/quicktime ",
			expected: "video/mov",
		},
		{
			name:     "wave audio alias",
			input:    "audio/wave",
			expected: "audio/wav",
		},
		{
			name:     "upper case supported type",
			input:    "IMAGE/JPEG",
			expected: "image/jpeg",
		},
		{
			name:     "supported type unchanged",
			input:    "video/mp4",
			expected: "video/mp4",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, normalizeGeminiMimeType(test.input))
		})
	}
}

func TestNormalizeGeminiMimeTypeAliasesAreSupported(t *testing.T) {
	for alias := range geminiMimeTypeAliases {
		normalized := normalizeGeminiMimeType(alias)
		require.Truef(t, geminiSupportedMimeTypes[normalized], "alias %s normalized to unsupported MIME %s", alias, normalized)
	}
}
