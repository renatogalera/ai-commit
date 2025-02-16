package config

import (
	"reflect"
	"strings"
)

// ConfigManager handles merging CLI flags into the Config struct.
// Priority: CLI flags > Config file > Default values.
type ConfigManager struct {
	Config *Config
	Flags  map[string]interface{}
}

// NewConfigManager creates a new ConfigManager instance.
func NewConfigManager(cfg *Config) *ConfigManager {
	return &ConfigManager{
		Config: cfg,
		Flags:  make(map[string]interface{}),
	}
}

// RegisterFlag registers a CLI flag value with the corresponding config key.
// The key should match the YAML tag used in the Config struct.
func (cm *ConfigManager) RegisterFlag(key string, value interface{}) {
	cm.Flags[key] = value
}

// MergeConfiguration uses reflection to merge CLI flag values into the Config struct.
// It only overrides fields if the CLI flag value is non-zero.
func (cm *ConfigManager) MergeConfiguration() *Config {
	configValue := reflect.ValueOf(cm.Config).Elem()
	configType := configValue.Type()

	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" {
			continue
		}
		// Extract the field name from the YAML tag (before any comma)
		configFieldName := strings.Split(yamlTag, ",")[0]
		if flagValue, exists := cm.Flags[configFieldName]; exists && !isZeroValue(reflect.ValueOf(flagValue)) {
			fieldValue := configValue.Field(i)
			if fieldValue.CanSet() {
				flagVal := reflect.ValueOf(flagValue)
				// Convert flag value to the field type if necessary.
				if flagVal.Type().ConvertibleTo(fieldValue.Type()) {
					fieldValue.Set(flagVal.Convert(fieldValue.Type()))
				}
			}
		}
	}
	return cm.Config
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	default:
		return v.IsZero()
	}
}

