package util

import (
	"errors"

	"github.com/spf13/viper"
)

// Config holds all configuration of the application.
// Values are read from app.env and can be overridden by environment variables.
type Config struct {
	Environment       string `mapstructure:"ENVIRONMENT"`
	DBSource          string `mapstructure:"DB_SOURCE"`
	HTTPServerAddress string `mapstructure:"HTTP_SERVER_ADDRESS"`
	TracingEndpoint   string `mapstructure:"TRACING_ENDPOINT"`
}

// LoadConfig reads configuration from app.env in the given path.
// A missing file is OK (e.g. inside Docker) — env vars are used instead.
func LoadConfig(path string) (Config, error) {
	v := viper.New()
	v.AddConfigPath(path)
	v.SetConfigName("app")
	v.SetConfigType("env")

	// Defaults register every key, so AutomaticEnv can override each one.
	v.SetDefault("ENVIRONMENT", "development")
	v.SetDefault("DB_SOURCE", "")
	v.SetDefault("HTTP_SERVER_ADDRESS", "0.0.0.0:8080")
	v.SetDefault("TRACING_ENDPOINT", "") // empty = tracing disabled

	v.AutomaticEnv()

	var config Config
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return config, err
		}
	}

	err := v.Unmarshal(&config)
	return config, err
}
