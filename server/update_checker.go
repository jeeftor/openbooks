package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	githubLatestReleaseURL = "https://api.github.com/repos/jeeftor/openbooks/releases/latest"
	updateStatusAvailable  = "available"
	updateStatusCurrent    = "current"
	updateStatusUnknown    = "unknown"
)

type updateChecker interface {
	Check(context.Context, VersionInfo) VersionUpdate
}

// VersionUpdate describes whether a newer OpenBooks ABS release is available.
type VersionUpdate struct {
	Status          string `json:"status"`
	Available       bool   `json:"available"`
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	ReleaseNotesURL string `json:"releaseNotesUrl,omitempty"`
	CheckedAt       string `json:"checkedAt,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type githubUpdateChecker struct {
	client   *http.Client
	endpoint string
	logger   *log.Logger
	ttl      time.Duration
	now      func() time.Time

	mu       sync.Mutex
	cached   latestRelease
	cachedAt time.Time
	err      error
}

type latestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func newGitHubUpdateChecker(logger *log.Logger) updateChecker {
	return newGitHubUpdateCheckerWithEndpoint(githubLatestReleaseURL, logger)
}

func newGitHubUpdateCheckerWithEndpoint(endpoint string, logger *log.Logger) *githubUpdateChecker {
	return &githubUpdateChecker{
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
		endpoint: endpoint,
		logger:   logger,
		ttl:      6 * time.Hour,
		now:      time.Now,
	}
}

func (checker *githubUpdateChecker) Check(ctx context.Context, current VersionInfo) VersionUpdate {
	now := checker.now().UTC()
	update := VersionUpdate{
		Status:         updateStatusUnknown,
		CurrentVersion: current.DisplayVersion,
		CheckedAt:      now.Format(time.RFC3339),
	}

	if !current.IsRelease {
		update.Reason = "not_release"
		return update
	}

	latest, err := checker.latest(ctx, now)
	if err != nil {
		update.Reason = "check_failed"
		if checker.logger != nil {
			checker.logger.Printf("release update check failed: %v", err)
		}
		return update
	}

	latestVersion := normalizeReleaseVersion(latest.TagName)
	update.LatestVersion = latestVersion
	update.ReleaseNotesURL = latest.HTMLURL
	if update.ReleaseNotesURL == "" && latestVersion != "" {
		update.ReleaseNotesURL = releaseNotesBaseURL + latestVersion
	}

	cmp, ok := compareReleaseVersions(current.DisplayVersion, latestVersion)
	if !ok {
		update.Reason = "version_parse_failed"
		return update
	}

	if cmp < 0 {
		update.Status = updateStatusAvailable
		update.Available = true
		return update
	}

	update.Status = updateStatusCurrent
	return update
}

func (checker *githubUpdateChecker) latest(ctx context.Context, now time.Time) (latestRelease, error) {
	checker.mu.Lock()
	if !checker.cachedAt.IsZero() && now.Sub(checker.cachedAt) < checker.ttl {
		cached := checker.cached
		err := checker.err
		checker.mu.Unlock()
		return cached, err
	}
	checker.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checker.endpoint, nil)
	if err != nil {
		return latestRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "openbooks-abs")

	resp, err := checker.client.Do(req)
	if err != nil {
		checker.store(latestRelease{}, now, err)
		return latestRelease{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("github latest release returned %s", resp.Status)
		checker.store(latestRelease{}, now, err)
		return latestRelease{}, err
	}

	var release latestRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		checker.store(latestRelease{}, now, err)
		return latestRelease{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		err := fmt.Errorf("github latest release did not include tag_name")
		checker.store(latestRelease{}, now, err)
		return latestRelease{}, err
	}

	checker.store(release, now, nil)
	return release, nil
}

func (checker *githubUpdateChecker) store(release latestRelease, checkedAt time.Time, err error) {
	checker.mu.Lock()
	defer checker.mu.Unlock()

	checker.cached = release
	checker.cachedAt = checkedAt
	checker.err = err
}

func normalizeReleaseVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

type semanticVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func compareReleaseVersions(current string, latest string) (int, bool) {
	currentVersion, ok := parseSemanticVersion(current)
	if !ok {
		return 0, false
	}
	latestVersion, ok := parseSemanticVersion(latest)
	if !ok {
		return 0, false
	}

	if currentVersion.major != latestVersion.major {
		return compareInts(currentVersion.major, latestVersion.major), true
	}
	if currentVersion.minor != latestVersion.minor {
		return compareInts(currentVersion.minor, latestVersion.minor), true
	}
	if currentVersion.patch != latestVersion.patch {
		return compareInts(currentVersion.patch, latestVersion.patch), true
	}
	return comparePrerelease(currentVersion.prerelease, latestVersion.prerelease), true
}

func parseSemanticVersion(version string) (semanticVersion, bool) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return semanticVersion{}, false
	}
	if plus := strings.Index(version, "+"); plus >= 0 {
		version = version[:plus]
	}

	prerelease := ""
	if dash := strings.Index(version, "-"); dash >= 0 {
		prerelease = version[dash+1:]
		version = version[:dash]
	}

	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return semanticVersion{}, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semanticVersion{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semanticVersion{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return semanticVersion{}, false
	}

	return semanticVersion{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: prerelease,
	}, true
}

func compareInts(current int, latest int) int {
	if current < latest {
		return -1
	}
	if current > latest {
		return 1
	}
	return 0
}

func comparePrerelease(current string, latest string) int {
	if current == latest {
		return 0
	}
	if current == "" {
		return 1
	}
	if latest == "" {
		return -1
	}

	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")
	for i := 0; i < len(currentParts) && i < len(latestParts); i++ {
		if currentParts[i] == latestParts[i] {
			continue
		}

		currentNumber, currentIsNumber := parsePrereleaseNumber(currentParts[i])
		latestNumber, latestIsNumber := parsePrereleaseNumber(latestParts[i])
		switch {
		case currentIsNumber && latestIsNumber:
			return compareInts(currentNumber, latestNumber)
		case currentIsNumber:
			return -1
		case latestIsNumber:
			return 1
		case currentParts[i] < latestParts[i]:
			return -1
		default:
			return 1
		}
	}

	return compareInts(len(currentParts), len(latestParts))
}

func parsePrereleaseNumber(value string) (int, bool) {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return number, true
}
