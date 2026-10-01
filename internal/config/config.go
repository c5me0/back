package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/rs/zerolog"
)

// Version is the application version
// Injected at build time
var Version = "local"

const (
	defaultConfigDir = "/config"
	configFileName   = "config.json"
)

// Config represents the application configuration
type Config struct {
	LogLevel string `json:"log_level"`

	DB        *DB        `json:"database"`
	Token     *Token     `json:"token"`
	Service   *Service   `json:"service"`
	Storage   *Storage   `json:"storage"`
	WebRTC    *WebRTC    `json:"webrtc"`
	Recording *Recording `json:"recording"`

	Twilio     *Twilio     `json:"twilio" config:"optional"`
	OTP        *OTP        `json:"otp" config:"optional"`
	OpenAI     *OpenAI     `json:"openai" config:"optional"`
	Push       *Push       `json:"push" config:"optional"`
	RevenueCat *RevenueCat `json:"revenuecat" config:"optional"`
	Quota      *Quota      `json:"quota" config:"optional"`
}

// Level returns the parsed zerolog level from LogLevel, defaulting to InfoLevel
// when LogLevel is empty or unparseable.
func (c *Config) Level() zerolog.Level {
	level, err := zerolog.ParseLevel(c.LogLevel)
	if err != nil || c.LogLevel == "" {
		return zerolog.InfoLevel
	}
	return level
}

// Validate ensures all required sections are present and every present section is valid.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	value := reflect.ValueOf(c).Elem()
	for i := range value.NumField() {
		field := value.Field(i)
		fieldType := value.Type().Field(i)
		if field.Kind() != reflect.Pointer {
			continue
		}

		if field.IsNil() {
			if fieldType.Tag.Get("config") == "optional" {
				continue
			}
			return fmt.Errorf("missing %s configuration", fieldType.Name)
		}

		if validator, ok := reflect.TypeAssert[ValidatableConfig](field); ok {
			if err := validator.Validate(); err != nil {
				return fmt.Errorf("invalid %s configuration: %w", fieldType.Name, err)
			}
		}
	}

	return nil
}

// LoadConfig reads and parses ${CONFIG_DIR:-/config}/config.json.
func LoadConfig() (*Config, error) {
	directory := os.Getenv("CONFIG_DIR")
	if directory == "" {
		directory = defaultConfigDir
	}

	configFile, err := os.ReadFile(filepath.Join(directory, configFileName)) //nolint:gosec // G703: CONFIG_DIR is set by the operator
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err = json.Unmarshal(configFile, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	// Optional sections with defaults are materialized so callers never nil-check them.
	if config.OTP == nil {
		config.OTP = &OTP{}
	}

	if err = config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

type ValidatableConfig interface {
	Validate() error
}
