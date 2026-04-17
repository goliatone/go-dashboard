package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

type stubLayoutResolver struct {
	layout Layout
	err    error
}

func (s *stubLayoutResolver) ConfigureLayout(ctx context.Context, viewer ViewerContext) (Layout, error) {
	return s.layout, s.err
}

type stubRenderer struct {
	lastTemplate string
	lastPage     Page
	err          error
}

func (r *stubRenderer) RenderPage(name string, page Page, out ...io.Writer) (string, error) {
	r.lastTemplate = name
	r.lastPage = page
	if len(out) > 0 && out[0] != nil {
		out[0].Write([]byte("<html></html>"))
	}
	return "<html></html>", r.err
}

func TestControllerRenderTemplate(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats", Metadata: map[string]any{"data": WidgetData{"value": 42}}},
				},
			},
		},
	}
	renderer := &stubRenderer{}
	controller := NewController(ControllerOptions{
		Service:  service,
		Renderer: renderer,
		Template: "dashboard.html",
	})

	var buf bytes.Buffer
	if err := controller.RenderTemplate(context.Background(), ViewerContext{UserID: "user"}, &buf); err != nil {
		t.Fatalf("RenderTemplate returned error: %v", err)
	}
	if renderer.lastTemplate != "dashboard.html" {
		t.Fatalf("expected dashboard template to render, got %s", renderer.lastTemplate)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected rendered output")
	}
}

func TestControllerPageReturnsTypedPage(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main", Metadata: map[string]any{"data": WidgetData{"value": 42}}},
				},
			},
		},
	}
	controller := NewController(ControllerOptions{Service: service})
	page, err := controller.Page(context.Background(), ViewerContext{Locale: "es"})
	if err != nil {
		t.Fatalf("Page returned error: %v", err)
	}
	if page.Locale != "es" {
		t.Fatalf("expected page locale propagated, got %q", page.Locale)
	}
	if len(page.Areas) != 3 || page.Areas[0].Slot != "main" {
		t.Fatalf("expected canonical ordered page areas, got %+v", page.Areas)
	}
	if len(page.Areas[0].Widgets) != 1 || page.Areas[0].Widgets[0].Definition != "admin.widget.user_stats" {
		t.Fatalf("expected typed widget frame on page, got %+v", page.Areas[0].Widgets)
	}
}

func TestLayoutPayloadUsesSnakeCaseKeys(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main", Configuration: map[string]any{"metric": "total"}, Metadata: map[string]any{"data": WidgetData{"value": 42}}},
				},
			},
		},
	}
	controller := NewController(ControllerOptions{Service: service})

	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if locale, ok := payload["locale"].(string); !ok || locale != "" {
		t.Fatalf("expected empty locale for anonymous viewer, got %#v", payload["locale"])
	}
	areas, ok := payload["areas"].(map[string]any)
	if !ok {
		t.Fatalf("expected areas map, got %T", payload["areas"])
	}
	mainArea, ok := areas["main"].(map[string]any)
	if !ok {
		t.Fatalf("expected main area map, got %T", areas["main"])
	}
	if _, ok := mainArea["code"]; !ok {
		t.Fatalf("expected snake_case code key")
	}
	widgets, ok := mainArea["widgets"].([]map[string]any)
	if !ok {
		t.Fatalf("expected widgets slice, got %T", mainArea["widgets"])
	}
	if len(widgets) == 0 {
		t.Fatalf("expected at least one widget")
	}
	if _, ok := widgets[0]["area_code"]; !ok {
		t.Fatalf("expected area_code key on widget payload")
	}
	if _, ok := widgets[0]["data"].(map[string]any); !ok {
		t.Fatalf("expected widget data normalized to map[string]any, got %T", widgets[0]["data"])
	}

	ordered, ok := payload["ordered_areas"].([]map[string]any)
	if !ok {
		raw, rok := payload["ordered_areas"].([]any)
		if !rok {
			t.Fatalf("expected ordered_areas slice, got %T", payload["ordered_areas"])
		}
		ordered = make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if m, ok := item.(map[string]any); ok {
				ordered = append(ordered, m)
			}
		}
	}
	if len(ordered) != 3 {
		t.Fatalf("expected 3 ordered areas, got %d", len(ordered))
	}
	if ordered[0]["slot"] != "main" || ordered[1]["slot"] != "sidebar" || ordered[2]["slot"] != "footer" {
		t.Fatalf("unexpected ordered area slots: %+v", ordered)
	}
}

func TestLayoutPayloadIncludesLocale(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{},
		},
	}
	controller := NewController(ControllerOptions{Service: service})
	viewer := ViewerContext{Locale: "es"}
	payload, err := controller.LayoutPayload(context.Background(), viewer)
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if payload["locale"] != "es" {
		t.Fatalf("expected locale propagated to payload, got %#v", payload["locale"])
	}
}

func TestLayoutPayloadIncludesTheme(t *testing.T) {
	theme := &ThemeSelection{
		Name:    "demo",
		Variant: "dark",
		Tokens: map[string]string{
			"--color-primary": "#fff",
		},
		Assets: ThemeAssets{
			Values: map[string]string{
				"logo": "img/logo.svg",
			},
			Prefix: "https://cdn.example.com",
		},
	}
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats"},
				},
			},
			Theme: theme,
		},
	}
	controller := NewController(ControllerOptions{Service: service})
	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	rawTheme, ok := payload["theme"].(map[string]any)
	if !ok {
		t.Fatalf("expected theme payload included in response")
	}
	if rawTheme["variant"] != "dark" {
		t.Fatalf("expected theme variant propagated, got %#v", rawTheme["variant"])
	}
	assets, ok := rawTheme["assets"].(map[string]string)
	if !ok {
		t.Fatalf("expected resolved assets map, got %T", rawTheme["assets"])
	}
	if assets["logo"] != "https://cdn.example.com/img/logo.svg" {
		t.Fatalf("expected asset URL resolved with prefix, got %s", assets["logo"])
	}
	widgets := payload["areas"].(map[string]any)["main"].(map[string]any)["widgets"].([]map[string]any)
	if widgets[0]["theme"] == nil {
		t.Fatalf("expected widget payload to include theme reference")
	}
}

func TestTemplatePathForDefinition(t *testing.T) {
	tests := map[string]string{
		"admin.widget.user_stats":       "widgets/user_stats.html",
		"admin.widget.analytics_funnel": "widgets/analytics_funnel.html",
		"user_stats":                    "widgets/user_stats.html",
		"":                              "widgets/unknown.html",
	}
	for def, expect := range tests {
		if got := templatePathFor(def); got != expect {
			t.Fatalf("templatePathFor(%q) = %q, want %q", def, got, expect)
		}
	}
}

func TestControllerSupportsCustomAreas(t *testing.T) {
	layout := Layout{
		Areas: map[string][]WidgetInstance{
			"custom.hero":   {{ID: "hero-1"}},
			"custom.bottom": {{ID: "bottom-1"}},
		},
	}
	service := &stubLayoutResolver{layout: layout}
	controller := NewController(ControllerOptions{
		Service: service,
		Areas: []AreaSlot{
			{Slot: "hero", Code: "custom.hero"},
			{Slot: "bottom", Code: "custom.bottom"},
		},
	})

	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	areas, ok := payload["areas"].(map[string]any)
	if !ok {
		t.Fatalf("expected areas map, got %T", payload["areas"])
	}
	if _, ok := areas["hero"]; !ok {
		t.Fatalf("expected hero slot in areas map")
	}
	if _, ok := areas["bottom"]; !ok {
		t.Fatalf("expected bottom slot in areas map")
	}
	ordered := payload["ordered_areas"].([]map[string]any)
	if ordered[0]["slot"] != "hero" || ordered[1]["slot"] != "bottom" {
		t.Fatalf("unexpected ordered slot sequence: %+v", ordered)
	}
}

func TestControllerPayloadDecoratorAppliesToLegacyLayoutPayloadOnly(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats"},
				},
			},
		},
	}
	renderer := &stubRenderer{}
	controller := NewController(ControllerOptions{
		Service:  service,
		Renderer: renderer,
		PayloadDecorator: func(_ context.Context, viewer ViewerContext, payload map[string]any) (map[string]any, error) {
			payload["viewer_id"] = viewer.UserID
			payload["decorated"] = true
			return payload, nil
		},
	})

	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if payload["viewer_id"] != "user-1" || payload["decorated"] != true {
		t.Fatalf("expected decorated layout payload, got %+v", payload)
	}

	var buf bytes.Buffer
	if err := controller.RenderTemplate(context.Background(), ViewerContext{UserID: "user-1"}, &buf); err != nil {
		t.Fatalf("RenderTemplate returned error: %v", err)
	}
	if renderer.lastPage.Title != "Dashboard" {
		t.Fatalf("expected HTML rendering to receive the canonical typed page, got %+v", renderer.lastPage)
	}
}

func TestControllerPageDecoratorAppliesBeforeHTMLAndJSONAdapters(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats"},
				},
			},
		},
	}
	renderer := &stubRenderer{}
	controller := NewController(ControllerOptions{
		Service:  service,
		Renderer: renderer,
		PageDecorator: func(_ context.Context, viewer ViewerContext, page Page) (Page, error) {
			page.Title = "Decorated " + viewer.UserID
			page.Areas[0].Title = "Primary"
			return page, nil
		},
	})

	page, err := controller.Page(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Page returned error: %v", err)
	}
	if page.Title != "Decorated user-1" || page.Areas[0].Title != "Primary" {
		t.Fatalf("expected page decorator applied to typed page, got %+v", page)
	}

	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if payload["title"] != "Decorated user-1" {
		t.Fatalf("expected layout payload to derive from decorated page, got %+v", payload)
	}
	main := payload["areas"].(map[string]any)["main"].(map[string]any)
	if main["title"] != "Primary" {
		t.Fatalf("expected area title preserved through payload adapter, got %+v", main)
	}

	var buf bytes.Buffer
	if err := controller.RenderTemplate(context.Background(), ViewerContext{UserID: "user-1"}, &buf); err != nil {
		t.Fatalf("RenderTemplate returned error: %v", err)
	}
	if renderer.lastPage.Title != "Decorated user-1" {
		t.Fatalf("expected HTML path to render the decorated typed page, got %+v", renderer.lastPage)
	}
}

func TestControllerLegacyPayloadAdapterDerivesFromTypedPage(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{
						ID:           "w1",
						DefinitionID: "admin.widget.user_stats",
						AreaCode:     "admin.dashboard.main",
						Configuration: map[string]any{
							"metric": "signups",
						},
						Metadata: map[string]any{
							"data": WidgetData{"value": 7},
							"layout": map[string]any{
								"row":     1,
								"column":  2,
								"width":   6,
								"columns": 6,
							},
							"source": "fixture",
						},
					},
				},
			},
			Theme: &ThemeSelection{Name: "admin", Variant: "dark"},
		},
	}
	controller := NewController(ControllerOptions{Service: service})
	viewer := ViewerContext{Locale: "fr"}

	page, err := controller.pageForViewer(context.Background(), viewer)
	if err != nil {
		t.Fatalf("pageForViewer returned error: %v", err)
	}
	payload, err := controller.LayoutPayload(context.Background(), viewer)
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if !reflect.DeepEqual(page.LegacyPayload(), payload) {
		t.Fatalf("expected layout payload adapter to derive from typed page")
	}
}

func TestControllerRenderPageUsesTypedPageContract(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main"},
				},
			},
		},
	}
	renderer := &stubRenderer{}
	controller := NewController(ControllerOptions{
		Service:  service,
		Renderer: renderer,
	})

	var buf bytes.Buffer
	if err := controller.RenderPage(context.Background(), ViewerContext{UserID: "user-1"}, &buf); err != nil {
		t.Fatalf("RenderPage returned error: %v", err)
	}
	page := renderer.lastPage
	if len(page.Areas) == 0 || page.Areas[0].Widgets[0].ID != "w1" {
		t.Fatalf("expected typed page passed to renderer, got %+v", page)
	}
}

func TestControllerPageAcceptsPreSerializedViewModelMetadata(t *testing.T) {
	controller := NewController(ControllerOptions{
		Service: &stubLayoutResolver{layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{
						ID:           "chart-1",
						DefinitionID: "admin.widget.bar_chart",
						Metadata: map[string]any{
							widgetViewModelMetadataKey: map[string]any{
								"chart_html": "<div>ok</div><script>render()</script>",
								"theme":      "wonderland",
								"js_assets":  []any{"/dashboard/assets/echarts/echarts.min.js"},
							},
						},
					},
				},
			},
		}},
	})

	page, err := controller.Page(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("Page returned error: %v", err)
	}

	main, ok := page.Area("main")
	if !ok || len(main.Widgets) != 1 {
		t.Fatalf("expected main area widget, got %+v", page.Areas)
	}
	data, ok := main.Widgets[0].Data.(map[string]any)
	if !ok {
		t.Fatalf("expected pre-serialized metadata to normalize into object data, got %T", main.Widgets[0].Data)
	}
	if data["theme"] != "wonderland" || data["chart_html"] != "<div>ok</div><script>render()</script>" {
		t.Fatalf("expected serialized metadata preserved, got %+v", data)
	}
	if _, ok := data["js_assets"]; ok {
		t.Fatalf("expected widget page assets promoted out of widget data, got %+v", data)
	}
	if page.Assets == nil || len(page.Assets.JS) != 1 || page.Assets.JS[0] != "/dashboard/assets/echarts/echarts.min.js" {
		t.Fatalf("expected page assets aggregated from widget data, got %+v", page.Assets)
	}
}

func TestControllerPageAggregatesAndDeduplicatesWidgetAssets(t *testing.T) {
	controller := NewController(ControllerOptions{
		Service: &stubLayoutResolver{layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{
						ID:           "chart-1",
						DefinitionID: "admin.widget.bar_chart",
						Metadata: map[string]any{
							widgetViewModelMetadataKey: map[string]any{
								"chart_html": "<div>a</div><script>a()</script>",
								"js_assets":  []any{"/assets/echarts.min.js", "/assets/theme.js"},
								"css_assets": []any{"/assets/chart.css"},
							},
						},
					},
					{
						ID:           "chart-2",
						DefinitionID: "admin.widget.line_chart",
						Metadata: map[string]any{
							widgetViewModelMetadataKey: map[string]any{
								"chart_html": "<div>b</div><script>b()</script>",
								"js_assets":  []any{"/assets/echarts.min.js"},
							},
						},
					},
				},
			},
		}},
	})

	page, err := controller.Page(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("Page returned error: %v", err)
	}
	if page.Assets == nil {
		t.Fatalf("expected page assets to be aggregated")
	}
	if !reflect.DeepEqual(page.Assets.JS, []string{"/assets/echarts.min.js", "/assets/theme.js"}) {
		t.Fatalf("expected js assets deduped in order, got %+v", page.Assets.JS)
	}
	if !reflect.DeepEqual(page.Assets.CSS, []string{"/assets/chart.css"}) {
		t.Fatalf("expected css assets preserved, got %+v", page.Assets.CSS)
	}
	main, ok := page.Area("main")
	if !ok || len(main.Widgets) != 2 {
		t.Fatalf("expected main widgets, got %+v", page.Areas)
	}
	for _, widget := range main.Widgets {
		data, ok := widget.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected aggregated chart widgets normalized to maps, got %T", widget.Data)
		}
		if _, ok := data["js_assets"]; ok {
			t.Fatalf("expected widget js assets stripped after promotion, got %+v", data)
		}
		if _, ok := data["css_assets"]; ok {
			t.Fatalf("expected widget css assets stripped after promotion, got %+v", data)
		}
	}
	payload := page.LegacyPayload()
	assets, ok := payload["assets"].(map[string]any)
	if !ok {
		t.Fatalf("expected legacy payload to expose page assets, got %+v", payload["assets"])
	}
	if !reflect.DeepEqual(assets["js"], []string{"/assets/echarts.min.js", "/assets/theme.js"}) {
		t.Fatalf("expected legacy payload js assets preserved, got %+v", assets["js"])
	}
}

func TestControllerHTMLAndJSONDeriveFromSameTypedPageSource(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main", Metadata: map[string]any{"data": WidgetData{"value": 7}}},
				},
			},
		},
	}
	renderer := &stubRenderer{}
	controller := NewController(ControllerOptions{
		Service:  service,
		Renderer: renderer,
		PageDecorator: func(_ context.Context, _ ViewerContext, page Page) (Page, error) {
			page.Description = "Shared"
			return page, nil
		},
	})

	page, err := controller.Page(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Page returned error: %v", err)
	}
	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	var buf bytes.Buffer
	if err := controller.RenderTemplate(context.Background(), ViewerContext{UserID: "user-1"}, &buf); err != nil {
		t.Fatalf("RenderTemplate returned error: %v", err)
	}
	if !reflect.DeepEqual(page.LegacyPayload(), payload) {
		t.Fatalf("expected JSON payload to derive from same typed page source")
	}
	if !reflect.DeepEqual(page, renderer.lastPage) {
		t.Fatalf("expected HTML path to render the same typed page source")
	}
}

func TestPageJSONUsesCanonicalTypedContract(t *testing.T) {
	page := Page{
		Title:       "Dashboard",
		Description: "Admin overview",
		Locale:      "es",
		Areas: []PageArea{
			{
				Slot:  "hero",
				Code:  "custom.hero",
				Order: 1,
				Widgets: []WidgetFrame{
					{
						ID:         "hero-1",
						Definition: "admin.widget.hero",
						Name:       "Hero KPI",
						Template:   "widgets/hero.html",
						Area:       "custom.hero",
						Span:       8,
						Config:     map[string]any{"metric": "sales"},
						Data:       map[string]any{"value": 42},
						Meta: WidgetMeta{
							Order: 1,
							Layout: &WidgetLayout{
								Row:     0,
								Column:  0,
								Width:   8,
								Columns: 8,
							},
							Extensions: map[string]json.RawMessage{
								"source": json.RawMessage(`"fixture"`),
							},
						},
					},
				},
			},
			{
				Slot:  "footer",
				Code:  "custom.footer",
				Order: 2,
			},
		},
		Theme: &ThemeSelection{
			Name:    "admin",
			Variant: "dark",
			Tokens: map[string]string{
				"color-primary": "#fff",
			},
			Assets: ThemeAssets{
				Values: map[string]string{
					"logo": "img/logo.svg",
				},
				Prefix: "https://cdn.example.com",
			},
			ChartTheme: "wonderland",
		},
	}

	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if _, ok := payload["ordered_areas"]; ok {
		t.Fatalf("expected canonical page json to preserve ordering via areas only")
	}
	areas, ok := payload["areas"].([]any)
	if !ok {
		t.Fatalf("expected areas array, got %T", payload["areas"])
	}
	if len(areas) != 2 {
		t.Fatalf("expected 2 ordered areas, got %d", len(areas))
	}
	firstArea := areas[0].(map[string]any)
	secondArea := areas[1].(map[string]any)
	if firstArea["slot"] != "hero" || secondArea["slot"] != "footer" {
		t.Fatalf("expected areas slice ordering preserved, got %+v", areas)
	}
	theme := payload["theme"].(map[string]any)
	if theme["variant"] != "dark" {
		t.Fatalf("expected typed page theme to reuse ThemeSelection shape, got %#v", theme["variant"])
	}
	if theme["chart_theme"] != "wonderland" {
		t.Fatalf("expected chart theme to serialize from ThemeSelection, got %#v", theme["chart_theme"])
	}
	widgets := firstArea["widgets"].([]any)
	widget := widgets[0].(map[string]any)
	meta := widget["meta"].(map[string]any)
	if meta["order"].(float64) != 1 {
		t.Fatalf("expected widget meta order in canonical json, got %#v", meta["order"])
	}
	extensions := meta["extensions"].(map[string]any)
	if extensions["source"] != "fixture" {
		t.Fatalf("expected widget extensions encoded through typed meta, got %#v", extensions["source"])
	}
}
