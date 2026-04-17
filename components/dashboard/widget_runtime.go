package dashboard

import (
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const widgetViewModelMetadataKey = "dashboard.widget.view_model"

// WidgetViewModel is the explicit serialization boundary for widget-specific
// view data. Framework code stores view models internally and serializes them
// only when assembling the final page/transport payload.
type WidgetViewModel interface {
	Serialize() (any, error)
}

// JSONViewModel wraps a typed view value and exposes it through the standard
// WidgetViewModel serialization boundary.
type JSONViewModel[T any] struct {
	Value T
}

// Serialize returns the wrapped typed view as the transport payload.
func (view JSONViewModel[T]) Serialize() (any, error) {
	return view.Value, nil
}

// WidgetRequest carries typed widget config alongside the runtime context used
// to fetch provider data.
type WidgetRequest[TConfig any] struct {
	Instance   WidgetInstance
	Viewer     ViewerContext
	Options    map[string]any
	Translator TranslationService
	Theme      *ThemeSelection
	Config     TConfig
}

// WidgetViewContext carries the typed config/runtime context into view-model
// construction.
type WidgetViewContext[TConfig any] struct {
	Request WidgetRequest[TConfig]
}

// WidgetSpec is the typed widget authoring contract used by built-in and
// application-defined widgets before runtime erasure.
type WidgetSpec[TConfig any, TData any, TView WidgetViewModel] struct {
	Definition   WidgetDefinition
	DecodeConfig func(raw map[string]any) (TConfig, error)
	Fetch        func(ctx context.Context, req WidgetRequest[TConfig]) (TData, error)
	BuildView    func(ctx context.Context, data TData, meta WidgetViewContext[TConfig]) (TView, error)
}

type widgetSpecRuntime interface {
	Code() string
	Definition() WidgetDefinition
	Resolve(ctx context.Context, meta WidgetContext) (ResolvedWidget, error)
}

// ResolvedWidget is the erased runtime result held by the framework until the
// page assembly/transport boundary needs serialized widget data.
type ResolvedWidget struct {
	Frame WidgetFrame
	View  WidgetViewModel
}

type typedWidgetRuntime[TConfig any, TData any, TView WidgetViewModel] struct {
	spec WidgetSpec[TConfig, TData, TView]
}

// NewWidgetRuntime erases a typed widget spec into the runtime contract used by
// the dashboard framework.
func NewWidgetRuntime[TConfig any, TData any, TView WidgetViewModel](spec WidgetSpec[TConfig, TData, TView]) widgetSpecRuntime {
	return typedWidgetRuntime[TConfig, TData, TView]{spec: spec}
}

// NewWidgetProvider adapts a typed widget spec back to the legacy Provider
// interface for compatibility with older callers and tests.
func NewWidgetProvider[TConfig any, TData any, TView WidgetViewModel](spec WidgetSpec[TConfig, TData, TView]) Provider {
	return runtimeProviderAdapter{runtime: NewWidgetRuntime(spec)}
}

func (runtime typedWidgetRuntime[TConfig, TData, TView]) Code() string {
	return runtime.spec.Definition.Code
}

func (runtime typedWidgetRuntime[TConfig, TData, TView]) Definition() WidgetDefinition {
	return cloneWidgetDefinition(runtime.spec.Definition)
}

func (runtime typedWidgetRuntime[TConfig, TData, TView]) Resolve(ctx context.Context, meta WidgetContext) (ResolvedWidget, error) {
	decode := runtime.spec.DecodeConfig
	if decode == nil {
		decode = DecodeWidgetConfig[TConfig]
	}
	cfg, err := decode(meta.Instance.Configuration)
	if err != nil {
		return ResolvedWidget{}, err
	}
	req := WidgetRequest[TConfig]{
		Instance:   meta.Instance,
		Viewer:     meta.Viewer,
		Options:    meta.Options,
		Translator: meta.Translator,
		Theme:      meta.Theme,
		Config:     cfg,
	}
	data, err := runtime.spec.Fetch(ctx, req)
	if err != nil {
		return ResolvedWidget{}, err
	}
	view, err := runtime.spec.BuildView(ctx, data, WidgetViewContext[TConfig]{Request: req})
	if err != nil {
		return ResolvedWidget{}, err
	}
	return ResolvedWidget{View: view}, nil
}

type providerRuntimeAdapter struct {
	code     string
	def      WidgetDefinition
	provider Provider
}

func newProviderRuntimeAdapter(code string, def WidgetDefinition, provider Provider) widgetSpecRuntime {
	return providerRuntimeAdapter{code: code, def: def, provider: provider}
}

func (adapter providerRuntimeAdapter) Code() string {
	if adapter.code != "" {
		return adapter.code
	}
	return adapter.def.Code
}

func (adapter providerRuntimeAdapter) Definition() WidgetDefinition {
	return cloneWidgetDefinition(adapter.def)
}

func (adapter providerRuntimeAdapter) Resolve(ctx context.Context, meta WidgetContext) (ResolvedWidget, error) {
	if adapter.provider == nil {
		return ResolvedWidget{}, fmt.Errorf("dashboard: provider is nil")
	}
	data, err := adapter.provider.Fetch(ctx, meta)
	if err != nil {
		return ResolvedWidget{}, err
	}
	return ResolvedWidget{View: data}, nil
}

type runtimeProviderAdapter struct {
	runtime widgetSpecRuntime
}

func (adapter runtimeProviderAdapter) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	if adapter.runtime == nil {
		return nil, fmt.Errorf("dashboard: widget runtime is nil")
	}
	resolved, err := adapter.runtime.Resolve(ctx, meta)
	if err != nil {
		return nil, err
	}
	if resolved.View == nil {
		return nil, nil
	}
	payload, err := resolved.View.Serialize()
	if err != nil {
		return nil, err
	}
	return serializedWidgetData(payload)
}

// DecodeWidgetConfig decodes a generic widget configuration map into a typed
// config struct/value using JSON round-tripping.
func DecodeWidgetConfig[T any](raw map[string]any) (T, error) {
	var out T
	if len(raw) == 0 {
		return out, nil
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return out, err
	}
	return out, nil
}

func serializedWidgetData(payload any) (WidgetData, error) {
	switch typed := payload.(type) {
	case nil:
		return nil, nil
	case WidgetData:
		return typed, nil
	case map[string]any:
		return WidgetData(typed), nil
	default:
		normalized, err := normalizeStructuredValue(payload)
		if err != nil {
			return nil, err
		}
		out, ok := normalized.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("dashboard: serialized widget payload must be an object, got %T", normalized)
		}
		return WidgetData(out), nil
	}
}

func normalizeStructuredValue(value any) (any, error) {
	return normalizeStructuredReflectValue(reflect.ValueOf(value))
}

func normalizeStructuredReflectValue(value reflect.Value) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}
	normalized, handled, err := normalizeCustomMarshaledValue(value)
	if handled || err != nil {
		return normalized, err
	}
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
		normalized, handled, err = normalizeCustomMarshaledValue(value)
		if handled || err != nil {
			return normalized, err
		}
	}

	switch value.Kind() {
	case reflect.Struct:
		result := map[string]any{}
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name, omitEmpty, skip := jsonFieldName(field)
			if skip {
				continue
			}
			nested, err := normalizeStructuredReflectValue(value.Field(i))
			if err != nil {
				return nil, err
			}
			if omitEmpty && value.Field(i).IsZero() {
				continue
			}
			result[name] = nested
		}
		return result, nil
	case reflect.Map:
		if value.IsNil() {
			return nil, nil
		}
		result := map[string]any{}
		iter := value.MapRange()
		for iter.Next() {
			nested, err := normalizeStructuredReflectValue(iter.Value())
			if err != nil {
				return nil, err
			}
			result[fmt.Sprint(iter.Key().Interface())] = nested
		}
		return result, nil
	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return nil, nil
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return value.Bytes(), nil
		}
		items := make([]any, value.Len())
		allMaps := true
		for i := 0; i < value.Len(); i++ {
			nested, err := normalizeStructuredReflectValue(value.Index(i))
			if err != nil {
				return nil, err
			}
			items[i] = nested
			if _, ok := items[i].(map[string]any); !ok {
				allMaps = false
			}
		}
		if allMaps {
			out := make([]map[string]any, len(items))
			for i := range items {
				out[i] = items[i].(map[string]any)
			}
			return out, nil
		}
		return items, nil
	default:
		return value.Interface(), nil
	}
}

func normalizeCustomMarshaledValue(value reflect.Value) (any, bool, error) {
	for _, candidate := range marshalerCandidates(value) {
		if !candidate.IsValid() || !candidate.CanInterface() {
			continue
		}
		if marshaler, ok := candidate.Interface().(json.Marshaler); ok {
			payload, err := marshaler.MarshalJSON()
			if err != nil {
				return nil, true, err
			}
			var decoded any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				return nil, true, err
			}
			return normalizeSerializedValue(decoded), true, nil
		}
		if marshaler, ok := candidate.Interface().(encoding.TextMarshaler); ok {
			payload, err := marshaler.MarshalText()
			if err != nil {
				return nil, true, err
			}
			return string(payload), true, nil
		}
	}
	return nil, false, nil
}

func marshalerCandidates(value reflect.Value) []reflect.Value {
	candidates := []reflect.Value{value}
	if value.Kind() == reflect.Pointer {
		return candidates
	}
	if value.CanAddr() {
		candidates = append(candidates, value.Addr())
		return candidates
	}
	copy := reflect.New(value.Type())
	copy.Elem().Set(value)
	return append(candidates, copy)
}

func jsonFieldName(field reflect.StructField) (name string, omitEmpty bool, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = field.Name
	}
	for _, option := range parts[1:] {
		if option == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func normalizeSerializedValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			out[key] = normalizeSerializedValue(nested)
		}
		return out
	case []any:
		items := make([]any, len(typed))
		allMaps := true
		for i, nested := range typed {
			items[i] = normalizeSerializedValue(nested)
			if _, ok := items[i].(map[string]any); !ok {
				allMaps = false
			}
		}
		if allMaps {
			out := make([]map[string]any, len(items))
			for i := range items {
				out[i] = items[i].(map[string]any)
			}
			return out
		}
		return items
	default:
		return value
	}
}
