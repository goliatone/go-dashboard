package dashboard

// DashboardSemanticUsage describes the framework-owned presentation surfaces
// present in one rendered page. It lets diagnostics distinguish a dashboard
// grid from an application shell instead of treating every supported token as
// consumed everywhere.
type DashboardSemanticUsage struct {
	Shell      bool
	Dashboard  bool
	Header     bool
	Areas      bool
	Widgets    bool
	EmptyState bool
	ErrorState bool
}

// DashboardSemanticStyles identifies the declarations the stock dashboard
// template may emit for a concrete page. Each declaration remains absent until
// its component or portable fallback resolves successfully.
type DashboardSemanticStyles struct {
	Surface        bool
	Text           bool
	BodyFont       bool
	HeadingFont    bool
	Motion         bool
	CardBackground bool
	CardBorder     bool
	CardRadius     bool
	CardShadow     bool
	MetricLabel    bool
	MetricValue    bool
	TrendPositive  bool
	TrendNegative  bool
	Focus          bool
	Gap            bool
	EmptyState     bool
	ErrorState     bool
}

// Active reports whether the stock dashboard template has any semantic
// declaration to emit.
func (styles DashboardSemanticStyles) Active() bool {
	return styles.Surface ||
		styles.Text ||
		styles.BodyFont ||
		styles.HeadingFont ||
		styles.Motion ||
		styles.CardBackground ||
		styles.CardBorder ||
		styles.CardRadius ||
		styles.CardShadow ||
		styles.MetricLabel ||
		styles.MetricValue ||
		styles.TrendPositive ||
		styles.TrendNegative ||
		styles.Focus ||
		styles.Gap
}

func (styles DashboardSemanticStyles) payload() map[string]bool {
	values := map[string]bool{}
	add := func(key string, enabled bool) {
		if enabled {
			values[key] = true
		}
	}
	add("surface", styles.Surface)
	add("text", styles.Text)
	add("body_font", styles.BodyFont)
	add("heading_font", styles.HeadingFont)
	add("motion", styles.Motion)
	add("card_background", styles.CardBackground)
	add("card_border", styles.CardBorder)
	add("card_radius", styles.CardRadius)
	add("card_shadow", styles.CardShadow)
	add("metric_label", styles.MetricLabel)
	add("metric_value", styles.MetricValue)
	add("trend_positive", styles.TrendPositive)
	add("trend_negative", styles.TrendNegative)
	add("focus", styles.Focus)
	add("gap", styles.Gap)
	add("empty_state", styles.EmptyState)
	add("error_state", styles.ErrorState)
	return values
}

// DashboardSemanticPlan is the render-aware semantic contract for one page.
type DashboardSemanticPlan struct {
	Styles      DashboardSemanticStyles
	Diagnostics []TokenDiagnostic
}

// SemanticDashboardPlan resolves only consumers present on the supplied page.
// Shell consumers are diagnosed separately from stock dashboard-grid styles.
func (theme *ThemeSelection) SemanticDashboardPlan(usage DashboardSemanticUsage) DashboardSemanticPlan {
	projection := theme.SemanticProjection()
	if theme == nil {
		return DashboardSemanticPlan{Diagnostics: projection.Diagnostics}
	}

	resolver := newDashboardSemanticResolver(theme)
	var styles DashboardSemanticStyles
	switch {
	case usage.Shell:
		resolver.consumeShell()
	case usage.Dashboard:
		styles = resolver.resolveDashboardStyles(usage)
	}
	styles.applyStateUsage(usage)

	return DashboardSemanticPlan{
		Styles:      styles,
		Diagnostics: projection.ConsumerDiagnostics("go-dashboard.template", resolver.consumed...),
	}
}

type dashboardSemanticResolver struct {
	theme    *ThemeSelection
	consumed []string
	seen     map[string]bool
}

func newDashboardSemanticResolver(theme *ThemeSelection) *dashboardSemanticResolver {
	return &dashboardSemanticResolver{
		theme:    theme,
		consumed: make([]string, 0, len(dashboardChromeFallbacks)+2),
		seen:     map[string]bool{},
	}
}

func (resolver *dashboardSemanticResolver) resolve(component string, portable ...string) bool {
	resolution := resolver.theme.ResolveSemanticToken(component, portable, "")
	if resolution.Token == "" {
		return false
	}
	appendUniqueToken(&resolver.consumed, resolver.seen, resolution.Token)
	return true
}

func (resolver *dashboardSemanticResolver) consumeShell() {
	resolver.resolve("dashboard.surface", "color.surface.canvas")
	resolver.resolve("", "color.text.primary")
	resolver.resolve("dashboard.card.background", "color.surface.raised")
	resolver.resolve("", "color.surface.default")
	resolver.resolve("dashboard.card.border", "color.border.default")
	resolver.resolve("dashboard.metric.label", "color.text.secondary")
	resolver.resolve("", "color.action.accent")
	resolver.resolve("", "color.focus.ring")
	resolver.resolve("dashboard.card.radius", "radius.surface")
	resolver.resolve("dashboard.card.shadow", "shadow.surface")
}

func (resolver *dashboardSemanticResolver) resolveDashboardStyles(
	usage DashboardSemanticUsage,
) DashboardSemanticStyles {
	styles := DashboardSemanticStyles{
		Surface:  resolver.resolve("dashboard.surface", "color.surface.canvas"),
		Text:     resolver.resolve("", "color.text.primary"),
		BodyFont: resolver.resolve("", "font.family.body"),
	}
	if usage.Header {
		styles.HeadingFont = resolver.resolve("", "font.family.heading", "font.family.body")
	}
	styles.Motion = resolver.resolve("", "motion.duration.fast")
	if styles.Motion {
		resolver.resolve("", "motion.easing.standard")
	}
	if usage.Areas {
		styles.Gap = resolver.resolve("", "space.stack")
	}
	if usage.Widgets || usage.EmptyState {
		styles.MetricLabel = resolver.resolve("dashboard.metric.label", "color.text.secondary")
	}
	if usage.Widgets {
		resolver.resolveWidgetStyles(&styles)
	}
	return styles
}

func (resolver *dashboardSemanticResolver) resolveWidgetStyles(styles *DashboardSemanticStyles) {
	styles.CardBackground = resolver.resolve(
		"dashboard.card.background",
		"color.surface.raised",
		"color.surface.default",
	)
	styles.CardBorder = resolver.resolve("dashboard.card.border", "color.border.default")
	styles.CardRadius = resolver.resolve("dashboard.card.radius", "radius.surface")
	styles.CardShadow = resolver.resolve("dashboard.card.shadow", "shadow.surface")
	styles.MetricValue = resolver.resolve("dashboard.metric.value", "color.text.primary")
	styles.TrendPositive = resolver.resolve("dashboard.metric.trend-positive", "color.status.success")
	styles.TrendNegative = resolver.resolve("dashboard.metric.trend-negative", "color.status.danger")
	styles.Focus = resolver.resolve("", "color.focus.ring")
}

func (styles *DashboardSemanticStyles) applyStateUsage(usage DashboardSemanticUsage) {
	if !styles.Active() {
		return
	}
	styles.EmptyState = usage.EmptyState && styles.MetricLabel
	styles.ErrorState = usage.ErrorState && styles.TrendNegative
}

func dashboardSemanticUsage(page Page) DashboardSemanticUsage {
	usage := DashboardSemanticUsage{Shell: page.Shell != nil}
	if usage.Shell {
		return usage
	}
	usage.Dashboard = true
	usage.Header = true
	usage.Areas = len(page.Areas) > 0
	for _, area := range page.Areas {
		if len(area.Widgets) == 0 {
			usage.EmptyState = true
			continue
		}
		usage.Widgets = true
		for _, widget := range area.Widgets {
			switch widget.State {
			case WidgetStateEmpty:
				usage.EmptyState = true
			case WidgetStateError:
				usage.ErrorState = true
			}
		}
	}
	return usage
}
