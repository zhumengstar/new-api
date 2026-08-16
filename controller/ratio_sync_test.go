package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDifferencesOnlyIncludesEnabledChannelModels(t *testing.T) {
	localData := map[string]any{
		"model_ratio": map[string]any{
			"enabled-model":  1.0,
			"disabled-model": 1.0,
		},
	}
	successfulChannels := []struct {
		name string
		data map[string]any
	}{
		{
			name: "custom-price",
			data: map[string]any{
				"model_ratio": map[string]any{
					"enabled-model":      2.0,
					"disabled-model":     2.0,
					"unconfigured-model": 2.0,
				},
			},
		},
	}

	differences := buildDifferences(localData, successfulChannels, map[string]struct{}{
		"enabled-model": {},
	})

	require.Contains(t, differences, "enabled-model")
	require.NotContains(t, differences, "disabled-model")
	require.NotContains(t, differences, "unconfigured-model")
}
