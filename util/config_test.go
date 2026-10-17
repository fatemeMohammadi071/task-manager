package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromFile(t *testing.T) {
	config, err := LoadConfig("..") // app.env is in the project root
	require.NoError(t, err)
	require.NotEmpty(t, config.DBSource)
	require.Equal(t, "0.0.0.0:8080", config.HTTPServerAddress)
}

func TestLoadConfigEnvOverride(t *testing.T) {
	t.Setenv("HTTP_SERVER_ADDRESS", "0.0.0.0:9999")

	config, err := LoadConfig("..")
	require.NoError(t, err)
	require.Equal(t, "0.0.0.0:9999", config.HTTPServerAddress)
}

func TestLoadConfigNoFile(t *testing.T) {
	t.Setenv("DB_SOURCE", "postgresql://from-env")

	config, err := LoadConfig(t.TempDir()) // empty dir → no app.env
	require.NoError(t, err)
	require.Equal(t, "postgresql://from-env", config.DBSource)
}
