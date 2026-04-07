package dashboard

import "context"

// AreaDiagnostics captures resolved widget state for a specific area in stable
// area order.
type AreaDiagnostics struct {
	Code    string           `json:"code"`
	Widgets []WidgetInstance `json:"widgets,omitempty"`
}

// LayoutDiagnostics captures the resolved dashboard layout state independent of
// the page/render contract.
type LayoutDiagnostics struct {
	Areas []AreaDiagnostics `json:"areas,omitempty"`
}

// DashboardDiagnostics captures typed operational state for hosts that need
// discovery, troubleshooting, or debug views separate from page rendering.
type DashboardDiagnostics struct {
	Viewer      ViewerContext     `json:"viewer"`
	Preferences LayoutOverrides   `json:"preferences"`
	Theme       *ThemeSelection   `json:"theme,omitempty"`
	Layout      LayoutDiagnostics `json:"layout"`
	Page        *Page             `json:"page,omitempty"`
}

func buildLayoutDiagnostics(areaOrder []string, layout Layout) LayoutDiagnostics {
	areas := make([]AreaDiagnostics, 0, len(areaOrder))
	for _, code := range areaOrder {
		areas = append(areas, AreaDiagnostics{
			Code:    code,
			Widgets: cloneWidgetInstances(layout.Areas[code]),
		})
	}
	return LayoutDiagnostics{Areas: areas}
}

// Diagnostics returns typed operational state for the current viewer.
func (s *Service) Diagnostics(ctx context.Context, viewer ViewerContext) (DashboardDiagnostics, error) {
	layout, overrides, err := s.resolveLayoutState(ctx, viewer)
	if err != nil {
		return DashboardDiagnostics{}, err
	}
	return DashboardDiagnostics{
		Viewer:      viewer,
		Preferences: cloneLayoutOverrides(overrides),
		Theme:       cloneThemeSelection(layout.Theme),
		Layout:      buildLayoutDiagnostics(s.areaList(), layout),
	}, nil
}

type diagnosticsProvider interface {
	Diagnostics(ctx context.Context, viewer ViewerContext) (DashboardDiagnostics, error)
}

type layoutStateProvider interface {
	resolveLayoutState(ctx context.Context, viewer ViewerContext) (Layout, LayoutOverrides, error)
}

// Diagnostics returns typed operational state plus the resolved typed page when
// the controller is available.
func (c *Controller) Diagnostics(ctx context.Context, viewer ViewerContext) (DashboardDiagnostics, error) {
	if provider, ok := c.service.(layoutStateProvider); ok {
		layout, overrides, err := provider.resolveLayoutState(ctx, viewer)
		if err != nil {
			return DashboardDiagnostics{}, err
		}
		page, err := c.pageFromLayout(layout, viewer)
		if err != nil {
			return DashboardDiagnostics{}, err
		}
		page, err = c.decoratePage(ctx, viewer, page)
		if err != nil {
			return DashboardDiagnostics{}, err
		}
		return DashboardDiagnostics{
			Viewer:      viewer,
			Preferences: cloneLayoutOverrides(overrides),
			Theme:       cloneThemeSelection(layout.Theme),
			Layout:      buildLayoutDiagnostics(c.areaCodes(), layout),
			Page:        ptr(clonePage(page)),
		}, nil
	}
	if provider, ok := c.service.(diagnosticsProvider); ok {
		resolved, err := provider.Diagnostics(ctx, viewer)
		if err != nil {
			return DashboardDiagnostics{}, err
		}
		page, err := c.Page(ctx, viewer)
		if err != nil {
			return DashboardDiagnostics{}, err
		}
		resolved.Page = ptr(clonePage(page))
		if resolved.Theme == nil {
			resolved.Theme = cloneThemeSelection(page.Theme)
		}
		return resolved, nil
	}
	page, err := c.Page(ctx, viewer)
	if err != nil {
		return DashboardDiagnostics{}, err
	}
	return DashboardDiagnostics{
		Viewer: viewer,
		Theme:  cloneThemeSelection(page.Theme),
		Page:   ptr(clonePage(page)),
	}, nil
}

func (c *Controller) areaCodes() []string {
	if len(c.areas) == 0 {
		return nil
	}
	codes := make([]string, 0, len(c.areas))
	seen := make(map[string]struct{}, len(c.areas))
	for _, area := range c.areas {
		if _, ok := seen[area.Code]; ok {
			continue
		}
		seen[area.Code] = struct{}{}
		codes = append(codes, area.Code)
	}
	return codes
}

func ptr[T any](value T) *T { return &value }
