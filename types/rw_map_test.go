package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromJsonStringKeepsExistingDataOnInvalidJSON(t *testing.T) {
	m := NewRWMap[string, int]()
	m.Set("keep", 1)

	err := LoadFromJsonString(m, "")

	require.Error(t, err)
	value, ok := m.Get("keep")
	require.True(t, ok)
	require.Equal(t, 1, value)
}

func TestLoadFromJsonStringReplacesDataAfterSuccessfulParse(t *testing.T) {
	m := NewRWMap[string, int]()
	m.Set("old", 1)

	require.NoError(t, LoadFromJsonString(m, `{"new":2}`))
	_, oldExists := m.Get("old")
	value, newExists := m.Get("new")
	require.False(t, oldExists)
	require.True(t, newExists)
	require.Equal(t, 2, value)
}
