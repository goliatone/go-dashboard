package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// HTTPConfig configures the HTTP analytics client.
type HTTPConfig struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// HTTPClient talks to remote BI/observability services via REST endpoints.
type HTTPClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewHTTPClient builds a client capable of hitting live analytics APIs.
func NewHTTPClient(cfg HTTPConfig) (*HTTPClient, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("analytics: base url is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPClient{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		client:  httpClient,
	}, nil
}

// FetchFunnel implements FunnelClient by calling the remote funnel endpoint.
func (c *HTTPClient) FetchFunnel(ctx context.Context, query dashboard.FunnelQuery) (dashboard.FunnelReport, error) {
	req := funnelRequest{
		Range:   query.Range,
		Segment: query.Segment,
		Goal:    query.Goal,
	}
	var resp funnelResponse
	if err := c.do(ctx, http.MethodPost, "/funnels/query", req, &resp); err != nil {
		return dashboard.FunnelReport{}, err
	}
	return resp.toReport(), nil
}

// FetchCohorts implements CohortClient via the cohorts endpoint.
func (c *HTTPClient) FetchCohorts(ctx context.Context, query dashboard.CohortQuery) (dashboard.CohortReport, error) {
	req := cohortRequest{
		Interval: query.Interval,
		Periods:  query.Periods,
		Metric:   query.Metric,
	}
	var resp cohortResponse
	if err := c.do(ctx, http.MethodPost, "/cohorts/query", req, &resp); err != nil {
		return dashboard.CohortReport{}, err
	}
	return resp.toReport(), nil
}

// FetchAlerts implements AlertClient via the alerts endpoint.
func (c *HTTPClient) FetchAlerts(ctx context.Context, query dashboard.AlertTrendQuery) (dashboard.AlertTrendsReport, error) {
	req := alertRequest{
		LookbackDays: query.LookbackDays,
		Severities:   query.Severities,
		Service:      query.Service,
	}
	var resp alertResponse
	if err := c.do(ctx, http.MethodPost, "/alerts/query", req, &resp); err != nil {
		return dashboard.AlertTrendsReport{}, err
	}
	return resp.toReport()
}

func (c *HTTPClient) do(ctx context.Context, method, path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("analytics: encode payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("analytics: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("analytics: http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return fmt.Errorf("analytics: remote error %d: %s", resp.StatusCode, buf.String())
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("analytics: decode response: %w", err)
	}
	return nil
}

type funnelRequest struct {
	Range   string  `json:"range"`
	Segment string  `json:"segment,omitempty"`
	Goal    float64 `json:"goal"`
}

type funnelStep struct {
	Label    string  `json:"label"`
	Value    float64 `json:"value"`
	DropOff  float64 `json:"dropoff"`
	Position int     `json:"position"`
}

type funnelResponse struct {
	Range          string       `json:"range"`
	Segment        string       `json:"segment"`
	Goal           float64      `json:"goal"`
	ConversionRate float64      `json:"conversion_rate"`
	Steps          []funnelStep `json:"steps"`
}

func (r funnelResponse) toReport() dashboard.FunnelReport {
	steps := make([]dashboard.FunnelStep, len(r.Steps))
	for i, step := range r.Steps {
		steps[i] = dashboard.FunnelStep{
			Label:    step.Label,
			Value:    step.Value,
			DropOff:  step.DropOff,
			Position: step.Position,
		}
	}
	return dashboard.FunnelReport{
		Range:          r.Range,
		Segment:        r.Segment,
		Goal:           r.Goal,
		ConversionRate: r.ConversionRate,
		Steps:          steps,
	}
}

type cohortRequest struct {
	Interval string `json:"interval"`
	Periods  int    `json:"periods"`
	Metric   string `json:"metric"`
}

type cohortRow struct {
	Label     string    `json:"label"`
	Size      int       `json:"size"`
	Retention []float64 `json:"retention"`
}

type cohortResponse struct {
	Interval string      `json:"interval"`
	Metric   string      `json:"metric"`
	Rows     []cohortRow `json:"rows"`
}

func (r cohortResponse) toReport() dashboard.CohortReport {
	rows := make([]dashboard.CohortRow, len(r.Rows))
	for i, row := range r.Rows {
		rows[i] = dashboard.CohortRow{
			Label:     row.Label,
			Size:      row.Size,
			Retention: append([]float64(nil), row.Retention...),
		}
	}
	return dashboard.CohortReport{
		Interval: r.Interval,
		Metric:   r.Metric,
		Rows:     rows,
	}
}

type alertRequest struct {
	LookbackDays int      `json:"lookback_days"`
	Severities   []string `json:"severities"`
	Service      string   `json:"service"`
}

type alertSeries struct {
	Day    string         `json:"day"`
	Counts map[string]int `json:"counts"`
}

type alertResponse struct {
	Service string        `json:"service"`
	Series  []alertSeries `json:"series"`
}

func (r alertResponse) toReport() (dashboard.AlertTrendsReport, error) {
	series := make([]dashboard.AlertSeries, len(r.Series))
	totals := map[string]int{}
	for i, bucket := range r.Series {
		parsedDay, err := time.Parse(time.DateOnly, bucket.Day)
		if err != nil {
			return dashboard.AlertTrendsReport{}, fmt.Errorf("analytics: parse alert day %q: %w", bucket.Day, err)
		}
		counts := make(map[string]int, len(bucket.Counts))
		for sev, count := range bucket.Counts {
			counts[sev] = count
			totals[sev] += count
		}
		series[i] = dashboard.AlertSeries{Day: parsedDay, Counts: counts}
	}
	return dashboard.AlertTrendsReport{Service: r.Service, Series: series, Totals: totals}, nil
}
