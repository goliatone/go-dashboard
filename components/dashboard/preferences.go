package dashboard

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryPreferenceStore provides a concurrency-safe default store for MVP.
type InMemoryPreferenceStore struct {
	mu   sync.RWMutex
	data map[string]LayoutOverrides
}

// NewInMemoryPreferenceStore creates an empty preference store.
func NewInMemoryPreferenceStore() *InMemoryPreferenceStore {
	return &InMemoryPreferenceStore{
		data: make(map[string]LayoutOverrides),
	}
}

// LayoutOverrides returns stored overrides or defaults.
func (s *InMemoryPreferenceStore) LayoutOverrides(_ context.Context, viewer ViewerContext) (LayoutOverrides, error) {
	if viewer.UserID == "" {
		return LayoutOverrides{
			Locale:        viewer.Locale,
			AreaOrder:     map[string][]string{},
			AreaRows:      map[string][]LayoutRow{},
			HiddenWidgets: map[string]bool{},
		}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if overrides, ok := s.data[s.key(viewer)]; ok {
		s.normalize(&overrides)
		if overrides.Locale == "" {
			overrides.Locale = viewer.Locale
		}
		return overrides, nil
	}
	return LayoutOverrides{
		Locale:        viewer.Locale,
		AreaOrder:     map[string][]string{},
		AreaRows:      map[string][]LayoutRow{},
		HiddenWidgets: map[string]bool{},
	}, nil
}

// SaveLayoutOverrides persists overrides for a viewer.
func (s *InMemoryPreferenceStore) SaveLayoutOverrides(_ context.Context, viewer ViewerContext, overrides LayoutOverrides) error {
	if viewer.UserID == "" {
		return fmt.Errorf("preference store requires viewer user id")
	}
	if overrides.Locale == "" {
		overrides.Locale = viewer.Locale
	}
	s.normalize(&overrides)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[s.key(viewer)] = overrides
	return nil
}

func (s *InMemoryPreferenceStore) key(viewer ViewerContext) string {
	if viewer.Locale == "" {
		return viewer.UserID
	}
	return viewer.UserID + "::" + viewer.Locale
}

func (s *InMemoryPreferenceStore) normalize(overrides *LayoutOverrides) {
	if overrides.AreaOrder == nil {
		overrides.AreaOrder = map[string][]string{}
	}
	if overrides.AreaRows == nil {
		overrides.AreaRows = map[string][]LayoutRow{}
	}
	if overrides.HiddenWidgets == nil {
		overrides.HiddenWidgets = map[string]bool{}
	}
	clampAreaRows(overrides.AreaRows)
}

func clampAreaRows(rows map[string][]LayoutRow) {
	for area, list := range rows {
		for rowIdx, row := range list {
			for slotIdx, slot := range row.Widgets {
				if slot.Width <= 0 || slot.Width > 12 {
					if slot.Width <= 0 {
						slot.Width = 12
					} else if slot.Width > 12 {
						slot.Width = 12
					}
					rows[area][rowIdx].Widgets[slotIdx] = slot
				}
			}
		}
	}
}
