package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goliatone/go-dashboard/components/dashboard"
)

type demoHarness struct {
	service    *dashboard.Service
	controller *dashboard.Controller
	app        *fiber.App
}

func TestDemoControllerPageUsesTypedContracts(t *testing.T) {
	harness := newDemoHarness(t)

	page, err := harness.controller.Page(context.Background(), dashboard.ViewerContext{
		UserID: sampleViewerID,
		Roles:  []string{"admin"},
		Locale: "es",
	})
	require.NoError(t, err)

	assert.Equal(t, "Panel de control", page.Title)
	assert.Equal(t, "Resumen administrativo", page.Description)
	require.NotNil(t, page.Theme)
	assert.Equal(t, "https://cdn.goadmin.dev/assets/logo.svg", page.Theme.AssetURL("logo"))

	userStats := requireWidgetByDefinition(t, page, "admin.widget.user_stats")
	userStatsData := requireWidgetDataMap(t, userStats)
	assert.Equal(t, "Salud de la cuenta", userStatsData["title"])

	salesChart := requireWidgetByDefinition(t, page, "admin.widget.sales_chart")
	salesChartData := requireWidgetDataMap(t, salesChart)
	assert.Equal(t, "wonderland", salesChartData["theme"])
	markup, _ := salesChartData["chart_html"].(string)
	assert.Contains(t, markup, dashboard.DefaultEChartsAssetsPath+"echarts.min.js")
}

func TestDemoHTMLRouteRendersTypedPageThemeAndCharts(t *testing.T) {
	harness := newDemoHarness(t)

	req := httptest.NewRequest("GET", "/admin/dashboard?locale=es", nil)
	resp, err := harness.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, string(body), `lang="es"`)
	assert.Contains(t, string(body), "Panel de control")
	assert.Contains(t, string(body), "https://cdn.goadmin.dev/assets/logo.svg")
	assert.Contains(t, string(body), "ops-night")
	assert.Contains(t, string(body), "--dashboard-accent: #22d3ee;")
	assert.Contains(t, string(body), dashboard.DefaultEChartsAssetsPath+"echarts.min.js")
}

func TestDemoPreferencesRoutePersistsCanonicalLayout(t *testing.T) {
	harness := newDemoHarness(t)
	viewer := dashboard.ViewerContext{UserID: sampleViewerID, Roles: []string{"admin"}, Locale: "en"}

	page, err := harness.controller.Page(context.Background(), viewer)
	require.NoError(t, err)

	mainArea := requireAreaByCode(t, page, "admin.dashboard.main")
	require.GreaterOrEqual(t, len(mainArea.Widgets), 2)

	hiddenID := mainArea.Widgets[0].ID
	swappedID := mainArea.Widgets[1].ID

	payload := canonicalPreferencesPayload(page)
	mainOrder := payload["area_order"].(map[string][]string)["admin.dashboard.main"]
	mainOrder[0], mainOrder[1] = mainOrder[1], mainOrder[0]
	payload["hidden_widget_ids"] = []string{hiddenID}

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/admin/dashboard/preferences", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := harness.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	updated, err := harness.controller.Page(context.Background(), viewer)
	require.NoError(t, err)

	updatedMain := requireAreaByCode(t, updated, "admin.dashboard.main")
	require.NotEmpty(t, updatedMain.Widgets)
	assert.Equal(t, swappedID, updatedMain.Widgets[0].ID)
	assert.False(t, hasWidgetID(updated, hiddenID))
}

func newDemoHarness(t *testing.T) demoHarness {
	t.Helper()

	ctx := context.Background()
	translator := newDemoTranslationService()
	themeSelection := demoThemeSelection()
	themeProvider := &staticThemeProvider{selection: themeSelection}
	themeSelector := func(context.Context, dashboard.ViewerContext) dashboard.ThemeSelector {
		return dashboard.ThemeSelector{Name: themeSelection.Name, Variant: themeSelection.Variant}
	}

	service, _, _, err := setupDemoDashboard(ctx, translator, themeProvider, themeSelector)
	require.NoError(t, err)

	controller := newDemoController(service, translator, newSampleRenderer())
	server, err := newDemoServer(controller, newDemoCommandExecutor(service), dashboard.NewBroadcastHook())
	require.NoError(t, err)

	return demoHarness{
		service:    service,
		controller: controller,
		app:        server.WrappedRouter(),
	}
}

func requireAreaByCode(t *testing.T, page dashboard.Page, code string) dashboard.PageArea {
	t.Helper()
	for _, area := range page.Areas {
		if area.Code == code {
			return area
		}
	}
	t.Fatalf("area %s not found", code)
	return dashboard.PageArea{}
}

func requireWidgetByDefinition(t *testing.T, page dashboard.Page, definition string) dashboard.WidgetFrame {
	t.Helper()
	for _, area := range page.Areas {
		for _, widget := range area.Widgets {
			if widget.Definition == definition {
				return widget
			}
		}
	}
	t.Fatalf("widget definition %s not found", definition)
	return dashboard.WidgetFrame{}
}

func requireWidgetByID(t *testing.T, page dashboard.Page, id string) dashboard.WidgetFrame {
	t.Helper()
	for _, area := range page.Areas {
		for _, widget := range area.Widgets {
			if widget.ID == id {
				return widget
			}
		}
	}
	t.Fatalf("widget %s not found", id)
	return dashboard.WidgetFrame{}
}

func hasWidgetID(page dashboard.Page, id string) bool {
	for _, area := range page.Areas {
		for _, widget := range area.Widgets {
			if widget.ID == id {
				return true
			}
		}
	}
	return false
}

func requireWidgetDataMap(t *testing.T, widget dashboard.WidgetFrame) map[string]any {
	t.Helper()
	data := toDataMap(widget.Data)
	require.NotNil(t, data, "expected widget %s data to serialize into an object", widget.Definition)
	return data
}

func canonicalPreferencesPayload(page dashboard.Page) map[string]any {
	areaOrder := map[string][]string{}
	layoutRows := map[string][]map[string]any{}
	for _, area := range page.Areas {
		if area.Code == "" {
			continue
		}
		ids := make([]string, 0, len(area.Widgets))
		rows := make([]map[string]any, 0, len(area.Widgets))
		for _, widget := range area.Widgets {
			ids = append(ids, widget.ID)
			rows = append(rows, map[string]any{
				"widgets": []map[string]any{{
					"id":    widget.ID,
					"width": widgetSpan(widget),
				}},
			})
		}
		areaOrder[area.Code] = ids
		layoutRows[area.Code] = rows
	}
	return map[string]any{
		"area_order":        areaOrder,
		"hidden_widget_ids": []string{},
		"layout_rows":       layoutRows,
	}
}

func widgetSpan(widget dashboard.WidgetFrame) int {
	if widget.Span > 0 {
		return widget.Span
	}
	if widget.Meta.Layout != nil && widget.Meta.Layout.Width > 0 {
		return widget.Meta.Layout.Width
	}
	return 12
}
