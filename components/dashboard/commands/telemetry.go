package commands

import "context"

// Telemetry allows commands to emit structured events.
type Telemetry interface {
	Record(ctx context.Context, event string, payload map[string]any)
}

type noopTelemetry struct{}

func (noopTelemetry) Record(context.Context, string, map[string]any) {}

func normalizeTelemetry(t Telemetry) Telemetry {
	if t == nil {
		return noopTelemetry{}
	}
	return t
}
