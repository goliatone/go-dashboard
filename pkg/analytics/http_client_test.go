package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

func TestHTTPClientFetchFunnel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/funnels/query" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("expected auth header, got %s", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&struct{}{})
		resp := funnelResponse{
			Range:          "30d",
			Segment:        "enterprise",
			Goal:           55,
			ConversionRate: 5.1,
			Steps:          []funnelStep{{Label: "Visitors", Value: 1000, Position: 0}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	client, err := NewHTTPClient(HTTPConfig{BaseURL: server.URL, APIKey: "secret"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	report, err := client.FetchFunnel(context.Background(), dashboard.FunnelQuery{Range: "30d"})
	if err != nil {
		t.Fatalf("fetch funnel: %v", err)
	}
	if len(report.Steps) != 1 || report.Steps[0].Label != "Visitors" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestHTTPClientFetchAlerts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alerts/query" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		resp := alertResponse{
			Service: "api",
			Series: []alertSeries{
				{Day: time.Now().UTC().Format(time.DateOnly), Counts: map[string]int{"critical": 3}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	client, err := NewHTTPClient(HTTPConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	report, err := client.FetchAlerts(context.Background(), dashboard.AlertTrendQuery{LookbackDays: 7})
	if err != nil {
		t.Fatalf("fetch alerts: %v", err)
	}
	if len(report.Series) != 1 {
		t.Fatalf("expected series, got %#v", report.Series)
	}
	if report.Totals["critical"] != 3 {
		t.Fatalf("expected totals updated, got %#v", report.Totals)
	}
}
