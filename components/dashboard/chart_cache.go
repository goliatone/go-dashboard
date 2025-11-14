package dashboard

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// RenderCache memoizes rendered chart HTML so repeated fetches are cheap.
type RenderCache interface {
	GetOrRender(key string, render func() (string, error)) (string, error)
}

// ChartCache is an in-memory TTL cache for rendered charts.
type ChartCache struct {
	ttl     time.Duration
	mu      sync.RWMutex
	entries map[string]cachedChart
}

type cachedChart struct {
	html    string
	expires time.Time
}

// NewChartCache builds a cache with the provided TTL.
func NewChartCache(ttl time.Duration) *ChartCache {
	return &ChartCache{
		ttl:     ttl,
		entries: make(map[string]cachedChart),
	}
}

// GetOrRender returns a cached entry or renders/stores a new one.
func (c *ChartCache) GetOrRender(key string, render func() (string, error)) (string, error) {
	if html, ok := c.get(key); ok {
		return html, nil
	}
	html, err := render()
	if err != nil {
		return "", err
	}
	c.set(key, html)
	return html, nil
}

func (c *ChartCache) get(key string) (string, bool) {
	if c == nil || c.ttl <= 0 {
		return "", false
	}
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		if ok {
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
		}
		return "", false
	}
	return entry.html, true
}

func (c *ChartCache) set(key, html string) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	c.entries[key] = cachedChart{
		html:    html,
		expires: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// configHash returns a deterministic hash for the widget configuration.
func configHash(cfg map[string]any) string {
	if len(cfg) == 0 {
		return "empty"
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "invalid"
	}
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])
}
