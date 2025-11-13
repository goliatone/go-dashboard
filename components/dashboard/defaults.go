package dashboard

import "time"

var defaultAreaDefinitions = []WidgetAreaDefinition{
	{Code: "admin.dashboard.main", Name: "Admin Dashboard (Main)", Description: "Primary dashboard canvas"},
	{Code: "admin.dashboard.sidebar", Name: "Admin Dashboard (Sidebar)", Description: "Secondary widgets"},
	{Code: "admin.dashboard.footer", Name: "Admin Dashboard (Footer)", Description: "Footer widgets"},
}

var defaultWidgetDefinitions = []WidgetDefinition{
	{
		Code: "admin.widget.user_stats",
		Name: "User Statistics",
		NameLocalized: map[string]string{
			"es": "Estadísticas de usuarios",
		},
		Description: "High-level user metrics",
		DescriptionLocalized: map[string]string{
			"es": "Métricas clave de usuarios",
		},
		Category: "stats",
		Schema: map[string]any{
			"type":       "object",
			"required":   []string{"metric"},
			"properties": map[string]any{"metric": map[string]any{"type": "string", "enum": []string{"total", "active", "new"}}},
		},
	},
	{
		Code: "admin.widget.recent_activity",
		Name: "Recent Activity",
		NameLocalized: map[string]string{
			"es": "Actividad reciente",
		},
		Description: "Latest activity feed entries",
		DescriptionLocalized: map[string]string{
			"es": "Últimos eventos registrados",
		},
		Category: "activity",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "default": 10},
			},
		},
	},
	{
		Code:        "admin.widget.sales_chart",
		Name:        "Sales Chart",
		Description: "Sales overview chart",
		Category:    "charts",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"range": map[string]any{"type": "string", "enum": []string{"7d", "30d", "90d"}, "default": "30d"},
			},
		},
	},
	{
		Code: "admin.widget.quick_actions",
		Name: "Quick Actions",
		NameLocalized: map[string]string{
			"es": "Acciones rápidas",
		},
		Description: "Common admin shortcuts",
		Category:    "actions",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"actions": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "object"},
				},
			},
		},
	},
	{
		Code: "admin.widget.system_status",
		Name: "System Status",
		NameLocalized: map[string]string{
			"es": "Estado del sistema",
		},
		Description: "Health indicators",
		DescriptionLocalized: map[string]string{
			"es": "Indicadores de salud",
		},
		Category: "status",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"checks": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
		},
	},
	{
		Code:        "admin.widget.analytics_funnel",
		Name:        "Conversion Funnel",
		Description: "Tracks drop-off through key funnel stages.",
		Category:    "analytics",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"range": map[string]any{
					"type":    "string",
					"enum":    []string{"7d", "14d", "30d", "90d", "180d"},
					"default": "30d",
				},
				"segment": map[string]any{
					"type":      "string",
					"minLength": 1,
				},
				"goal": map[string]any{
					"type":    "number",
					"minimum": 0,
					"maximum": 100,
					"default": 45,
				},
			},
			"additionalProperties": false,
		},
	},
	{
		Code:        "admin.widget.cohort_overview",
		Name:        "Cohort Overview",
		Description: "Retention grid grouped by signup cohort.",
		Category:    "analytics",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"interval": map[string]any{
					"type":    "string",
					"enum":    []string{"weekly", "monthly"},
					"default": "weekly",
				},
				"periods": map[string]any{
					"type":    "integer",
					"minimum": 4,
					"maximum": 12,
					"default": 8,
				},
				"metric": map[string]any{
					"type":    "string",
					"enum":    []string{"active", "retained", "upgraded"},
					"default": "retained",
				},
			},
			"additionalProperties": false,
		},
	},
	{
		Code:        "admin.widget.alert_trends",
		Name:        "Alert Trends",
		Description: "Compares alert volume/severity over time.",
		Category:    "analytics",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"lookback_days": map[string]any{
					"type":    "integer",
					"minimum": 7,
					"maximum": 90,
					"default": 30,
				},
				"severity": map[string]any{
					"type":     "array",
					"minItems": 1,
					"items": map[string]any{
						"type": "string",
						"enum": []string{"info", "warning", "critical"},
					},
					"uniqueItems": true,
					"default":     []string{"warning", "critical"},
				},
				"service": map[string]any{
					"type": "string",
				},
			},
			"additionalProperties": false,
		},
	},
}

var defaultSeedConfigs = []AddWidgetRequest{
	{
		DefinitionID:  "admin.widget.user_stats",
		AreaCode:      "admin.dashboard.main",
		Configuration: map[string]any{"metric": "total"},
	},
	{
		DefinitionID:  "admin.widget.recent_activity",
		AreaCode:      "admin.dashboard.sidebar",
		Configuration: map[string]any{"limit": 10},
	},
	{
		DefinitionID:  "admin.widget.quick_actions",
		AreaCode:      "admin.dashboard.footer",
		Configuration: map[string]any{},
	},
}

// DefaultAreaDefinitions returns copies of built-in area definitions.
func DefaultAreaDefinitions() []WidgetAreaDefinition {
	out := make([]WidgetAreaDefinition, len(defaultAreaDefinitions))
	copy(out, defaultAreaDefinitions)
	return out
}

// DefaultWidgetDefinitions returns copies of built-in widget definitions.
func DefaultWidgetDefinitions() []WidgetDefinition {
	out := make([]WidgetDefinition, len(defaultWidgetDefinitions))
	for i, def := range defaultWidgetDefinitions {
		out[i] = def
	}
	return out
}

// DefaultSeedWidgets returns starter widget configurations.
func DefaultSeedWidgets() []AddWidgetRequest {
	out := make([]AddWidgetRequest, len(defaultSeedConfigs))
	for i, cfg := range defaultSeedConfigs {
		copyCfg := cfg
		if cfg.StartAt != nil {
			start := *cfg.StartAt
			copyCfg.StartAt = &start
		}
		if cfg.EndAt != nil {
			end := *cfg.EndAt
			copyCfg.EndAt = &end
		}
		out[i] = copyCfg
	}
	return out
}

// DefaultWidgetVisibility returns a permissive visibility configuration for seeds.
func DefaultWidgetVisibility() WidgetVisibility {
	now := time.Now().UTC()
	return WidgetVisibility{
		StartAt: &now,
	}
}
