package dashboard

import "testing"

func TestJSONSchemaValidatorRejectsInvalidPayload(t *testing.T) {
	validator := NewJSONSchemaValidator()
	def := WidgetDefinition{
		Code: "demo.widget.string_required",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "minLength": 1},
			},
		},
	}
	if err := validator.Validate(def, map[string]any{"name": "Dashboard"}); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
	if err := validator.Validate(def, map[string]any{}); err == nil {
		t.Fatalf("expected validation error for missing name")
	}
}

func TestJSONSchemaValidatorCachesCompiledSchemas(t *testing.T) {
	validator := NewJSONSchemaValidator()
	def := WidgetDefinition{
		Code:   "demo.widget.cache",
		Schema: map[string]any{"type": "object"},
	}
	if err := validator.Validate(def, nil); err != nil {
		t.Fatalf("unexpected error validating config: %v", err)
	}
	if len(validator.compiled) != 1 {
		t.Fatalf("expected schema cache to contain 1 entry, got %d", len(validator.compiled))
	}
	if err := validator.Validate(def, map[string]any{}); err != nil {
		t.Fatalf("unexpected error on cached validation: %v", err)
	}
	if len(validator.compiled) != 1 {
		t.Fatalf("expected schema cache to remain 1 entry, got %d", len(validator.compiled))
	}
}
