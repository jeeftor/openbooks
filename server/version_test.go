package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewVersionInfoRelease(t *testing.T) {
	info := newVersionInfo("v2.0.1", "1234567890abcdef", "2026-05-08T10:00:00Z")

	if !info.IsRelease {
		t.Fatal("expected release version")
	}
	if info.DisplayVersion != "v2.0.1" {
		t.Fatalf("display version = %q, want %q", info.DisplayVersion, "v2.0.1")
	}
	if info.RawVersion != "v2.0.1" {
		t.Fatalf("raw version = %q, want %q", info.RawVersion, "v2.0.1")
	}
	if info.ReleaseNotesURL != "https://github.com/jeeftor/openbooks/releases/tag/v2.0.1" {
		t.Fatalf("release notes URL = %q", info.ReleaseNotesURL)
	}
	if info.CommitSHA != "1234567890abcdef" {
		t.Fatalf("commit SHA = %q", info.CommitSHA)
	}
	if info.BuildDate != "2026-05-08T10:00:00Z" {
		t.Fatalf("build date = %q", info.BuildDate)
	}
}

func TestNewVersionInfoNormalizesReleaseWithoutVPrefix(t *testing.T) {
	info := newVersionInfo("2.0.1", "unknown", "unknown")

	if !info.IsRelease {
		t.Fatal("expected release version")
	}
	if info.DisplayVersion != "v2.0.1" {
		t.Fatalf("display version = %q, want %q", info.DisplayVersion, "v2.0.1")
	}
	if info.ReleaseNotesURL != "https://github.com/jeeftor/openbooks/releases/tag/v2.0.1" {
		t.Fatalf("release notes URL = %q", info.ReleaseNotesURL)
	}
}

func TestNewVersionInfoSupportsPrereleaseBuildMetadata(t *testing.T) {
	info := newVersionInfo("v2.1.0-rc.1+20260508", "unknown", "unknown")

	if !info.IsRelease {
		t.Fatal("expected release version")
	}
	if info.DisplayVersion != "v2.1.0-rc.1+20260508" {
		t.Fatalf("display version = %q, want %q", info.DisplayVersion, "v2.1.0-rc.1+20260508")
	}
	if info.ReleaseNotesURL != "https://github.com/jeeftor/openbooks/releases/tag/v2.1.0-rc.1+20260508" {
		t.Fatalf("release notes URL = %q", info.ReleaseNotesURL)
	}
}

func TestNewVersionInfoBranchBuildUsesChannelAndCommitLabel(t *testing.T) {
	info := newVersionInfo("master", "abcdef1234567890", "unknown")

	if info.IsRelease {
		t.Fatal("expected non-release version")
	}
	if info.DisplayVersion != "master abcdef1" {
		t.Fatalf("display version = %q, want %q", info.DisplayVersion, "master abcdef1")
	}
	if info.ReleaseNotesURL != "" {
		t.Fatalf("release notes URL = %q, want empty", info.ReleaseNotesURL)
	}
	if info.RawVersion != "master" {
		t.Fatalf("raw version = %q, want %q", info.RawVersion, "master")
	}
}

func TestNewVersionInfoDevBuildKeepsGenericDevLabel(t *testing.T) {
	info := newVersionInfo("dev", "abcdef1234567890", "unknown")

	if info.IsRelease {
		t.Fatal("expected non-release version")
	}
	if info.DisplayVersion != "dev abcdef1" {
		t.Fatalf("display version = %q, want %q", info.DisplayVersion, "dev abcdef1")
	}
	if info.RawVersion != "dev" {
		t.Fatalf("raw version = %q, want %q", info.RawVersion, "dev")
	}
}

func TestNewVersionInfoUnknownBuildFallsBackToDev(t *testing.T) {
	info := newVersionInfo("unknown", "unknown", "unknown")

	if info.IsRelease {
		t.Fatal("expected non-release version")
	}
	if info.DisplayVersion != "dev" {
		t.Fatalf("display version = %q, want %q", info.DisplayVersion, "dev")
	}
}

func TestVersionHandlerReturnsStructuredMetadata(t *testing.T) {
	server := New(Config{
		Version:   "v2.0.1",
		CommitSHA: "1234567890abcdef",
		BuildDate: "2026-05-08T10:00:00Z",
	})
	server.updateChecker = nil

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/version", nil)
	server.versionHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content type = %q, want application/json", contentType)
	}

	var info VersionInfo
	if err := json.NewDecoder(recorder.Body).Decode(&info); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if !info.IsRelease || info.DisplayVersion != "v2.0.1" || info.ReleaseNotesURL == "" {
		t.Fatalf("unexpected version response: %#v", info)
	}
}

func TestVersionHandlerReturnsUpdateMetadata(t *testing.T) {
	server := New(Config{
		Version:   "v2.0.1",
		CommitSHA: "1234567890abcdef",
		BuildDate: "2026-05-08T10:00:00Z",
	})
	server.updateChecker = staticUpdateChecker{
		update: VersionUpdate{
			Status:          updateStatusAvailable,
			Available:       true,
			CurrentVersion:  "v2.0.1",
			LatestVersion:   "v2.0.2",
			ReleaseNotesURL: "https://github.com/jeeftor/openbooks/releases/tag/v2.0.2",
		},
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/version", nil)
	server.versionHandler().ServeHTTP(recorder, request)

	var info VersionInfo
	if err := json.NewDecoder(recorder.Body).Decode(&info); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if info.Update == nil {
		t.Fatal("expected update metadata")
	}
	if !info.Update.Available || info.Update.LatestVersion != "v2.0.2" {
		t.Fatalf("unexpected update response: %#v", info.Update)
	}
}

type staticUpdateChecker struct {
	update VersionUpdate
}

func (checker staticUpdateChecker) Check(_ context.Context, _ VersionInfo) VersionUpdate {
	return checker.update
}
