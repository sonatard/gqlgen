package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONConfig(t *testing.T) {
	for _, version := range []string{"", "v1", "v2"} {
		require.NoError(t, JSONConfig{Version: version}.Check(), "version %q", version)
	}
	require.EqualError(t, JSONConfig{Version: "v3"}.Check(), `unknown version "v3": use v1 or v2`)

	assert.False(t, JSONConfig{}.IsV2())
	assert.False(t, JSONConfig{Version: "v1"}.IsV2())
	assert.True(t, JSONConfig{Version: "v2"}.IsV2())
}
