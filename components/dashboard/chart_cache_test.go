package dashboard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChartCacheStoresEntry(t *testing.T) {
	cache := NewChartCache(10 * time.Millisecond)
	calls := 0
	render := func() (string, error) {
		calls++
		return "html", nil
	}

	val1, err := cache.GetOrRender("key", render)
	require.NoError(t, err)
	val2, err := cache.GetOrRender("key", render)
	require.NoError(t, err)

	assert.Equal(t, "html", val1)
	assert.Equal(t, val1, val2)
	assert.Equal(t, 1, calls)
}

func TestChartCacheExpires(t *testing.T) {
	cache := NewChartCache(2 * time.Millisecond)
	calls := 0
	render := func() (string, error) {
		calls++
		return "fresh", nil
	}

	_, err := cache.GetOrRender("key", render)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	_, err = cache.GetOrRender("key", render)
	require.NoError(t, err)

	assert.Equal(t, 2, calls)
}
