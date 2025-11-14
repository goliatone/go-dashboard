package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// ConfigValidator validates widget configuration payloads against their schema.
type ConfigValidator interface {
	Validate(def WidgetDefinition, config map[string]any) error
}

// JSONSchemaValidator compiles widget schemas and validates configuration maps.
type JSONSchemaValidator struct {
	mu       sync.RWMutex
	compiled map[string]*jsonschema.Schema
}

// NewJSONSchemaValidator builds a validator backed by jsonschema v5.
func NewJSONSchemaValidator() *JSONSchemaValidator {
	return &JSONSchemaValidator{
		compiled: make(map[string]*jsonschema.Schema),
	}
}

// Validate ensures the provided configuration satisfies the widget schema.
func (v *JSONSchemaValidator) Validate(def WidgetDefinition, config map[string]any) error {
	if len(def.Schema) == 0 {
		return nil
	}
	schema, err := v.schemaFor(def)
	if err != nil {
		return err
	}
	var payload map[string]any
	if config == nil {
		payload = map[string]any{}
	} else {
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("dashboard: marshal config for %s: %w", def.Code, err)
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("dashboard: normalize config for %s: %w", def.Code, err)
		}
	}
	if err := schema.Validate(payload); err != nil {
		return fmt.Errorf("dashboard: configuration for %s failed validation: %w", def.Code, err)
	}
	return nil
}

func (v *JSONSchemaValidator) schemaFor(def WidgetDefinition) (*jsonschema.Schema, error) {
	v.mu.RLock()
	schema, ok := v.compiled[def.Code]
	v.mu.RUnlock()
	if ok {
		return schema, nil
	}
	data, err := json.Marshal(def.Schema)
	if err != nil {
		return nil, fmt.Errorf("dashboard: marshal schema %s: %w", def.Code, err)
	}
	compiler := jsonschema.NewCompiler()
	name := def.Code + ".json"
	if err := compiler.AddResource(name, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("dashboard: load schema %s: %w", def.Code, err)
	}
	compiled, err := compiler.Compile(name)
	if err != nil {
		return nil, fmt.Errorf("dashboard: compile schema %s: %w", def.Code, err)
	}
	v.mu.Lock()
	v.compiled[def.Code] = compiled
	v.mu.Unlock()
	return compiled, nil
}

type noopConfigValidator struct{}

func (noopConfigValidator) Validate(WidgetDefinition, map[string]any) error { return nil }
