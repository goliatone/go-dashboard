package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

const defaultChartHeight = "360px"

var (
	chartVarPattern = regexp.MustCompile(`let\s+(goecharts_[A-Za-z0-9]+)\s*=`)
	chartDivPattern = regexp.MustCompile(`id="([\w-]+)"`)
)

var sharedChartCache = NewChartCache(5 * time.Minute)

type chartRenderContext struct {
	Viewer ViewerContext
	Theme  string
}

// ThemeResolver selects a chart theme per viewer.
type ThemeResolver func(ViewerContext) string

// EChartsProvider renders server-side chart HTML for the given chart type.
type EChartsProvider struct {
	chartType     string
	cache         RenderCache
	theme         string
	themeResolver ThemeResolver
	assetsHost    string
	showTitle     bool
	customTheme   bool
}

// EChartsProviderOption customizes provider behavior.
type EChartsProviderOption func(*EChartsProvider)

// WithChartCache injects a render cache.
func WithChartCache(cache RenderCache) EChartsProviderOption {
	return func(p *EChartsProvider) {
		p.cache = cache
	}
}

// WithChartTheme sets a static theme (defaults to Westeros).
func WithChartTheme(theme string) EChartsProviderOption {
	return func(p *EChartsProvider) {
		p.theme = theme
		p.customTheme = true
	}
}

// WithChartThemeResolver resolves themes dynamically per viewer.
func WithChartThemeResolver(resolver ThemeResolver) EChartsProviderOption {
	return func(p *EChartsProvider) {
		p.themeResolver = resolver
	}
}

// WithChartAssetsHost rewrites the assets host so ECharts JS loads from a CDN.
func WithChartAssetsHost(host string) EChartsProviderOption {
	return func(p *EChartsProvider) {
		p.assetsHost = ensureTrailingSlash(host)
	}
}

// NewEChartsProvider builds a provider for a specific chart type.
func NewEChartsProvider(chartType string, opts ...EChartsProviderOption) *EChartsProvider {
	p := &EChartsProvider{
		chartType:  strings.ToLower(chartType),
		cache:      sharedChartCache,
		theme:      types.ThemeWesteros,
		showTitle:  false,
		assetsHost: DefaultEChartsAssetsHost(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// WithChartTitleVisibility toggles the internal go-echarts title/subtitle rendering.
func WithChartTitleVisibility(enabled bool) EChartsProviderOption {
	return func(p *EChartsProvider) {
		p.showTitle = enabled
	}
}

// Fetch converts widget configuration into go-echarts markup.
func (p *EChartsProvider) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	cfg := meta.Instance.Configuration
	if cfg == nil {
		cfg = map[string]any{}
	}

	title := stringValue(cfg["title"], "Chart")
	subtitle := stringValue(cfg["subtitle"], "")
	title = sanitizeText(title)
	subtitle = sanitizeText(subtitle)
	displayTitle := title
	displaySubtitle := subtitle

	if meta.Translator != nil {
		key := fmt.Sprintf("dashboard.widget.%s.title", meta.Instance.DefinitionID)
		if translated := translateOrFallback(ctx, meta.Translator, key, meta.Viewer.Locale, title, nil); translated != "" {
			title = translated
		}
	}
	title = sanitizeText(title)
	subtitle = sanitizeText(subtitle)

	series := parseChartSeries(cfg["series"])
	if len(series) == 0 {
		return nil, fmt.Errorf("chart series is required")
	}

	xAxis := stringSliceValue(cfg["x_axis"])
	if len(xAxis) == 0 {
		xAxis = inferredAxisLabels(series)
	}

	xAxis = p.translateAxis(ctx, meta, xAxis)
	p.translateSeries(ctx, meta, series)
	xAxis = sanitizeLabels(xAxis)
	sanitizeSeries(series)

	renderCtx := chartRenderContext{
		Viewer: meta.Viewer,
		Theme:  p.resolveTheme(meta.Viewer, meta.Theme),
	}
	if override := strings.TrimSpace(stringValue(cfg["theme"], "")); override != "" {
		renderCtx.Theme = override
	}

	showTitle := boolValue(cfg["show_chart_title"])
	if !showTitle {
		showTitle = p.showTitle
	}
	chartTitle := title
	chartSubtitle := subtitle
	if !showTitle {
		chartTitle = ""
		chartSubtitle = ""
	}

	renderFn := func() (string, error) {
		return p.render(chartTitle, chartSubtitle, xAxis, series, renderCtx)
	}

	var (
		markup string
		err    error
	)

	if p.cache != nil {
		key := fmt.Sprintf("%s:%s:%s:%s", meta.Instance.DefinitionID, meta.Instance.ID, p.chartType, configHash(cfg))
		markup, err = p.cache.GetOrRender(key, renderFn)
	} else {
		markup, err = renderFn()
	}
	if err != nil {
		return nil, err
	}

	markup = addResponsiveBehavior(markup)
	html := applySecurityDecorators(markup, nonceFrom(meta.Options))

	data := WidgetData{
		"chart_html": html,
		"chart_type": p.chartType,
		"title":      displayTitle,
		"subtitle":   displaySubtitle,
		"theme":      renderCtx.Theme,
	}

	if dynamic := boolValue(cfg["dynamic"]); dynamic {
		data["dynamic"] = true
		if refresh := stringValue(cfg["refresh_endpoint"], ""); refresh != "" {
			data["refresh_endpoint"] = refresh
		}
	}

	return data, nil
}

func (p *EChartsProvider) render(title, subtitle string, xAxis []string, series []ChartSeries, ctx chartRenderContext) (string, error) {
	options := p.globalChartOptions(title, subtitle, ctx)
	switch p.chartType {
	case "bar":
		bar := charts.NewBar()
		bar.SetGlobalOptions(options...)
		bar.SetXAxis(xAxis)
		for _, s := range series {
			bar.AddSeries(s.Name, toBarData(s.Points))
		}
		return renderChart(bar)
	case "line":
		line := charts.NewLine()
		line.SetGlobalOptions(options...)
		line.SetXAxis(xAxis)
		for _, s := range series {
			line.AddSeries(s.Name, toLineData(s.Points))
		}
		line.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: opts.Bool(true)}))
		return renderChart(line)
	case "pie":
		pie := charts.NewPie()
		pie.SetGlobalOptions(options...)
		for _, s := range series {
			pie.AddSeries(s.Name, toPieData(s.Points))
		}
		return renderChart(pie)
	case "scatter":
		scatter := charts.NewScatter()
		scatter.SetGlobalOptions(options...)
		for _, s := range series {
			scatter.AddSeries(s.Name, toScatterData(s.Points))
		}
		return renderChart(scatter)
	case "gauge":
		gauge := charts.NewGauge()
		gauge.SetGlobalOptions(options...)
		for _, s := range series {
			if len(s.Points) == 0 {
				continue
			}
			gauge.AddSeries(s.Name, []opts.GaugeData{
				{Name: s.Name, Value: s.Points[0].Value},
			})
		}
		return renderChart(gauge)
	default:
		return "", fmt.Errorf("unsupported chart type: %s", p.chartType)
	}
}

func renderChart(renderable interface{ Render(io.Writer) error }) (string, error) {
	var buf bytes.Buffer
	if err := renderable.Render(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func nonceFrom(options map[string]any) string {
	if len(options) == 0 {
		return ""
	}
	raw, ok := options[scriptNonceOptionKey]
	if !ok {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func applySecurityDecorators(markup, nonce string) string {
	if nonce == "" {
		return markup
	}
	return injectScriptNonce(markup, nonce)
}

func injectScriptNonce(markup, nonce string) string {
	if nonce == "" {
		return markup
	}
	lower := strings.ToLower(markup)
	if !strings.Contains(lower, "<script") {
		return markup
	}
	safeNonce := template.HTMLEscapeString(nonce)
	var result strings.Builder
	i := 0
	length := len(markup)
	for i < length {
		next := strings.Index(lower[i:], "<script")
		if next == -1 {
			result.WriteString(markup[i:])
			break
		}
		next += i
		tagStart := next + len("<script")
		result.WriteString(markup[i:tagStart])

		attrEnd := strings.Index(markup[tagStart:], ">")
		if attrEnd == -1 {
			result.WriteString(markup[tagStart:])
			break
		}
		attrEnd += tagStart
		attrs := markup[tagStart:attrEnd]
		if !strings.Contains(strings.ToLower(attrs), "nonce=") {
			result.WriteString(` nonce="` + safeNonce + `"`)
		}
		result.WriteString(attrs)
		result.WriteByte('>')
		i = attrEnd + 1
	}
	return result.String()
}

func addResponsiveBehavior(markup string) string {
	if markup == "" {
		return markup
	}
	varName := chartVariable(markup)
	if varName == "" {
		return markup
	}
	containerID := chartContainerID(markup)
	snippet := buildResizeSnippet(varName, containerID)
	if snippet == "" {
		return markup
	}
	closing := "</script>"
	idx := strings.LastIndex(markup, closing)
	if idx == -1 {
		return markup
	}
	return markup[:idx] + snippet + markup[idx:]
}

func chartVariable(markup string) string {
	matches := chartVarPattern.FindStringSubmatch(markup)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func chartContainerID(markup string) string {
	matches := chartDivPattern.FindStringSubmatch(markup)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func buildResizeSnippet(varName, containerID string) string {
	if varName == "" {
		return ""
	}
	container := containerID
	if container == "" {
		container = strings.TrimPrefix(varName, "goecharts_")
	}
	return fmt.Sprintf(`
    if (typeof window !== "undefined") {
      window.addEventListener("resize", function() {
        try { %s.resize(); } catch (err) {}
      });
      if (window.ResizeObserver && document.getElementById("%s")) {
        new ResizeObserver(function() {
          try { %s.resize(); } catch (err) {}
        }).observe(document.getElementById("%s"));
      }
    }
`, varName, container, varName, container)
}

func (p *EChartsProvider) globalChartOptions(title, subtitle string, ctx chartRenderContext) []charts.GlobalOpts {
	initOpts := opts.Initialization{
		Theme:  ctx.Theme,
		Width:  "100%",
		Height: defaultChartHeight,
	}
	if p.assetsHost != "" {
		initOpts.AssetsHost = p.assetsHost
	}
	optsList := []charts.GlobalOpts{
		charts.WithInitializationOpts(initOpts),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(true)}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithToolboxOpts(opts.Toolbox{Show: opts.Bool(true)}),
	}
	if title != "" || subtitle != "" {
		optsList = append([]charts.GlobalOpts{charts.WithTitleOpts(opts.Title{Title: title, Subtitle: subtitle})}, optsList...)
	}
	return optsList
}

func (p *EChartsProvider) resolveTheme(viewer ViewerContext, selection *ThemeSelection) string {
	if p.themeResolver != nil {
		if theme := p.themeResolver(viewer); theme != "" {
			return theme
		}
	}
	if p.customTheme && p.theme != "" {
		return p.theme
	}
	if derived := chartThemeFromSelection(selection); derived != "" {
		if selection != nil && selection.ChartTheme == "" {
			selection.ChartTheme = derived
		}
		return derived
	}
	if p.theme != "" {
		return p.theme
	}
	return string(types.ThemeWesteros)
}

func chartThemeFromSelection(selection *ThemeSelection) string {
	if selection == nil {
		return ""
	}
	if selection.ChartTheme != "" {
		return selection.ChartTheme
	}
	variant := strings.TrimSpace(strings.ToLower(selection.Variant))
	if variant == "" {
		return ""
	}
	if strings.Contains(variant, "dark") {
		return string(types.ThemeWonderland)
	}
	if strings.Contains(variant, "light") {
		return string(types.ThemeWesteros)
	}
	return ""
}

func toBarData(points []ChartPoint) []opts.BarData {
	data := make([]opts.BarData, len(points))
	for i, point := range points {
		data[i] = opts.BarData{
			Name:  point.Label,
			Value: point.Value,
		}
	}
	return data
}

func toLineData(points []ChartPoint) []opts.LineData {
	data := make([]opts.LineData, len(points))
	for i, point := range points {
		data[i] = opts.LineData{
			Name:  point.Label,
			Value: point.Value,
		}
	}
	return data
}

func toPieData(points []ChartPoint) []opts.PieData {
	data := make([]opts.PieData, len(points))
	for i, point := range points {
		name := point.Label
		if name == "" {
			name = fmt.Sprintf("Slice %d", i+1)
		}
		data[i] = opts.PieData{
			Name:  name,
			Value: point.Value,
		}
	}
	return data
}

func toScatterData(points []ChartPoint) []opts.ScatterData {
	data := make([]opts.ScatterData, len(points))
	for i, point := range points {
		value := []float64{float64(i + 1), point.Value}
		if len(point.Pair) >= 2 {
			value = point.Pair[:2]
		}
		data[i] = opts.ScatterData{
			Name:  point.Label,
			Value: value,
		}
	}
	return data
}

// ChartSeries represents a set of values plotted for a given legend entry.
type ChartSeries struct {
	Name   string
	Points []ChartPoint
}

// ChartPoint represents an individual value (optionally labeled).
type ChartPoint struct {
	Label string
	Value float64
	Pair  []float64
}

func parseChartSeries(v any) []ChartSeries {
	switch val := v.(type) {
	case []map[string]any:
		out := make([]ChartSeries, 0, len(val))
		for _, item := range val {
			if series := buildSeries(item); len(series.Points) > 0 {
				out = append(out, series)
			}
		}
		return out
	case []any:
		out := make([]ChartSeries, 0, len(val))
		for _, item := range val {
			seriesMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if series := buildSeries(seriesMap); len(series.Points) > 0 {
				out = append(out, series)
			}
		}
		return out
	default:
		return nil
	}
}

func buildSeries(m map[string]any) ChartSeries {
	name := stringValue(m["name"], "Series")
	points := parseChartPoints(m["data"])
	return ChartSeries{
		Name:   name,
		Points: points,
	}
}

func parseChartPoints(v any) []ChartPoint {
	switch value := v.(type) {
	case []any:
		return convertAnyPoints(value)
	case []float64:
		points := make([]ChartPoint, len(value))
		for i, val := range value {
			points[i] = ChartPoint{Value: val}
		}
		return points
	case []int:
		points := make([]ChartPoint, len(value))
		for i, val := range value {
			points[i] = ChartPoint{Value: float64(val)}
		}
		return points
	case []map[string]any:
		points := make([]ChartPoint, 0, len(value))
		for _, item := range value {
			points = append(points, ChartPoint{
				Label: stringValue(item["name"], ""),
				Value: float64Value(item["value"]),
				Pair:  pairFromMap(item),
			})
		}
		return points
	default:
		return nil
	}
}

func convertAnyPoints(items []any) []ChartPoint {
	points := make([]ChartPoint, 0, len(items))
	for _, item := range items {
		switch val := item.(type) {
		case float64:
			points = append(points, ChartPoint{Value: val})
		case float32:
			points = append(points, ChartPoint{Value: float64(val)})
		case int:
			points = append(points, ChartPoint{Value: float64(val)})
		case int64:
			points = append(points, ChartPoint{Value: float64(val)})
		case []float64:
			if len(val) >= 2 {
				points = append(points, ChartPoint{Pair: val[:2]})
			}
		case []any:
			if len(val) >= 2 {
				points = append(points, ChartPoint{
					Pair: []float64{float64Value(val[0]), float64Value(val[1])},
				})
			}
		case json.Number:
			points = append(points, ChartPoint{Value: float64Value(val)})
		case map[string]any:
			points = append(points, ChartPoint{
				Label: stringValue(val["name"], ""),
				Value: float64Value(val["value"]),
				Pair:  pairFromMap(val),
			})
		}
	}
	return points
}

func pairFromMap(m map[string]any) []float64 {
	x, xOK := m["x"]
	y, yOK := m["y"]
	if !xOK || !yOK {
		return nil
	}
	return []float64{float64Value(x), float64Value(y)}
}

func stringSliceValue(v any) []string {
	switch val := v.(type) {
	case []string:
		out := make([]string, len(val))
		for i, item := range val {
			out[i] = sanitizeText(item)
		}
		return out
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, sanitizeText(s))
			}
		}
		return out
	default:
		return nil
	}
}

func stringValue(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

func float64Value(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return f
		}
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}

func boolValue(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	case int:
		return val != 0
	case int64:
		return val != 0
	default:
		return false
	}
}

func sanitizeText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return template.HTMLEscapeString(value)
}

func sanitizeLabels(labels []string) []string {
	if len(labels) == 0 {
		return labels
	}
	out := make([]string, len(labels))
	for i, label := range labels {
		out[i] = sanitizeText(label)
	}
	return out
}

func sanitizeSeries(series []ChartSeries) {
	for i := range series {
		series[i].Name = sanitizeText(series[i].Name)
		for j := range series[i].Points {
			series[i].Points[j].Label = sanitizeText(series[i].Points[j].Label)
		}
	}
}

func (p *EChartsProvider) translateAxis(ctx context.Context, meta WidgetContext, labels []string) []string {
	if meta.Translator == nil || len(labels) == 0 {
		return labels
	}
	out := make([]string, len(labels))
	for i, label := range labels {
		out[i] = translateOrFallback(ctx, meta.Translator, label, meta.Viewer.Locale, label, nil)
	}
	return out
}

func (p *EChartsProvider) translateSeries(ctx context.Context, meta WidgetContext, series []ChartSeries) {
	if meta.Translator == nil {
		return
	}
	for i := range series {
		if series[i].Name == "" {
			continue
		}
		series[i].Name = translateOrFallback(ctx, meta.Translator, series[i].Name, meta.Viewer.Locale, series[i].Name, nil)
	}
}

func inferredAxisLabels(series []ChartSeries) []string {
	if len(series) == 0 {
		return nil
	}
	var candidate []string
	max := 0
	for _, s := range series {
		if len(s.Points) > max {
			max = len(s.Points)
			candidate = make([]string, len(s.Points))
			for i, point := range s.Points {
				if point.Label != "" {
					candidate[i] = point.Label
				} else {
					candidate[i] = fmt.Sprintf("Item %d", i+1)
				}
			}
		}
	}
	return candidate
}

func init() {
	RegisterWidgetHook(func(reg *Registry) error {
		providers := map[string]string{
			"admin.widget.bar_chart":     "bar",
			"admin.widget.line_chart":    "line",
			"admin.widget.pie_chart":     "pie",
			"admin.widget.scatter_chart": "scatter",
			"admin.widget.gauge_chart":   "gauge",
		}
		for code, chartType := range providers {
			if _, ok := reg.Provider(code); ok {
				continue
			}
			if _, ok := reg.Definition(code); !ok {
				continue
			}
			if err := reg.RegisterProvider(code, NewEChartsProvider(chartType)); err != nil {
				return err
			}
		}
		return nil
	})
}
