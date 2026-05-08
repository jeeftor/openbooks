package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompareReleaseVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		latest  string
		want    int
	}{
		{
			name:    "older patch",
			current: "v2.0.1",
			latest:  "v2.0.2",
			want:    -1,
		},
		{
			name:    "same with v prefix mismatch",
			current: "2.0.1",
			latest:  "v2.0.1",
			want:    0,
		},
		{
			name:    "newer current",
			current: "v2.1.0",
			latest:  "v2.0.9",
			want:    1,
		},
		{
			name:    "prerelease is older than release",
			current: "v2.1.0-rc.1",
			latest:  "v2.1.0",
			want:    -1,
		},
		{
			name:    "build metadata ignored",
			current: "v2.1.0+20260508",
			latest:  "v2.1.0",
			want:    0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := compareReleaseVersions(test.current, test.latest)
			if !ok {
				t.Fatalf("compareReleaseVersions(%q, %q) returned ok=false", test.current, test.latest)
			}
			if got != test.want {
				t.Fatalf("compareReleaseVersions(%q, %q) = %d, want %d", test.current, test.latest, got, test.want)
			}
		})
	}
}

func TestCompareReleaseVersionsRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	if _, ok := compareReleaseVersions("master", "v2.0.1"); ok {
		t.Fatal("expected invalid current version to be rejected")
	}
	if _, ok := compareReleaseVersions("v2.0.1", "latest"); ok {
		t.Fatal("expected invalid latest version to be rejected")
	}
}

func TestGitHubUpdateCheckerReportsAvailableUpdateAndCachesLatestRelease(t *testing.T) {
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tag_name":"v2.0.2","html_url":"https://github.com/jeeftor/openbooks/releases/tag/v2.0.2"}`)
	}))
	t.Cleanup(api.Close)

	checker := newGitHubUpdateCheckerWithEndpoint(api.URL, nil)
	checker.now = func() time.Time {
		return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	}
	current := newVersionInfo("v2.0.1", "abcdef1234567890", "2026-05-08T10:00:00Z")

	update := checker.Check(context.Background(), current)
	if !update.Available {
		t.Fatalf("expected update to be available: %#v", update)
	}
	if update.Status != updateStatusAvailable {
		t.Fatalf("status = %q, want %q", update.Status, updateStatusAvailable)
	}
	if update.LatestVersion != "v2.0.2" {
		t.Fatalf("latest version = %q, want %q", update.LatestVersion, "v2.0.2")
	}

	second := checker.Check(context.Background(), current)
	if !second.Available {
		t.Fatalf("expected cached update to be available: %#v", second)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestGitHubUpdateCheckerSkipsDevBuilds(t *testing.T) {
	requested := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(api.Close)

	checker := newGitHubUpdateCheckerWithEndpoint(api.URL, nil)
	update := checker.Check(context.Background(), newVersionInfo("master", "abcdef1234567890", "unknown"))

	if update.Status != updateStatusUnknown {
		t.Fatalf("status = %q, want %q", update.Status, updateStatusUnknown)
	}
	if update.Reason != "not_release" {
		t.Fatalf("reason = %q, want %q", update.Reason, "not_release")
	}
	if requested {
		t.Fatal("dev builds should not call the release endpoint")
	}
}

func TestGitHubUpdateCheckerReportsCurrentRelease(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tag_name":"v2.0.1","html_url":"https://github.com/jeeftor/openbooks/releases/tag/v2.0.1"}`)
	}))
	t.Cleanup(api.Close)

	checker := newGitHubUpdateCheckerWithEndpoint(api.URL, nil)
	update := checker.Check(context.Background(), newVersionInfo("v2.0.1", "abcdef1234567890", "unknown"))

	if update.Available {
		t.Fatalf("expected current release: %#v", update)
	}
	if update.Status != updateStatusCurrent {
		t.Fatalf("status = %q, want %q", update.Status, updateStatusCurrent)
	}
}
